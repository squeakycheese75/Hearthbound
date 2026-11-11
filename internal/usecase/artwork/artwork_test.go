package artwork_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/artwork"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
)

type fakeGen struct {
	broke   map[string]bool // payers without a key
	mu      sync.Mutex
	prompts []string
	payers  []string
	err     error
}

func (f *fakeGen) Ready(_ context.Context, payer string) bool { return !f.broke[payer] }

func (f *fakeGen) Generate(_ context.Context, payer, prompt, size string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prompts = append(f.prompts, size+"|"+prompt)
	f.payers = append(f.payers, payer)
	return []byte("img"), f.err
}

type fakeMedia struct {
	mu      sync.Mutex
	files   map[string][]byte
	saveErr error
}

func (m *fakeMedia) Exists(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[key]
	return ok
}

func (m *fakeMedia) Save(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.files[key] = data
	return nil
}

func (m *fakeMedia) URL(key string) (string, bool) {
	if !m.Exists(key) {
		return "", false
	}
	return "/media/" + key, true
}

type recorder struct {
	mu     sync.Mutex
	events []domain.Event
}

func (r *recorder) Publish(_ string, e domain.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

type games struct{ err error }

func (g games) Load(_ context.Context, id string) (*domain.Game, error) {
	return &domain.Game{
		ID: id, LeaderID: "me", Party: []string{"pc1"},
		Scene: domain.Scene{Key: "forest", Title: "Whispering Wood", Mood: "eerie", Visual: "Twisted pines in fog."},
		Creatures: []*domain.Creature{
			{ID: "pc1", OwnerID: "friend", Name: "Aria", Kind: domain.KindPC, Race: "Human", Class: "Fighter"},
			{ID: "goblin-1", Name: "Goblin 1", Kind: domain.KindMonster, Portrait: "goblin"},
		},
	}, g.err
}

var quiet = slog.New(slog.DiscardHandler)

// worker starts the painter inside a synctest bubble; settle waits until it is idle.
func worker(u *artwork.UseCase) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { u.Run(ctx); close(done) }()
	return func() { cancel(); <-done }
}

func TestPaintsOnceAndPublishes(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		gen, media, pub := &fakeGen{}, &fakeMedia{files: map[string][]byte{}}, &recorder{}
		u := artwork.New(gen, media, nil, pub, games{}, quiet, 8)
		hero := &domain.Creature{ID: "pc1", Name: "Aria", Kind: domain.KindPC, Race: "Human", Class: "Fighter"}
		req := domain.PortraitRequest("g1", hero)

		u.Request(req)
		u.Request(req) // already pending: not queued twice
		if !u.Pending(req.Key) || !u.Enabled() {
			t.Fatal("expected pending portrait")
		}
		stop := worker(u)
		synctest.Wait()
		if _, ok := u.Picture(req.Key, ""); !ok || len(gen.prompts) != 1 || len(pub.events) != 1 || pub.events[0].Type != domain.EvImage {
			t.Fatalf("prompts=%d events=%+v", len(gen.prompts), pub.events)
		}
		u.Request(req) // exists now: ignored
		if u.Pending(req.Key) {
			t.Fatal("existing art must not be repainted")
		}
		stop()
	})
}

func TestRerollRepaintsExistingArt(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		gen, media := &fakeGen{}, &fakeMedia{files: map[string][]byte{"games/g1/pc1": []byte("old")}}
		u := artwork.New(gen, media, nil, &recorder{}, games{}, quiet, 8)
		ctx := context.Background()
		for _, subject := range []string{"pc1", "goblin-1", domain.SceneSubject} {
			if err := u.Reroll(ctx, "me", "g1", subject); err != nil {
				t.Fatal(err)
			}
		}
		stop := worker(u)
		synctest.Wait()
		stop()
		payers := strings.Join(gen.payers, ",")
		if !strings.Contains(payers, "friend") || strings.Count(payers, "me") != 2 {
			t.Fatalf("portrait billed to owner, monster+scene to leader; got %s", payers)
		}
		if string(media.files["games/g1/pc1"]) != "img" || len(gen.prompts) != 3 {
			t.Fatalf("files=%v prompts=%d", media.files, len(gen.prompts))
		}
		joined := strings.Join(gen.prompts, "\n")
		for _, want := range []string{"1536x1024|Wide establishing shot", "Twisted pines in fog", "portrait of a goblin", "Aria, a human fighter"} {
			if !strings.Contains(joined, want) {
				t.Errorf("prompts missing %q", want)
			}
		}
		if err := u.Reroll(ctx, "stranger", "g1", "pc1"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("stranger reroll: %v", err)
		}
		if err := u.Reroll(ctx, "me", "g1", "nobody"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("unknown subject: %v", err)
		}
		missing := artwork.New(gen, media, nil, &recorder{}, games{err: domain.ErrNotFound}, quiet, 1)
		if err := missing.Reroll(ctx, "me", "x", "pc1"); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("missing game: %v", err)
		}
	})
}

