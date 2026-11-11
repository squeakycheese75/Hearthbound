// Package deadlines lets agents act for players who have been away too long.
package deadlines

import (
	"context"
	"dnd/internal/domain"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Due finds games whose deadline has passed.
type Due interface {
	DueGames(ctx context.Context, now time.Time) ([]string, error)
}

// Expirer resolves one overdue game.
type Expirer interface {
	Expire(ctx context.Context, gameID string) error
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// UseCase sweeps for overdue games.
type UseCase struct {
	due   Due
	play  Expirer
	clock Clock
	log   *slog.Logger
}

// New wires the use case.
func New(due Due, play Expirer, clock Clock, log *slog.Logger) *UseCase {
	return &UseCase{due: due, play: play, clock: clock, log: log}
}

// Sweep expires every overdue game once and reports how many it moved on.
// A game that is busy is skipped; the next sweep picks it up.
func (u *UseCase) Sweep(ctx context.Context) (int, error) {
	ids, err := u.due.DueGames(ctx, u.clock.Now())
	if err != nil {
		return 0, fmt.Errorf("find due games: %w", err)
	}
	moved := 0
	for _, id := range ids {
		err := u.play.Expire(ctx, id)
		switch {
		case errors.Is(err, domain.ErrBusy):
			u.log.Debug("deadline sweep: game busy, will retry", "game", id)
		case err != nil:
			u.log.Warn("deadline sweep: agents could not act", "game", id, "err", err)
		default:
			moved++
		}
	}
	return moved, nil
}
