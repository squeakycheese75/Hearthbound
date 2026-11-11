// Package backups takes consistent snapshots of the database while the server
// runs, and keeps the newest few.
package backups

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Snapshotter writes a consistent copy of the database to a new file.
type Snapshotter interface {
	Snapshot(ctx context.Context, path string) error
}

// Files lists and removes files in the backup folder.
type Files interface {
	List(dir string) ([]string, error)
	Remove(path string) error
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// Config says where backups go and how many to keep.
type Config struct {
	Dir  string
	Keep int
}

// UseCase takes and prunes backups.
type UseCase struct {
	db    Snapshotter
	files Files
	clock Clock
	cfg   Config
	log   *slog.Logger
}

// New wires the use case. Keep is at least 1.
func New(db Snapshotter, files Files, clock Clock, cfg Config, log *slog.Logger) *UseCase {
	cfg.Keep = max(cfg.Keep, 1)
	return &UseCase{db: db, files: files, clock: clock, cfg: cfg, log: log}
}

// Prefix and Suffix frame backup file names: tabletop-20261102-031500.db.
const (
	Prefix = "tabletop-"
	Suffix = ".db"
)

// Run takes a snapshot now and removes all but the newest Keep backups. It
// returns the new backup's path.
func (u *UseCase) Run(ctx context.Context) (string, error) {
	path := filepath.Join(u.cfg.Dir, Prefix+u.clock.Now().UTC().Format("20060102-150405")+Suffix)
	if err := u.db.Snapshot(ctx, path); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	if err := u.prune(); err != nil {
		return path, err
	}
	u.log.Info("backup taken", "path", path)
	return path, nil
}

// prune removes the oldest backups beyond Keep. Names sort by time.
func (u *UseCase) prune() error {
	names, err := u.files.List(u.cfg.Dir)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	names = slices.DeleteFunc(names, func(n string) bool {
		return !strings.HasPrefix(n, Prefix) || !strings.HasSuffix(n, Suffix)
	})
	slices.Sort(names)
	for len(names) > u.cfg.Keep {
		if err := u.files.Remove(filepath.Join(u.cfg.Dir, names[0])); err != nil {
			return fmt.Errorf("remove old backup: %w", err)
		}
		names = names[1:]
	}
	return nil
}
