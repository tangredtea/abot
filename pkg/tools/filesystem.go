package tools

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// docSyncMap maps workspace-relative paths to their doc type and store target.
// "workspace" = workspace_docs, "user" = user_workspace_docs.
var docSyncMap = map[string]struct {
	docType string
	store   string // "workspace" or "user"
}{
	"IDENTITY.md":  {"IDENTITY", "user"},
	"SOUL.md":      {"SOUL", "user"},
	"RULES.md":     {"RULES", "workspace"},
	"TOOLS.md":     {"TOOLS", "workspace"},
	"AGENT.md":     {"AGENT", "user"},
	"HEARTBEAT.md": {"HEARTBEAT", "workspace"},
	"USER.md":      {"USER", "user"},
}

// syncFileToStore syncs a workspace file to the corresponding MySQL store
// if the path matches a known doc type. Best-effort — logs errors but doesn't fail.
func syncFileToStore(ctx tool.Context, deps *Deps, relPath, content string) error {
	info, ok := docSyncMap[relPath]
	if !ok {
		return nil
	}
	tenantID := stateStr(ctx, "tenant_id")
	if tenantID == "" {
		tenantID = types.DefaultTenantID
	}

	switch info.store {
	case "workspace":
		if deps.WorkspaceStore == nil {
			return nil
		}
		if err := deps.WorkspaceStore.Put(ctx, &types.WorkspaceDoc{
			TenantID: tenantID,
			DocType:  info.docType,
			Content:  content,
			Version:  1,
		}); err != nil {
			return fmt.Errorf("sync to workspace store: %w", err)
		}
		slog.Debug("sync file to store", "path", relPath, "target", "workspace_docs."+info.docType)

	case "user":
		if deps.UserWorkspaceStore == nil {
			return nil
		}
		userID := stateStr(ctx, "user_id")
		if userID == "" {
			userID = types.DefaultUserID
		}
		if err := deps.UserWorkspaceStore.Put(ctx, &types.UserWorkspaceDoc{
			TenantID: tenantID,
			UserID:   userID,
			DocType:  info.docType,
			Content:  content,
			Version:  1,
		}); err != nil {
			return fmt.Errorf("sync to user workspace store: %w", err)
		}
		slog.Debug("sync file to store", "path", relPath, "target", "user_workspace_docs."+info.docType)
	}
	return nil
}

// --- read_file ---

type readFileArgs struct {
	Path string `json:"path" jsonschema:"File path relative to workspace"`
}

type readFileResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

const MaxReadFileSize = 2 * 1024 * 1024 // 2 MB

func newReadFile(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Read the contents of a file from the workspace (max 2 MB).",
	}, func(ctx tool.Context, args readFileArgs) (readFileResult, error) {
		wsDir := UserWorkspaceDir(deps.WorkspaceDir, stateStr(ctx, "tenant_id"), stateStr(ctx, "user_id"))
		fullPath, err := ValidatePath(args.Path, wsDir, deps.AllowedPaths...)
		if err != nil {
			return readFileResult{Error: err.Error()}, nil
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return readFileResult{Error: fmt.Sprintf("stat failed: %v", err)}, nil
		}
		if info.Size() > MaxReadFileSize {
			return readFileResult{Error: fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), MaxReadFileSize)}, nil
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return readFileResult{Error: fmt.Sprintf("read failed: %v", err)}, nil
		}
		return readFileResult{Content: string(data)}, nil
	})
	return t
}

// --- write_file ---

type writeFileArgs struct {
	Path    string `json:"path" jsonschema:"File path relative to workspace"`
	Content string `json:"content" jsonschema:"Content to write"`
}

type writeFileResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func newWriteFile(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "write_file",
		Description: "Write content to a file in the workspace. Creates parent directories if needed.",
	}, func(ctx tool.Context, args writeFileArgs) (writeFileResult, error) {
		wsDir := UserWorkspaceDir(deps.WorkspaceDir, stateStr(ctx, "tenant_id"), stateStr(ctx, "user_id"))
		fullPath, err := ValidatePath(args.Path, wsDir, deps.AllowedPaths...)
		if err != nil {
			return writeFileResult{Error: err.Error()}, nil
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return writeFileResult{Error: fmt.Sprintf("mkdir failed: %v", err)}, nil
		}
		if err := os.WriteFile(fullPath, []byte(args.Content), 0o644); err != nil {
			return writeFileResult{Error: fmt.Sprintf("write failed: %v", err)}, nil
		}
		if err := syncFileToStore(ctx, deps, args.Path, args.Content); err != nil {
			slog.Warn("write_file: sync to store failed", "path", args.Path, "err", err)
		}
		return writeFileResult{Result: fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path)}, nil
	})
	return t
}

