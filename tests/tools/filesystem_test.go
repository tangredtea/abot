package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"abot/pkg/tools"
)

// --- read_file logic tests ---

func TestReadFile_Basic(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	content := "hello world\nline two"
	os.WriteFile(filepath.Join(ws, "test.txt"), []byte(content), 0o644)

	fullPath, err := tools.ValidatePath("test.txt", ws)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

func TestReadFile_TooLarge(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	// Create a file larger than MaxReadFileSize (2MB)
	big := make([]byte, tools.MaxReadFileSize+1)
	os.WriteFile(filepath.Join(ws, "big.bin"), big, 0o644)

	fullPath, _ := tools.ValidatePath("big.bin", ws)
	info, _ := os.Stat(fullPath)
	if info.Size() <= tools.MaxReadFileSize {
		t.Fatal("test file should exceed max size")
	}
}

func TestReadFile_PathTraversal(t *testing.T) {
	ws := t.TempDir()
	_, err := tools.ValidatePath("../../etc/passwd", ws)
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestReadFile_Nonexistent(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fullPath, err := tools.ValidatePath("nonexistent.txt", ws)
	if err != nil {
		t.Fatal(err) // path validation should pass for new files
	}
	_, err = os.Stat(fullPath)
	if !os.IsNotExist(err) {
		t.Error("expected file not to exist")
	}
}

// --- write_file logic tests ---

func TestWriteFile_CreateAndRead(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fullPath, err := tools.ValidatePath("output.txt", ws)
	if err != nil {
		t.Fatal(err)
	}
	content := "written content"
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(fullPath)
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

func TestWriteFile_CreateSubdir(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fullPath, err := tools.ValidatePath("sub/dir/file.txt", ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(fullPath)
	if string(data) != "nested" {
		t.Errorf("got %q", string(data))
	}
}

func TestWriteFile_Overwrite(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fp := filepath.Join(ws, "overwrite.txt")
	os.WriteFile(fp, []byte("old"), 0o644)
	os.WriteFile(fp, []byte("new"), 0o644)

	data, _ := os.ReadFile(fp)
	if string(data) != "new" {
		t.Errorf("got %q, want %q", string(data), "new")
	}
}

// --- edit_file logic tests ---

func TestEditFile_UniqueMatch(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fp := filepath.Join(ws, "edit.txt")
	os.WriteFile(fp, []byte("hello world"), 0o644)

	data, _ := os.ReadFile(fp)
	content := string(data)
	count := strings.Count(content, "hello")
	if count != 1 {
		t.Fatalf("expected 1 match, got %d", count)
	}
	newContent := strings.Replace(content, "hello", "goodbye", 1)
	os.WriteFile(fp, []byte(newContent), 0o644)

	result, _ := os.ReadFile(fp)
	if string(result) != "goodbye world" {
		t.Errorf("got %q", string(result))
	}
}

func TestEditFile_NoMatch(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fp := filepath.Join(ws, "edit2.txt")
	os.WriteFile(fp, []byte("hello world"), 0o644)

	data, _ := os.ReadFile(fp)
	content := string(data)
	count := strings.Count(content, "nonexistent")
	if count != 0 {
		t.Fatal("should not find nonexistent text")
	}
}

func TestEditFile_MultipleMatches(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fp := filepath.Join(ws, "edit3.txt")
	os.WriteFile(fp, []byte("aaa bbb aaa"), 0o644)

	data, _ := os.ReadFile(fp)
	content := string(data)
	count := strings.Count(content, "aaa")
	if count != 2 {
		t.Fatalf("expected 2 matches, got %d", count)
	}
	// edit_file should reject this — multiple matches
}

// --- append_file logic tests ---

func TestAppendFile_NewFile(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	fp := filepath.Join(ws, "append.txt")
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("first line\n")
	f.Close()

	f, _ = os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	f.WriteString("second line\n")
	f.Close()

	data, _ := os.ReadFile(fp)
	if string(data) != "first line\nsecond line\n" {
		t.Errorf("got %q", string(data))
	}
}

// --- list_dir logic tests ---

func TestListDir_Basic(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	os.WriteFile(filepath.Join(ws, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(ws, "b.txt"), []byte("bb"), 0o644)
	os.Mkdir(filepath.Join(ws, "subdir"), 0o755)

	entries, err := os.ReadDir(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	for _, want := range []string{"a.txt", "b.txt", "subdir"} {
		if !names[want] {
			t.Errorf("missing entry %q", want)
		}
	}
}

func TestListDir_Empty(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	entries, err := os.ReadDir(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestListDir_Nonexistent(t *testing.T) {
	ws := t.TempDir()
	_, err := os.ReadDir(filepath.Join(ws, "nope"))
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}
