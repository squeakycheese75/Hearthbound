package backups_test

import (
	"context"
	"dnd/internal/usecase/backups"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

type disk struct {
	files                   []string
	snapErr, listErr, rmErr error
	removed                 []string
}

func (d *disk) Snapshot(_ context.Context, path string) error {
	if d.snapErr != nil {
		return d.snapErr
	}
	d.files = append(d.files, path[len("/b/"):])
	return nil
}

func (d *disk) List(string) ([]string, error) { return slices.Clone(d.files), d.listErr }

func (d *disk) Remove(path string) error {
	if d.rmErr != nil {
		return d.rmErr
	}
	d.removed = append(d.removed, path)
	d.files = slices.DeleteFunc(d.files, func(f string) bool { return "/b/"+f == path })
	return nil
}

type clock struct{ t time.Time }

func (c clock) Now() time.Time { return c.t }

var quiet = slog.New(slog.DiscardHandler)

func TestRunSnapshotsAndKeepsTheNewest(t *testing.T) {
	t.Parallel()
	d := &disk{files: []string{"tabletop-20261030-030000.db", "tabletop-20261031-030000.db", "notes.txt", "tabletop-20261101-030000.db"}}
	now := time.Date(2026, 11, 2, 3, 0, 0, 0, time.FixedZone("CET", 3600))
	uc := backups.New(d, d, clock{now}, backups.Config{Dir: "/b", Keep: 2}, quiet)
	path, err := uc.Run(context.Background())
	if err != nil || path != "/b/tabletop-20261102-020000.db" {
		t.Fatalf("path %q err %v (times are UTC)", path, err)
	}
	want := []string{"/b/tabletop-20261030-030000.db", "/b/tabletop-20261031-030000.db"}
	if !slices.Equal(d.removed, want) || !slices.Contains(d.files, "notes.txt") {
		t.Fatalf("removed %v, left %v", d.removed, d.files)
	}
}

func TestRunFailures(t *testing.T) {
	t.Parallel()
	for name, d := range map[string]*disk{
		"snapshot": {snapErr: errBoom},
		"list":     {listErr: errBoom},
		"remove":   {files: []string{"tabletop-1.db", "tabletop-2.db"}, rmErr: errBoom},
	} {
		uc := backups.New(d, d, clock{time.Unix(0, 0)}, backups.Config{Dir: "/b"}, quiet)
		if _, err := uc.Run(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
