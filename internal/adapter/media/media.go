// Package media stores generated illustrations as files and serves their URLs.
package media

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Ext is the file type every illustration is stored as.
const Ext = ".webp"

// URLPrefix is where the web server mounts the media directory.
const URLPrefix = "/media/"

var validKey = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)

// ErrBadKey means a key would escape the media directory.
var ErrBadKey = errors.New("invalid media key")

// Store keeps images under a root directory, one file per art key.
type Store struct {
	root string
}

// New returns a store rooted at dir, creating it if needed.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create media dir: %w", err)
	}
	return &Store{root: dir}, nil
}

// Root is the directory to serve at URLPrefix.
func (s *Store) Root() string { return s.root }

func (s *Store) path(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fmt.Errorf("%w: %q", ErrBadKey, key)
	}
	return filepath.Join(s.root, filepath.FromSlash(key)+Ext), nil
}

// Exists reports whether an image has been saved for key.
func (s *Store) Exists(key string) bool {
	p, err := s.path(key)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Save writes the image atomically, replacing any previous version.
func (s *Store) Save(key string, data []byte) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("create media dir: %w", err)
	}
	tmp := p + ".tmp"
	// #nosec G703 -- p comes from s.path, which only accepts keys matching validKey (no "..", no absolute paths).
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write image: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("store image: %w", err)
	}
	return nil
}

// URL returns the image's address, versioned by modification time so a
// rerolled image is never served stale from the browser cache.
func (s *Store) URL(key string) (string, bool) {
	p, err := s.path(key)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", false
	}
	return URLPrefix + key + Ext + "?v=" + strconv.FormatInt(info.ModTime().UnixNano(), 36), true
}

// Copy duplicates an image to a new key, unless the source is missing or the
// destination already exists. It reports whether a copy was made.
func (s *Store) Copy(from, to string) (bool, error) {
	src, err := s.path(from)
	if err != nil {
		return false, err
	}
	if s.Exists(to) {
		return false, nil
	}
	data, err := os.ReadFile(src) // #nosec G304 -- path validated by s.path
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read image: %w", err)
	}
	if err := s.Save(to, data); err != nil {
		return false, err
	}
	return true, nil
}

// Shelf serves one art pack's stock images, bundled with the UI
// (web/public/stock/<pack>/, embedded in the binary). Keys are relative to the
// pack, like "scenes/tavern".
type Shelf struct {
	fsys fs.FS
	dir  string
}

// NewShelf reads a pack's stock images from fsys, the built UI's file system.
func NewShelf(fsys fs.FS, pack string) *Shelf {
	return &Shelf{fsys: fsys, dir: "stock/" + pack + "/"}
}

// URL returns where a stock image is served, if it exists.
func (s *Shelf) URL(key string) (string, bool) {
	if !validKey.MatchString(key) {
		return "", false
	}
	if _, err := fs.Stat(s.fsys, s.dir+key+Ext); err != nil {
		return "", false
	}
	return "/" + s.dir + key + Ext, true
}
