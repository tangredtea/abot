package storage_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"abot/pkg/storage/objectstore"
)

// --- LocalStore tests ---

func TestLocalStore_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	err := store.Put(ctx, "test.txt", strings.NewReader("hello world"))
	if err != nil {
		t.Fatal(err)
	}

	rc, err := store.Get(ctx, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	data, _ := io.ReadAll(rc)
	if string(data) != "hello world" {
		t.Errorf("got %q", string(data))
	}
}

func TestLocalStore_Exists(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	ok, err := store.Exists(ctx, "nope.txt")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false for missing file")
	}

	store.Put(ctx, "yes.txt", strings.NewReader("data"))
	ok, err = store.Exists(ctx, "yes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true after put")
	}
}

func TestLocalStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	store.Put(ctx, "del.txt", strings.NewReader("bye"))

	if err := store.Delete(ctx, "del.txt"); err != nil {
		t.Fatal(err)
	}

	ok, _ := store.Exists(ctx, "del.txt")
	if ok {
		t.Error("expected file to be deleted")
	}
}

func TestLocalStore_NestedPath(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	err := store.Put(ctx, "sub/dir/file.txt", strings.NewReader("nested"))
	if err != nil {
		t.Fatal(err)
	}

	rc, err := store.Get(ctx, "sub/dir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "nested" {
		t.Errorf("got %q", string(data))
	}
}

func TestLocalStore_TraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	err := store.Put(ctx, "../../etc/passwd", strings.NewReader("bad"))
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestLocalStore_AbsolutePathBlocked(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	err := store.Put(ctx, "/etc/passwd", strings.NewReader("bad"))
	if err == nil {
		t.Error("expected absolute path to be blocked")
	}
}

func TestLocalStore_EmptyPathBlocked(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)
	ctx := context.Background()

	err := store.Put(ctx, "", strings.NewReader("bad"))
	if err == nil {
		t.Error("expected empty path to be blocked")
	}
}

func TestLocalStore_GetMissing(t *testing.T) {
	dir := t.TempDir()
	store := objectstore.NewLocalStore(dir)

	_, err := store.Get(context.Background(), "nonexistent.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- Comprehensive traversal tests (migrated from pkg/storage/objectstore/store_test.go) ---

func TestLocalStore_RejectsTraversal(t *testing.T) {
	store := objectstore.NewLocalStore(t.TempDir())
	ctx := context.Background()

	attacks := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"foo/../../etc/passwd",
		"foo/../../../etc/passwd",
	}
	for _, path := range attacks {
		if err := store.Put(ctx, path, strings.NewReader("x")); err == nil {
			t.Errorf("Put(%q) should fail", path)
		}
		if _, err := store.Get(ctx, path); err == nil {
			t.Errorf("Get(%q) should fail", path)
		}
		if err := store.Delete(ctx, path); err == nil {
			t.Errorf("Delete(%q) should fail", path)
		}
		if _, err := store.Exists(ctx, path); err == nil {
			t.Errorf("Exists(%q) should fail", path)
		}
	}
}

func TestLocalStore_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	store := objectstore.NewLocalStore(root)
	ctx := context.Background()

	// Create a symlink inside root that points outside.
	target := t.TempDir() // a different temp dir
	linkPath := root + "/escape"
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	// Writing through the symlink should be rejected.
	if err := store.Put(ctx, "escape/secret.txt", strings.NewReader("x")); err == nil {
		t.Error("Put through symlink escape should fail")
	}
}
