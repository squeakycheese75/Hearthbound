package newgame_test

import (
	"context"
	"dnd/internal/adapter/srd"
	"dnd/internal/domain"
	"dnd/internal/usecase/newgame"
	"errors"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()

type fakeStore struct {
	chars map[string]domain.Character
	saved *domain.Game
	party []domain.Character
	err   error
}

func (f *fakeStore) Character(_ context.Context, id string) (domain.Character, error) {
	ch, ok := f.chars[id]
	if !ok {
		return domain.Character{}, domain.ErrNotFound
	}
	return ch, nil
}

func (f *fakeStore) CreateAdventure(_ context.Context, g *domain.Game, party []domain.Character) error {
	f.saved, f.party = g, party
	return f.err
}

type easel struct{ keys []string }

func (e *easel) Request(r domain.ArtRequest) { e.keys = append(e.keys, r.Key) }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC) }

// ai reports a registered AI for every account except "nokey".
type ai struct{}

func (ai) Ready(_ context.Context, accountID string) bool { return accountID != "nokey" }

type fixedIDs struct{}

func (fixedIDs) New() string { return "game1" }

func char(id, owner, name string, status domain.CharacterStatus) domain.Character {
	return domain.Character{
		ID: id, OwnerID: owner, Status: status, Hire: domain.HireNo,
		Sheet: &domain.Creature{ID: id, OwnerID: owner, Name: name, Kind: domain.KindPC, Controller: domain.ControlAI, HP: 10, MaxHP: 10},
	}
}

func store() *fakeStore {
	return &fakeStore{chars: map[string]domain.Character{
		"aria":   char("aria", "me", "Aria", domain.CharIdle),
		"pip":    char("pip", "me", "Pip", domain.CharIdle),
		"maren":  char("maren", "me", "Maren", domain.CharIdle),
		"busy":   char("busy", "me", "Busy", domain.CharAdventuring),
		"theirs": char("theirs", "friend", "Theirs", domain.CharIdle),
	}}
}

func TestStartsAdventureFromRoster(t *testing.T) {
	t.Parallel()
	st, art := store(), &easel{}
	uc := newgame.New(srd.New(), st, art, ai{}, fixedClock{}, fixedIDs{})
	out, err := uc.Execute(ctx, newgame.Input{AccountID: "me", DMStyle: "grim", Pace: "brisk", HeroID: "aria", CompanionIDs: []string{"pip", "maren"}})
	if err != nil {
		t.Fatal(err)
	}
	g := st.saved
	if g.Pace.Key != "brisk" || len(uc.Options().Paces) == 0 {
		t.Fatalf("pace %+v", g.Pace)
	}
	if out.GameID != "game1" || !g.Roster || g.LeaderID != "me" || g.Title != "The Adventures of Aria" || g.DMStyle != "grim" {
		t.Fatalf("game %+v", g)
	}
	if strings.Join(g.Party, ",") != "aria,pip,maren" {
		t.Fatalf("party %v", g.Party)
	}
	hero, _ := g.Hero()
	pip, _ := g.Creature("pip")
	if hero.ID != "aria" || pip.Controller != domain.ControlAI || pip.OwnerID != "me" {
		t.Fatalf("seats hero=%+v pip=%+v", hero, pip)
	}
	for _, ch := range st.party {
		if ch.Status != domain.CharAdventuring || ch.AdventureID != "game1" {
			t.Fatalf("%s should be away on game1: %+v", ch.ID, ch)
		}
	}
	if st.chars["aria"].Sheet.Controller != domain.ControlAI {
		t.Fatal("the roster sheet must not be changed by seating the hero")
	}
	if strings.Join(art.keys, ",") != "characters/aria,characters/pip,characters/maren" {
		t.Fatalf("portraits: %v", art.keys)
	}
	opts := uc.Options()
	if len(opts.Classes) == 0 || len(opts.Races) == 0 || len(opts.Archetypes) == 0 || len(opts.DMStyles) == 0 {
		t.Fatal("options")
	}
}

func TestCustomTitle(t *testing.T) {
	t.Parallel()
	st := store()
	uc := newgame.New(srd.New(), st, &easel{}, ai{}, fixedClock{}, fixedIDs{})
	if _, err := uc.Execute(ctx, newgame.Input{AccountID: "me", Title: "  Curse of the Crow ", HeroID: "aria", Tutorial: true}); err != nil {
		t.Fatal(err)
	}
	if st.saved.Title != "Curse of the Crow" || !st.saved.Tutorial {
		t.Fatalf("title %q tutorial %v", st.saved.Title, st.saved.Tutorial)
	}
}

func TestRejectsBadParties(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   newgame.Input
		want error
	}{
		{"signed out", newgame.Input{HeroID: "aria"}, domain.ErrUnauthorized},
		{"no AI registered", newgame.Input{AccountID: "nokey", HeroID: "aria"}, domain.ErrNoKey},
		{"no hero", newgame.Input{AccountID: "me"}, domain.ErrInvalid},
		{"too many", newgame.Input{AccountID: "me", HeroID: "aria", CompanionIDs: []string{"a", "b", "c", "d"}}, domain.ErrInvalid},
		{"same character twice", newgame.Input{AccountID: "me", HeroID: "aria", CompanionIDs: []string{"aria"}}, domain.ErrInvalid},
		{"unknown character", newgame.Input{AccountID: "me", HeroID: "ghost"}, domain.ErrNotFound},
		{"someone else's character", newgame.Input{AccountID: "me", HeroID: "theirs"}, domain.ErrNotFound},
		{"already adventuring", newgame.Input{AccountID: "me", HeroID: "aria", CompanionIDs: []string{"busy"}}, domain.ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := newgame.New(srd.New(), store(), &easel{}, ai{}, fixedClock{}, fixedIDs{})
			if _, err := uc.Execute(ctx, tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFallenAndStoreFailures(t *testing.T) {
	t.Parallel()
	st := store()
	dead := st.chars["pip"]
	dead.Sheet.Dead = true
	st.chars["pip"] = dead
	uc := newgame.New(srd.New(), st, &easel{}, ai{}, fixedClock{}, fixedIDs{})
	if _, err := uc.Execute(ctx, newgame.Input{AccountID: "me", HeroID: "aria", CompanionIDs: []string{"pip"}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a fallen character cannot adventure: %v", err)
	}
	st = store()
	st.err = errors.New("disk full")
	uc = newgame.New(srd.New(), st, &easel{}, ai{}, fixedClock{}, fixedIDs{})
	if _, err := uc.Execute(ctx, newgame.Input{AccountID: "me", HeroID: "aria"}); err == nil {
		t.Fatal("store failure should surface")
	}
}
