package deadlines_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/deadlines"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type due struct {
	ids  []string
	err  error
	seen time.Time
}

func (d *due) DueGames(_ context.Context, now time.Time) ([]string, error) {
	d.seen = now
	return d.ids, d.err
}

type expirer struct {
	errs map[string]error
	got  []string
}

func (e *expirer) Expire(_ context.Context, id string) error {
	e.got = append(e.got, id)
	return e.errs[id]
}

type clock struct{}

var now = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func (clock) Now() time.Time { return now }

func TestSweep(t *testing.T) {
	t.Parallel()
	d := &due{ids: []string{"ok", "busy", "broken"}}
	e := &expirer{errs: map[string]error{"busy": domain.ErrBusy, "broken": errors.New("no key")}}
	uc := deadlines.New(d, e, clock{}, slog.New(slog.DiscardHandler))
	moved, err := uc.Sweep(context.Background())
	if err != nil || moved != 1 || len(e.got) != 3 || !d.seen.Equal(now) {
		t.Fatalf("moved %d err %v got %v", moved, err, e.got)
	}
	d.err = errors.New("db down")
	if _, err := uc.Sweep(context.Background()); err == nil {
		t.Fatal("store failure should surface")
	}
}
