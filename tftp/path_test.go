package tftp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolvePath_OK(t *testing.T) {
	root := t.TempDir()
	full, err := resolvePath(root, "firmware.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "firmware.bin")
	if full != want {
		t.Errorf("want %q, got %q", want, full)
	}
}

func TestResolvePath_Subdir(t *testing.T) {
	root := t.TempDir()
	full, err := resolvePath(root, "subdir/image.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "subdir", "image.bin")
	if full != want {
		t.Errorf("want %q, got %q", want, full)
	}
}

func TestResolvePath_DotDotTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"foo/../../etc/passwd",
		"./../../etc/passwd",
	}
	for _, name := range cases {
		full, err := resolvePath(root, name)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", name, err)
			continue
		}
		if !strings.HasPrefix(full, root+string(filepath.Separator)) && full != root {
			t.Errorf("%q escaped root: %q", name, full)
		}
	}
}

func TestResolvePath_AbsolutePath(t *testing.T) {
	root := t.TempDir()
	abs := "/etc/passwd"
	if runtime.GOOS == "windows" {
		abs = `C:\Windows\System32\cmd.exe`
	}
	full, err := resolvePath(root, abs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(full, root) {
		t.Errorf("absolute %q escaped root: %q", abs, full)
	}
}

func TestResolvePath_SymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation on Windows requires admin privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := resolvePath(root, "escape")
	if err == nil {
		t.Fatal("expected error for symlink pointing outside root, got nil")
	}
}

func TestResolvePath_SymlinkInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation on Windows requires admin privileges")
	}
	root := t.TempDir()
	target := filepath.Join(root, "real.bin")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "latest.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	full, err := resolvePath(root, "latest.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(full, root+string(filepath.Separator)) {
		t.Errorf("resolved outside root: %q", full)
	}
}

func TestResolvePath_NonExistent(t *testing.T) {
	root := t.TempDir()
	full, err := resolvePath(root, "newfile.bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "newfile.bin")
	if full != want {
		t.Errorf("want %q, got %q", want, full)
	}
}
