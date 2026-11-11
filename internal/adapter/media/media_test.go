package media_test

import (
	"dnd/internal/adapter/media"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSaveExistsURL(t *testing.T) {
	t.Parallel()
	s, err := media.New(filepath.Join(t.TempDir(), "media"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Exists("games/g1/pc1") {
		t.Fatal("nothing saved yet")
	}
	if _, ok := s.URL("games/g1/pc1"); ok {
		t.Fatal("no URL before saving")
	}
	if err := s.Save("games/g1/pc1", []byte("img")); err != nil {
		t.Fatal(err)
	}
	url, ok := s.URL("games/g1/pc1")
	if !ok || !s.Exists("games/g1/pc1") || !strings.HasPrefix(url, "/media/games/g1/pc1.webp?v=") {
		t.Fatalf("url %q ok %v", url, ok)
	}
	for _, bad := range []string{"../etc/passwd", "games/../../x", "Games/X", ""} {
		if err := s.Save(bad, nil); !errors.Is(err, media.ErrBadKey) {
			t.Errorf("Save(%q) = %v", bad, err)
		}
		if s.Exists(bad) {
			t.Errorf("Exists(%q) should be false", bad)
		}
		if _, ok := s.URL(bad); ok {
			t.Errorf("URL(%q) should fail", bad)
		}
	}
}

func TestCopy(t *testing.T) {
	t.Parallel()
	s, _ := media.New(filepath.Join(t.TempDir(), "media"))
	_ = s.Save("games/g1/pc1", []byte("img"))
	if ok, err := s.Copy("games/g1/pc1", "characters/ch1"); !ok || err != nil || !s.Exists("characters/ch1") {
		t.Fatalf("copy %v %v", ok, err)
	}
	if ok, _ := s.Copy("games/g1/pc1", "characters/ch1"); ok {
		t.Fatal("must not overwrite an existing image")
	}
	if ok, err := s.Copy("games/g1/missing", "characters/ch2"); ok || err != nil {
		t.Fatalf("missing source: %v %v", ok, err)
	}
	if _, err := s.Copy("../x", "characters/ch3"); !errors.Is(err, media.ErrBadKey) {
		t.Fatalf("bad key: %v", err)
	}
}

func TestShelfServesBundledStock(t *testing.T) {
	t.Parallel()
	s := media.NewShelf(fstest.MapFS{"stock/candlelit/scenes/tavern.webp": {Data: []byte("img")}}, "candlelit")
	if url, ok := s.URL("scenes/tavern"); !ok || url != "/stock/candlelit/scenes/tavern.webp" {
		t.Fatalf("got %q %v", url, ok)
	}
	if _, ok := s.URL("scenes/cave"); ok {
		t.Fatal("missing stock")
	}
	if _, ok := s.URL("../secret"); ok {
		t.Fatal("bad keys are refused")
	}
}
