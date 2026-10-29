package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "chesstui"
	accountName = "lichess"
	envToken    = "CHESSTUI_LICHESS_TOKEN"
)

var ErrTokenNotFound = errors.New("lichess token not found")

// TokenStore abstracts secret storage for production and tests.
type TokenStore interface {
	Load() (string, error)
	Save(token string) error
	Delete() error
}

// SystemTokenStore reads an environment override first, then the OS
// credential store (Keychain, Secret Service, Windows Credential Manager).
// When no credential store is available, as on a headless Linux box, it
// falls back to FallbackPath, written with 0600 permissions.
type SystemTokenStore struct {
	FallbackPath string
}

func (s SystemTokenStore) Load() (string, error) {
	if token := os.Getenv(envToken); token != "" {
		return token, nil
	}
	token, err := keyring.Get(serviceName, accountName)
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		slog.Warn("system keyring unavailable", "error", err)
	}
	if s.FallbackPath == "" {
		return "", ErrTokenNotFound
	}
	return FileTokenStore{Path: s.FallbackPath}.Load()
}

func (s SystemTokenStore) Save(token string) error {
	if token == "" {
		return errors.New("refusing to save an empty token")
	}
	err := keyring.Set(serviceName, accountName, token)
	if err == nil {
		// Do not leave an older plaintext copy behind.
		if s.FallbackPath != "" {
			_ = FileTokenStore{Path: s.FallbackPath}.Delete()
		}
		return nil
	}
	if s.FallbackPath == "" {
		return fmt.Errorf("save token to system keyring: %w", err)
	}
	slog.Warn("system keyring unavailable; using private token file", "error", err)
	return FileTokenStore{Path: s.FallbackPath}.Save(token)
}

func (s SystemTokenStore) Delete() error {
	err := keyring.Delete(serviceName, accountName)
	if errors.Is(err, keyring.ErrNotFound) {
		err = nil
	}
	if s.FallbackPath != "" {
		if fileErr := (FileTokenStore{Path: s.FallbackPath}).Delete(); fileErr != nil {
			return fileErr
		}
		// Keyring absence is fine when the file held the token.
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete token from system keyring: %w", err)
	}
	return nil
}

// FileTokenStore keeps the token in a file only the current user can read.
type FileTokenStore struct {
	Path string
}

func (f FileTokenStore) Load() (string, error) {
	info, err := os.Stat(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		// Someone loosened the permissions. Tighten them before trusting it.
		if err := os.Chmod(f.Path, 0o600); err != nil {
			return "", fmt.Errorf("token file %s is readable by other users: %w", f.Path, err)
		}
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", ErrTokenNotFound
	}
	return token, nil
}

func (f FileTokenStore) Save(token string) error {
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".token-*")
	if err != nil {
		return fmt.Errorf("create token file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect token file: %w", err)
	}
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("write token file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.Path); err != nil {
		return fmt.Errorf("replace token file: %w", err)
	}
	return nil
}

func (f FileTokenStore) Delete() error {
	if err := os.Remove(f.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete token file: %w", err)
	}
	return nil
}
