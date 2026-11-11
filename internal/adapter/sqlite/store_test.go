package sqlite_test

import (
	"context"
	"database/sql"
	"dnd/internal/adapter/sqlite"
	"dnd/internal/domain"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	g := &domain.Game{
		ID: "g1", LeaderID: "me", Title: "Quest", Mode: domain.ModeExploration, UpdatedAt: time.UnixMilli(1000).UTC(),
		Scene:     domain.Scene{Key: "tavern", Title: "Tavern"},
		Creatures: []*domain.Creature{{ID: "pc1", Name: "Aria", Class: "Fighter", Kind: domain.KindPC}},
		Party:     []string{"pc1"},
	}
	if err := store.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	g.Title, g.UpdatedAt = "Quest II", time.UnixMilli(2000).UTC()
	if err := store.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(ctx, "g1")
	if err != nil || got.Title != "Quest II" || got.Transcripts == nil || got.Creatures[0].Name != "Aria" {
		t.Fatalf("load %+v %v", got, err)
	}
	if other, _ := store.List(ctx, "someone-else"); len(other) != 0 {
		t.Fatalf("other accounts must not see the game: %+v", other)
	}
	list, err := store.List(ctx, "me")
	if err != nil || len(list) != 1 || list[0].Party != "Aria the Fighter" || list[0].Scene != "Tavern" || list[0].SceneKey != "tavern" {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := store.Delete(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx, "g1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("load deleted: %v", err)
	}
	if err := store.Delete(ctx, "g1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestClaimOrphans(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	old := &domain.Game{ID: "old", Party: []string{"pc1"}, Creatures: []*domain.Creature{{ID: "pc1", Kind: domain.KindPC}}}
	mine := &domain.Game{ID: "mine", LeaderID: "someone"}
	_ = store.Save(ctx, old)
	_ = store.Save(ctx, mine)
	n, err := store.ClaimOrphans(ctx, "admin")
	if err != nil || n != 1 {
		t.Fatalf("claimed %d %v", n, err)
	}
	g, _ := store.Load(ctx, "old")
	if g.LeaderID != "admin" || g.Creatures[0].OwnerID != "admin" {
		t.Fatalf("orphan not claimed: %+v", g)
	}
	if list, _ := store.List(ctx, "admin"); len(list) != 1 {
		t.Fatalf("admin should now list the old game: %+v", list)
	}
	if n, _ := store.ClaimOrphans(ctx, "admin"); n != 0 {
		t.Fatal("claiming is idempotent")
	}
}

func TestMembershipTurnsAndDeadlines(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	due := time.UnixMilli(10_000).UTC()
	g := &domain.Game{
		ID: "g1", LeaderID: "me", Mode: domain.ModeExploration, Started: true, Awaiting: true, Deadline: due,
		Party: []string{"a", "b"}, Pending: map[string]string{"a": "done"},
		Creatures: []*domain.Creature{
			{ID: "a", OwnerID: "me", Kind: domain.KindPC, Controller: domain.ControlHuman, HP: 5},
			{ID: "b", OwnerID: "friend", Kind: domain.KindPC, Controller: domain.ControlHuman, HP: 5},
		},
	}
	if err := store.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	mine, _ := store.List(ctx, "me")
	theirs, _ := store.List(ctx, "friend")
	if len(mine) != 1 || mine[0].YourTurn || !mine[0].Deadline.Equal(due) {
		t.Fatalf("leader view %+v", mine)
	}
	if len(theirs) != 1 || !theirs[0].YourTurn {
		t.Fatalf("a party member sees the game and that it is their turn: %+v", theirs)
	}
	if other, _ := store.List(ctx, "fr"); len(other) != 0 {
		t.Fatal("membership must match whole account IDs")
	}
	if ids, _ := store.DueGames(ctx, due.Add(-time.Second)); len(ids) != 0 {
		t.Fatal("not due yet")
	}
	if ids, _ := store.DueGames(ctx, due); len(ids) != 1 || ids[0] != "g1" {
		t.Fatalf("due %v", ids)
	}
	g.Deadline, g.Awaiting = time.Time{}, false
	_ = store.Save(ctx, g)
	if ids, _ := store.DueGames(ctx, due.Add(time.Hour)); len(ids) != 0 {
		t.Fatal("no deadline, never due")
	}
	if theirs, _ := store.List(ctx, "friend"); theirs[0].YourTurn {
		t.Fatal("a busy game is nobody's turn")
	}
}

func TestOlderGamesGetTheirSceneKeyOnOpen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []*domain.Game{
		{ID: "g1", LeaderID: "me", Scene: domain.Scene{Key: "crypt", Title: "The Crypt"}},
		{ID: "g2", LeaderID: "me"},
	} {
		_ = store.Save(ctx, g)
	}
	_ = store.Close()
	// An older version saved no scene_key.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE games SET scene_key = ''`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	store, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	list, _ := store.List(ctx, "me")
	keys := map[string]string{}
	for _, s := range list {
		keys[s.ID] = s.SceneKey
	}
	if keys["g1"] != "crypt" || keys["g2"] != "" {
		t.Fatalf("scene keys %v", keys)
	}
}

func TestSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	_ = store.Save(ctx, &domain.Game{ID: "g1", LeaderID: "me", Title: "Kept"})
	copyPath := filepath.Join(dir, "copy.db")
	if err := store.Snapshot(ctx, copyPath); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, copyPath); err == nil {
		t.Fatal("a snapshot never overwrites a file")
	}
	restored, err := sqlite.Open(ctx, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restored.Close() }()
	if g, err := restored.Load(ctx, "g1"); err != nil || g.Title != "Kept" {
		t.Fatalf("restored %+v %v", g, err)
	}
}

func TestFriendsSeatsAndTheBoard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	party := func(id string) *domain.Game {
		return &domain.Game{
			ID: id, LeaderID: "lead", Title: id, Mode: domain.ModeExploration,
			Creatures: []*domain.Creature{
				{ID: id + "-a", Name: "Aria", Kind: domain.KindPC, OwnerID: "lead"},
				{ID: id + "-b", Name: "Bram", Kind: domain.KindPC, OwnerID: "lead", PlayerID: "friend"},
			},
			Party: []string{id + "-a", id + "-b"},
		}
	}
	open, over, closed, wanted := party("open"), party("over"), party("closed"), party("wanted")
	open.OpenSeats, over.OpenSeats, over.Mode = []string{"open-a"}, []string{"over-a"}, domain.ModeOver
	wanted.Wanted = "A healer, please"
	for _, g := range []*domain.Game{open, over, closed, wanted} {
		if err := store.Save(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	games, err := store.GamesOnTheBoard(ctx)
	if err != nil || len(games) != 2 {
		t.Fatalf("only running games with business on the board: %v %v", games, err)
	}
	mine, _ := store.List(ctx, "friend")
	if len(mine) != 4 {
		t.Fatalf("a friend holding a seat sees the game: %d", len(mine))
	}
}