func TestFailuresAreDroppedNotPublished(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		pub := &recorder{}
		req := domain.PortraitRequest("g1", &domain.Creature{ID: "pc1", Name: "Aria", Kind: domain.KindPC})
		for _, u := range []*artwork.UseCase{
			artwork.New(&fakeGen{err: errors.New("rate limited")}, &fakeMedia{files: map[string][]byte{}}, nil, pub, games{}, quiet, 2),
			artwork.New(&fakeGen{err: context.Canceled}, &fakeMedia{files: map[string][]byte{}}, nil, pub, games{}, quiet, 2),
			artwork.New(&fakeGen{}, &fakeMedia{files: map[string][]byte{}, saveErr: errors.New("disk full")}, nil, pub, games{}, quiet, 2),
		} {
			u.Request(req)
			stop := worker(u)
			synctest.Wait()
			stop()
			if u.Pending(req.Key) {
				t.Fatal("a failed painting must clear its pending flag")
			}
		}
		if len(pub.events) != 0 {
			t.Fatalf("failed paintings must not be announced: %+v", pub.events)
		}
	})
}

func TestQueueFullSkipsAndDisabled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		u := artwork.New(&fakeGen{}, &fakeMedia{files: map[string][]byte{}}, nil, &recorder{}, games{}, quiet, 1)
		u.Request(domain.ArtRequest{Key: "a"})
		u.Request(domain.ArtRequest{Key: "b"}) // queue full: dropped
		if u.Pending("b") {
			t.Fatal("overflow request should be dropped")
		}
		if err := u.Reroll(context.Background(), "me", "g1", "pc1"); !errors.Is(err, domain.ErrBusy) {
			t.Fatalf("reroll with full queue: %v", err)
		}

		// A queued job whose image appeared meanwhile is skipped without generating.
		gen, media := &fakeGen{}, &fakeMedia{files: map[string][]byte{}}
		u = artwork.New(gen, media, nil, &recorder{}, games{}, quiet, 2)
		u.Request(domain.ArtRequest{Key: "c"})
		media.files["c"] = []byte("x")
		stop := worker(u)
		synctest.Wait()
		stop()
		if len(gen.prompts) != 0 {
			t.Fatal("existing image should not be regenerated")
		}

		off := artwork.New(nil, &fakeMedia{files: map[string][]byte{}}, nil, &recorder{}, games{}, quiet, 1)
		off.Request(domain.ArtRequest{Key: "a"})
		if off.Enabled() || off.Pending("a") {
			t.Fatal("disabled artwork must do nothing")
		}
		if err := off.Reroll(context.Background(), "me", "g1", "pc1"); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("disabled reroll: %v", err)
		}
	})
}

