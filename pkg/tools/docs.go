package tools

import (
	"fmt"
	"log/slog"

	"github.com/google/jsonschema-go/jsonschema"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// allowedDocTypes defines which doc types can be updated via update_doc.
// All are user-level documents stored in user_workspace_docs.
// IsAllowedDocType reports whether a doc type can be updated via update_doc.
func IsAllowedDocType(dt string) bool { return allowedDocTypes[dt] }

var allowedDocTypes = map[string]bool{
	"IDENTITY": true,
	"SOUL":     true,
	"AGENT":    true,
	"USER":     true,
}

type updateDocArgs struct {
	DocType string `json:"doc_type" jsonschema:"Document type to update: IDENTITY, SOUL, AGENT, or USER"`
	Content string `json:"content" jsonschema:"Full new content for the document"`
}

type updateDocResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func newUpdateDoc(deps *Deps) tool.Tool {
	inputSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"doc_type": {
				Type:        "string",
				Description: "Document type to update",
				Enum:        []any{"IDENTITY", "SOUL", "AGENT", "USER"},
			},
			"content": {
				Type:        "string",
				Description: "Full new content for the document",
			},
		},
		Required: []string{"doc_type", "content"},
	}

	t, _ := functiontool.New(functiontool.Config{
		Name:        "update_doc",
		Description: "Update a persona document. Changes are persisted across sessions.",
		InputSchema: inputSchema,
	}, func(ctx tool.Context, args updateDocArgs) (updateDocResult, error) {
		if !allowedDocTypes[args.DocType] {
			return updateDocResult{Error: fmt.Sprintf("invalid doc_type %q; allowed: IDENTITY, SOUL, AGENT, USER", args.DocType)}, nil
		}
		if args.Content == "" {
			return updateDocResult{Error: "content must not be empty"}, nil
		}
		if deps.UserWorkspaceStore == nil {
			return updateDocResult{Error: "user workspace store not available"}, nil
		}

		tenantID := stateStr(ctx, "tenant_id")
		if tenantID == "" {
			tenantID = types.DefaultTenantID
		}
		userID := stateStr(ctx, "user_id")
		if userID == "" {
			userID = types.DefaultUserID
		}

		err := deps.UserWorkspaceStore.Put(ctx, &types.UserWorkspaceDoc{
			TenantID: tenantID,
			UserID:   userID,
			DocType:  args.DocType,
			Content:  args.Content,
			Version:  1,
		})
		if err != nil {
			return updateDocResult{Error: fmt.Sprintf("store failed: %v", err)}, nil
		}

		slog.Debug("update_doc", "doc_type", args.DocType, "tenant", tenantID, "user", userID)
		return updateDocResult{Result: fmt.Sprintf("%s updated", args.DocType)}, nil
	})
	return t
}
