package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateLockFileReturnsOpenError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "bsptile.lock")
	file, err := createLockFile(path)
	if err == nil {
		if file != nil {
			file.Close()
		}
		t.Fatal("expected lock-file open error")
	}
	if file != nil {
		t.Fatal("lock-file open failure returned a non-nil file")
	}
}

func TestCreateLockFilePreventsSecondLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bsptile.lock")
	first, err := createLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second, err := createLockFile(path)
	if err == nil {
		if second != nil {
			second.Close()
		}
		t.Fatal("expected second lock acquisition to fail")
	}
}

func TestCreateLockFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bsptile.lock")
	file, err := createLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("lock mode = %o, want 600", got)
	}
}
