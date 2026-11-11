package system_test

import (
	"dnd/internal/adapter/system"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, n := range []string{"a.db", "b.db"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o700)
	var f system.Files
	names, err := f.List(dir)
	if err != nil || !slices.Equal(names, []string{"a.db", "b.db"}) {
		t.Fatalf("list %v %v", names, err)
	}
	if err := f.Remove(filepath.Join(dir, "a.db")); err != nil {
		t.Fatal(err)
	}
	if err := f.Remove(filepath.Join(dir, "a.db")); err == nil {
		t.Fatal("removing a missing file fails")
	}
	if _, err := f.List(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("listing a missing folder fails")
	}
	if c := (system.Clock{}).Now(); c.IsZero() || len(system.IDs{}.New()) != 12 {
		t.Fatal("clock and ids")
	}
}
