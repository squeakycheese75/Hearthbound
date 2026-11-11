package tavern_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/tavern"
	"errors"
	"testing"
)

var (
	ctx     = context.Background()
	errBoom = errors.New("boom")
)

type world struct {
	games    []*domain.Game
	gamesErr error
	nameErr  map[string]bool
	ready    map[string]bool
	took     string
	takeErr  error
	offered  string
}

func (w *world) OfferHero(_ context.Context, accountID, gameID, characterID string) error {
	w.offered = accountID + " " + gameID + " " + characterID
	return w.takeErr
}

func (w *world) GamesOnTheBoard(context.Context) ([]*domain.Game, error) {
	return w.games, w.gamesErr
}

func (w *world) TakeSeat(_ context.Context, accountID, name, gameID, characterID string) error {
	w.took = accountID + " " + name + " " + gameID + " " + characterID
	return w.takeErr
}

func (w *world) Username(_ context.Context, id string) (string, error) {
	if w.nameErr[id] {
		return "", errBoom
	}
	return "name-of-" + id, nil
}

func (w *world) Ready(_ context.Context, id string) bool { return w.ready[id] }

func game(id, leader string, open ...string) *domain.Game {
	return &domain.Game{
		ID: id, LeaderID: leader, Title: "Tale " + id, Scene: domain.Scene{Title: "The Flagon"},
		Creatures: []*domain.Creature{
			{ID: id + "-a", Name: "Aria", Kind: domain.KindPC, OwnerID: leader},
			{ID: id + "-b", Name: "Bram", Kind: domain.KindPC, OwnerID: leader},
		},
		Party: []string{id + "-a", id + "-b"}, OpenSeats: open,
	}
}

func TestBoard(t *testing.T) {
	t.Parallel()
	w := &world{games: []*domain.Game{game("g1", "lead", "g1-b", "gone"), game("mine", "me", "mine-b")}}
	board, err := tavern.New(w, w, w, w).Board(ctx, "me")
	if err != nil || len(board) != 1 {
		t.Fatalf("other people's open seats only: %+v %v", board, err)
	}
	s := board[0]
	if s.GameID != "g1" || s.Leader != "name-of-lead" || s.Character.Name != "Bram" || len(s.Party) != 2 || s.Scene != "The Flagon" {
		t.Fatalf("seat %+v", s)
	}
	w.nameErr = map[string]bool{"lead": true}
	if _, err := tavern.New(w, w, w, w).Board(ctx, "me"); !errors.Is(err, errBoom) {
		t.Fatalf("name failure: %v", err)
	}
	w.gamesErr = errBoom
	if _, err := tavern.New(w, w, w, w).Board(ctx, "me"); !errors.Is(err, errBoom) {
		t.Fatalf("load failure: %v", err)
	}
}

func TestTake(t *testing.T) {
	t.Parallel()
	w := &world{ready: map[string]bool{"pip": true, "anon": true}, nameErr: map[string]bool{"anon": true}}
	uc := tavern.New(w, w, w, w)
	if err := uc.Take(ctx, "keyless", "g1", "g1-b"); !errors.Is(err, domain.ErrNoKey) || w.took != "" {
		t.Fatalf("no key, no seat: %v", err)
	}
	if err := uc.Take(ctx, "anon", "g1", "g1-b"); !errors.Is(err, errBoom) {
		t.Fatalf("name failure: %v", err)
	}
	if err := uc.Take(ctx, "pip", "g1", "g1-b"); err != nil || w.took != "pip name-of-pip g1 g1-b" {
		t.Fatalf("seated: %q %v", w.took, err)
	}
	w.takeErr = domain.ErrNotFound
	if err := uc.Take(ctx, "pip", "g1", "g1-b"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("seat gone: %v", err)
	}
}

func TestOffer(t *testing.T) {
	t.Parallel()
	w := &world{ready: map[string]bool{"pip": true}}
	uc := tavern.New(w, w, w, w)
	if err := uc.Offer(ctx, "keyless", "g1", "h1"); !errors.Is(err, domain.ErrNoKey) || w.offered != "" {
		t.Fatalf("no key, no offer: %v", err)
	}
	if err := uc.Offer(ctx, "pip", "g1", "h1"); err != nil || w.offered != "pip g1 h1" {
		t.Fatalf("offered %q %v", w.offered, err)
	}
	w.takeErr = errBoom
	if err := uc.Offer(ctx, "pip", "g1", "h1"); !errors.Is(err, errBoom) {
		t.Fatalf("offer failure: %v", err)
	}
}

func TestNotices(t *testing.T) {
	t.Parallel()
	posting := game("g1", "lead")
	posting.Wanted = "A healer"
	posting.Calls = []domain.Call{
		{Kind: domain.CallAsk, CharacterID: "h1", OwnerID: "me", Name: "Nim"},
		{Kind: domain.CallOffer, CharacterID: "h2", OwnerID: "me", Name: "Oslo"},
		{Kind: domain.CallAsk, CharacterID: "h3", OwnerID: "other", Name: "Pell"},
	}
	mine := game("mine", "me")
	mine.Wanted = "Anyone"
	mine.Calls = []domain.Call{{Kind: domain.CallOffer, CharacterID: "h4", OwnerID: "pip", Name: "Quill"}}
	w := &world{games: []*domain.Game{posting, mine}}
	n, err := tavern.New(w, w, w, w).Notices(ctx, "me")
	if err != nil || len(n.Posts) != 1 || len(n.Inbox) != 2 || len(n.Waiting) != 1 {
		t.Fatalf("notices %+v %v", n, err)
	}
	if p := n.Posts[0]; p.GameID != "g1" || p.Text != "A healer" || p.Leader != "name-of-lead" || p.Offered != "Oslo" || len(p.Party) != 2 {
		t.Fatalf("post %+v", p)
	}
	if a := n.Inbox[0]; a.Call.Name != "Nim" || a.From != "name-of-lead" || n.Inbox[1].From != "name-of-pip" {
		t.Fatalf("inbox %+v", n.Inbox)
	}
	if n.Waiting[0].Call.Name != "Oslo" {
		t.Fatalf("waiting %+v", n.Waiting)
	}
	for who, wantErr := range map[string]bool{"lead": true, "pip": true} {
		w.nameErr = map[string]bool{who: wantErr}
		if _, err := tavern.New(w, w, w, w).Notices(ctx, "me"); !errors.Is(err, errBoom) {
			t.Errorf("%s's name: %v", who, err)
		}
	}
	w.nameErr = map[string]bool{"lead": true}
	w.games = []*domain.Game{game("quiet", "lead")}
	if _, err := tavern.New(w, w, w, w).Notices(ctx, "me"); err != nil {
		t.Fatalf("no post, no names needed: %v", err)
	}
	w.games = []*domain.Game{{ID: "p", LeaderID: "lead", Wanted: "x"}}
	if _, err := tavern.New(w, w, w, w).Notices(ctx, "me"); !errors.Is(err, errBoom) {
		t.Fatalf("poster's name: %v", err)
	}
	w.gamesErr = errBoom
	if _, err := tavern.New(w, w, w, w).Notices(ctx, "me"); !errors.Is(err, errBoom) {
		t.Fatalf("load failure: %v", err)
	}
}
