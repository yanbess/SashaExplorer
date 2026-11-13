// Package protect keeps files readable only for the current Windows user.
package protect

import (
	"fmt"
	"os"
	"path/filepath"
)

// Protector encrypts file content.
type Protector interface {
	Protect(data []byte) ([]byte, error)
	Unprotect(data []byte) ([]byte, error)
}

// LoadFile reads and decrypts a file.
func LoadFile(path string, protector Protector) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return protector.Unprotect(content)
}

// SaveFile encrypts content and replaces the file, so a crash leaves either the
// old or the new version.
func SaveFile(path string, protector Protector, plain []byte) error {
	content, err := protector.Protect(plain)
	if err != nil {
		return fmt.Errorf("protect file: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("save file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}
