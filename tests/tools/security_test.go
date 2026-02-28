package tools_test

import (
	"os"
	"path/filepath"
	"testing"

	"abot/pkg/tools"
)

// --- ValidatePath tests ---

func TestValidatePath_RelativeOK(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	os.WriteFile(filepath.Join(ws, "hello.txt"), []byte("hi"), 0o644)

	got, err := tools.ValidatePath("hello.txt", ws)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(ws, "hello.txt") {
		t.Errorf("got %q", got)
	}
}

func TestValidatePath_TraversalBlocked(t *testing.T) {
	ws := t.TempDir()
	_, err := tools.ValidatePath("../../etc/passwd", ws)
	if err == nil {
		t.Fatal("expected path traversal to be blocked")
	}
}

func TestValidatePath_AbsoluteOutsideBlocked(t *testing.T) {
	ws := t.TempDir()
	_, err := tools.ValidatePath("/etc/passwd", ws)
	if err == nil {
		t.Fatal("expected absolute path outside workspace to be blocked")
	}
}

func TestValidatePath_EmptyWorkspace(t *testing.T) {
	_, err := tools.ValidatePath("file.txt", "")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestValidatePath_NewFileInWorkspace(t *testing.T) {
	ws := t.TempDir()
	ws, _ = filepath.EvalSymlinks(ws)

	got, err := tools.ValidatePath("newfile.txt", ws)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(ws, "newfile.txt") {
		t.Errorf("got %q", got)
	}
}

func TestValidatePath_AllowedPaths(t *testing.T) {
	ws := t.TempDir()
	allowed := t.TempDir()
	allowed, _ = filepath.EvalSymlinks(allowed)

	os.WriteFile(filepath.Join(allowed, "ok.txt"), []byte("ok"), 0o644)

	got, err := tools.ValidatePath(filepath.Join(allowed, "ok.txt"), ws, allowed)
	if err != nil {
		t.Fatalf("allowed path should be permitted: %v", err)
	}
	if got != filepath.Join(allowed, "ok.txt") {
		t.Errorf("got %q", got)
	}
}

func TestUserWorkspaceDir(t *testing.T) {
	tests := []struct {
		base, tenant, user, want string
	}{
		{"ws", "", "", "ws"},
		{"ws", "default", "", "ws"},
		{"ws", "t1", "", filepath.Join("ws", "t1")},
		{"ws", "t1", "default", filepath.Join("ws", "t1")},
		{"ws", "t1", "u1", filepath.Join("ws", "t1", "u1")},
		{"ws", "", "u1", filepath.Join("ws", "u1")},
		{"ws", "default", "u1", filepath.Join("ws", "u1")},
	}
	for _, tt := range tests {
		got := tools.UserWorkspaceDir(tt.base, tt.tenant, tt.user)
		if got != tt.want {
			t.Errorf("UserWorkspaceDir(%q,%q,%q) = %q, want %q",
				tt.base, tt.tenant, tt.user, got, tt.want)
		}
	}
}
