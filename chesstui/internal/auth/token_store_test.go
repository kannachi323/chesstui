package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileTokenStoreIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "token")
	store := FileTokenStore{Path: path}
	if _, err := store.Load(); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("empty load err = %v", err)
	}
	if err := store.Save("lio_secret"); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("token file mode = %o, want 600", perm)
		}
		dir, _ := os.Stat(filepath.Dir(path))
		if perm := dir.Mode().Perm(); perm != 0o700 {
			t.Fatalf("token dir mode = %o, want 700", perm)
		}
	}
	if token, err := store.Load(); err != nil || token != "lio_secret" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	if err := store.Delete(); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(); err != nil {
		t.Fatalf("second delete should be a no-op: %v", err)
	}
}

func TestFileTokenStoreTightensLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("lio_secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (FileTokenStore{Path: path}).Load(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode after load = %o", perm)
	}
}

func TestEnvironmentTokenWins(t *testing.T) {
	t.Setenv(envToken, "lio_env")
	token, err := SystemTokenStore{}.Load()
	if err != nil || token != "lio_env" {
		t.Fatalf("token=%q err=%v", token, err)
	}
}