// --- edit_file ---

type editFileArgs struct {
	Path    string `json:"path" jsonschema:"File path relative to workspace"`
	OldText string `json:"old_text" jsonschema:"Exact text to find and replace"`
	NewText string `json:"new_text" jsonschema:"Replacement text"`
}

type editFileResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func newEditFile(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "edit_file",
		Description: "Replace an exact text match in a file. Fails if old_text is not found or appears multiple times.",
	}, func(ctx tool.Context, args editFileArgs) (editFileResult, error) {
		wsDir := UserWorkspaceDir(deps.WorkspaceDir, stateStr(ctx, "tenant_id"), stateStr(ctx, "user_id"))
		fullPath, err := ValidatePath(args.Path, wsDir, deps.AllowedPaths...)
		if err != nil {
			return editFileResult{Error: err.Error()}, nil
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return editFileResult{Error: fmt.Sprintf("read failed: %v", err)}, nil
		}
		content := string(data)
		count := strings.Count(content, args.OldText)
		if count == 0 {
			return editFileResult{Error: "old_text not found in file"}, nil
		}
		if count > 1 {
			return editFileResult{Error: fmt.Sprintf("old_text found %d times, must be unique", count)}, nil
		}
		newContent := strings.Replace(content, args.OldText, args.NewText, 1)
		if err := os.WriteFile(fullPath, []byte(newContent), 0o644); err != nil {
			return editFileResult{Error: fmt.Sprintf("write failed: %v", err)}, nil
		}
		if err := syncFileToStore(ctx, deps, args.Path, newContent); err != nil {
			slog.Warn("edit_file: sync to store failed", "path", args.Path, "err", err)
		}
		return editFileResult{Result: "edit applied"}, nil
	})
	return t
}

// --- append_file ---

type appendFileArgs struct {
	Path    string `json:"path" jsonschema:"File path relative to workspace"`
	Content string `json:"content" jsonschema:"Content to append"`
}

func newAppendFile(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "append_file",
		Description: "Append content to the end of a file. Creates the file if it does not exist.",
	}, func(ctx tool.Context, args appendFileArgs) (writeFileResult, error) {
		wsDir := UserWorkspaceDir(deps.WorkspaceDir, stateStr(ctx, "tenant_id"), stateStr(ctx, "user_id"))
		fullPath, err := ValidatePath(args.Path, wsDir, deps.AllowedPaths...)
		if err != nil {
			return writeFileResult{Error: err.Error()}, nil
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return writeFileResult{Error: fmt.Sprintf("mkdir failed: %v", err)}, nil
		}
		f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return writeFileResult{Error: fmt.Sprintf("open failed: %v", err)}, nil
		}
		defer f.Close()
		if _, err := f.WriteString(args.Content); err != nil {
			return writeFileResult{Error: fmt.Sprintf("append failed: %v", err)}, nil
		}
		// Sync full content to MySQL after append.
		if final, err := os.ReadFile(fullPath); err == nil {
			if err := syncFileToStore(ctx, deps, args.Path, string(final)); err != nil {
				slog.Warn("append_file: sync to store failed", "path", args.Path, "err", err)
			}
		}
		return writeFileResult{Result: fmt.Sprintf("appended %d bytes to %s", len(args.Content), args.Path)}, nil
	})
	return t
}

// --- list_dir ---

type listDirArgs struct {
	Path string `json:"path" jsonschema:"Directory path relative to workspace. Empty string for workspace root."`
}

type listDirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type listDirResult struct {
	Entries []listDirEntry `json:"entries,omitempty"`
	Error   string         `json:"error,omitempty"`
}

func newListDir(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "list_dir",
		Description: "List files and directories in a workspace path.",
	}, func(ctx tool.Context, args listDirArgs) (listDirResult, error) {
		dirPath := args.Path
		if dirPath == "" {
			dirPath = "."
		}
		wsDir := UserWorkspaceDir(deps.WorkspaceDir, stateStr(ctx, "tenant_id"), stateStr(ctx, "user_id"))
		fullPath, err := ValidatePath(dirPath, wsDir, deps.AllowedPaths...)
		if err != nil {
			return listDirResult{Error: err.Error()}, nil
		}
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return listDirResult{Error: fmt.Sprintf("readdir failed: %v", err)}, nil
		}
		result := make([]listDirEntry, 0, len(entries))
		for _, e := range entries {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			result = append(result, listDirEntry{
				Name:  e.Name(),
				IsDir: e.IsDir(),
				Size:  size,
			})
		}
		return listDirResult{Entries: result}, nil
	})
	return t
}
