package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCheckCommand(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644)

	if ok, detail := runCheckCommand(context.Background(), dir, "test -f hello.txt", ""); !ok {
		t.Fatalf("expected pass, got %s", detail)
	}
	if ok, _ := runCheckCommand(context.Background(), dir, "test -f nope.txt", ""); ok {
		t.Fatal("expected fail for a missing file")
	}
	if ok, _ := runCheckCommand(context.Background(), dir, "cat hello.txt", "world"); !ok {
		t.Fatal("expected pass when output contains the expected substring")
	}
	if ok, _ := runCheckCommand(context.Background(), dir, "cat hello.txt", "absent-string"); ok {
		t.Fatal("expected fail when output lacks the expected substring")
	}
}

func TestRunCheckGrep(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc Foo() {}\n"), 0o644)

	if ok, detail := runCheckGrep(context.Background(), dir, "func Foo", ""); !ok {
		t.Fatalf("expected present pass, got %s", detail)
	}
	if ok, _ := runCheckGrep(context.Background(), dir, "func Bar", ""); ok {
		t.Fatal("expected fail when pattern is absent")
	}
	if ok, _ := runCheckGrep(context.Background(), dir, "func Bar", "absent"); !ok {
		t.Fatal("expected pass when an absent pattern is expected absent")
	}
	if ok, _ := runCheckGrep(context.Background(), dir, "func Foo", "absent"); ok {
		t.Fatal("expected fail when a present pattern is expected absent")
	}
}
