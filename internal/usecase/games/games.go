// Package games lists, loads and deletes saved games.
package games

import (
	"context"
	"dnd/internal/domain"
	"fmt"
)

// Store reads and removes saved games.
type Store interface {
	Load(ctx context.Context, id string) (*domain.Game, error)
	List(ctx context.Context, accountID string) ([]domain.Summary, error)
	Delete(ctx context.Context, id string) error
}

// UseCase manages saved games.
type UseCase struct {
	store Store
}

// New wires the use case.
func New(store Store) *UseCase {
	return &UseCase{store: store}
}

// List returns the account's games, most recent first.
func (u *UseCase) List(ctx context.Context, accountID string) ([]domain.Summary, error) {
	out, err := u.store.List(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	return out, nil
}

// Get loads one game the account may see.
func (u *UseCase) Get(ctx context.Context, accountID, id string) (*domain.Game, error) {
	g, err := u.store.Load(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load game %s: %w", id, err)
	}
	if !g.CanView(accountID) {
		return nil, fmt.Errorf("%w: game %s", domain.ErrNotFound, id)
	}
	return g, nil
}

// Delete removes a saved game. Only its leader may delete it.
func (u *UseCase) Delete(ctx context.Context, accountID, id string) error {
	g, err := u.Get(ctx, accountID, id)
	if err != nil {
		return err
	}
	if g.LeaderID != accountID {
		return fmt.Errorf("%w: only the party leader can delete this game", domain.ErrForbidden)
	}
	if err := u.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete game %s: %w", id, err)
	}
	return nil
}
