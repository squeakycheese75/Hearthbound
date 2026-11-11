// Package system wraps the wall clock, ID generation and the file system.
package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// Clock reads the wall clock.
type Clock struct{}

// Now returns the current UTC time.
func (Clock) Now() time.Time { return time.Now().UTC() }

// IDs generates random hex IDs.
type IDs struct{}

// New returns a 12-character random ID.
func (IDs) New() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Files lists and removes files on disk.
type Files struct{}

// List returns the names of the files in a folder.
func (Files) List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Remove deletes a file.
func (Files) Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