func TestEnsureRequestsWhatIsMissing(t *testing.T) {
	t.Parallel()
	media := &fakeMedia{files: map[string][]byte{"games/g1/pc1": []byte("done")}}
	u := artwork.New(&fakeGen{}, media, nil, &recorder{}, games{}, quiet, 16)
	g := &domain.Game{
		ID:    "g1",
		Scene: domain.Scene{Key: "crypt", Title: "Thornwatch Crypt"},
		Creatures: []*domain.Creature{
			{ID: "pc1", Kind: domain.KindPC},
			{ID: "pc2", Kind: domain.KindPC},
			{ID: "goblin-1", Kind: domain.KindMonster, Portrait: "goblin"},
			{ID: "ogre-1", Kind: domain.KindMonster, Portrait: "ogre", Dead: true},
		},
		NPCs: []*domain.NPC{{ID: "npc-snik", Type: "kobold", Monster: true, Here: true}, {ID: "npc-mara", Type: "innkeeper"}},
	}
	u.Ensure(g)
	for key, want := range map[string]bool{
		"games/g1/pc1": false, "games/g1/pc2": true, "monsters/goblin": true, "monsters/ogre": false,
		"games/g1/scene-crypt-thornwatch-crypt": true, "games/g1/npc-snik": true, "games/g1/npc-mara": false,
	} {
		if u.Pending(key) != want {
			t.Errorf("pending(%s) = %v, want %v", key, !want, want)
		}
	}
	u.Ensure(&domain.Game{ID: "g2"}) // no scene yet: nothing to paint
	if u.Pending("games/g2/scene-") {
		t.Error("a game without a scene should not request a backdrop")
	}
	off := artwork.New(nil, media, nil, &recorder{}, games{}, quiet, 1)
	off.Ensure(g)
	if off.Pending("games/g1/pc2") {
		t.Error("disabled artwork must not queue")
	}
}

func TestPayersWithoutKeysAreSkipped(t *testing.T) {
	t.Parallel()
	u := artwork.New(&fakeGen{broke: map[string]bool{"friend": true}}, &fakeMedia{files: map[string][]byte{}}, nil, &recorder{}, games{}, quiet, 8)
	if u.CanPaint(context.Background(), "friend") || !u.CanPaint(context.Background(), "me") {
		t.Fatal("CanPaint should follow whether the payer has a key")
	}
	u.Request(domain.ArtRequest{Key: "games/g1/pc1", Payer: "friend"})
	if u.Pending("games/g1/pc1") {
		t.Fatal("art for a payer without a key must not queue")
	}
	if err := u.Reroll(context.Background(), "me", "g1", "pc1"); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("reroll billed to a keyless owner: %v", err)
	}
}

func TestEnsureCharacter(t *testing.T) {
	t.Parallel()
	u := artwork.New(&fakeGen{}, &fakeMedia{files: map[string][]byte{}}, nil, &recorder{}, games{}, quiet, 4)
	ch := &domain.Character{ID: "ch1", OwnerID: "me", Sheet: &domain.Creature{ID: "ch1", Name: "Aria", Kind: domain.KindPC}}
	u.EnsureCharacter(ch)
	u.EnsureCharacter(&domain.Character{ID: "blank"})
	if !u.Pending("characters/ch1") || u.Pending("characters/blank") {
		t.Fatal("portrait should be queued for characters with a sheet")
	}
	off := artwork.New(nil, &fakeMedia{files: map[string][]byte{}}, nil, &recorder{}, games{}, quiet, 1)
	off.EnsureCharacter(ch)
	if off.Pending("characters/ch1") {
		t.Fatal("disabled artwork must not queue")
	}
}

type shelf map[string]bool

func (s shelf) URL(key string) (string, bool) {
	if !s[key] {
		return "", false
	}
	return "/stock/candlelit/" + key + ".webp", true
}

func TestPicturePrefersPaintingThenStock(t *testing.T) {
	t.Parallel()
	media := &fakeMedia{files: map[string][]byte{"characters/aria": []byte("img")}}
	u := artwork.New(nil, media, shelf{"portraits/elf-wizard": true}, &recorder{}, games{}, quiet, 1)
	if url, ok := u.Picture("characters/aria", "portraits/elf-wizard"); !ok || url == "/stock/candlelit/portraits/elf-wizard.webp" {
		t.Fatalf("a painting wins over stock: %s", url)
	}
	if url, ok := u.Picture("characters/pip", "portraits/elf-wizard"); !ok || url != "/stock/candlelit/portraits/elf-wizard.webp" {
		t.Fatalf("stock fills in: %s %v", url, ok)
	}
	if _, ok := u.Picture("characters/pip", "portraits/orc-bard"); ok {
		t.Fatal("missing stock is missing")
	}
	if _, ok := u.Picture("characters/pip", ""); ok {
		t.Fatal("no stock key, no picture")
	}
	none := artwork.New(nil, media, nil, &recorder{}, games{}, quiet, 1)
	if _, ok := none.Picture("characters/pip", "portraits/elf-wizard"); ok {
		t.Fatal("art off: no stock either")
	}
}
