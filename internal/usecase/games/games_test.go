package games_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/games"
	"errors"
	"testing"
)

type fakeStore struct {
	err       error
	deleteErr error
	deleted   string
}

func (f *fakeStore) Load(_ context.Context, id string) (*domain.Game, error) {
	return &domain.Game{
		ID: id, LeaderID: "me", Party: []string{"pc2"},
		Creatures: []*domain.Creature{{ID: "pc2", OwnerID: "friend", Kind: domain.KindPC}},
	}, f.err
}

func (f *fakeStore) List(context.Context, string) ([]domain.Summary, error) {
	return []domain.Summary{{ID: "a"}}, f.err
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	f.deleted = id
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return f.err
}

var ctx = context.Background()

func TestHappyPath(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	uc := games.New(store)
	list, err := uc.List(ctx, "me")
	if err != nil || len(list) != 1 {
		t.Fatalf("list %v %v", list, err)
	}
	g, err := uc.Get(ctx, "me", "x")
	if err != nil || g.ID != "x" {
		t.Fatalf("get %v %v", g, err)
	}
	if _, err := uc.Get(ctx, "friend", "x"); err != nil {
		t.Fatalf("a party member's owner may view the game: %v", err)
	}
	if _, err := uc.Get(ctx, "stranger", "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("strangers must not see the game: %v", err)
	}
	if err := uc.Delete(ctx, "friend", "x"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("only the leader deletes: %v", err)
	}
	if err := uc.Delete(ctx, "stranger", "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stranger delete: %v", err)
	}
	if err := uc.Delete(ctx, "me", "x"); err != nil || store.deleted != "x" {
		t.Fatalf("delete %v", err)
	}
}

func TestErrorsAreWrapped(t *testing.T) {
	t.Parallel()
	uc := games.New(&fakeStore{err: domain.ErrNotFound})
	if _, err := uc.List(ctx, "me"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("list %v", err)
	}
	if _, err := uc.Get(ctx, "me", "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get %v", err)
	}
	if err := uc.Delete(ctx, "me", "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete %v", err)
	}
}

func TestDeleteStoreFailure(t *testing.T) {
	t.Parallel()
	uc := games.New(&fakeStore{deleteErr: errors.New("disk")})
	if err := uc.Delete(ctx, "me", "x"); err == nil {
		t.Fatal("expected delete failure")
	}
}
