// Package stockart lists the stock illustrations the server ships with (scene
// backdrops, monster portraits, race-and-class portraits) and paints the
// missing ones once, so play itself never waits on, or pays for, an image.
package stockart

import (
	"context"
	"dnd/internal/domain"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
)

// Catalog supplies the races, classes and monster types to cover.
type Catalog interface {
	Races() []domain.RaceTemplate
	Classes() []domain.ClassTemplate
	MonsterKeys() []string
}

// Generator paints one image from a prompt.
type Generator interface {
	Generate(ctx context.Context, payer, prompt, size string) ([]byte, error)
}

// Store keeps finished stock images by art key.
type Store interface {
	Exists(key string) bool
	Save(key string, data []byte) error
}

// UseCase lists and paints one art pack's stock images.
type UseCase struct {
	pack    domain.ArtPack
	catalog Catalog
	gen     Generator
	store   Store
	log     *slog.Logger
}

// New wires the use case for one art pack; store holds that pack's images.
func New(pack domain.ArtPack, catalog Catalog, gen Generator, store Store, log *slog.Logger) *UseCase {
	return &UseCase{pack: pack, catalog: catalog, gen: gen, store: store, log: log}
}

// Item is one stock image and whether it has been painted yet.
type Item struct {
	domain.ArtRequest
	Done bool `json:"done"`
}

// List returns every stock image, optionally only of the given kinds.
func (u *UseCase) List(kinds ...domain.ArtKind) []Item {
	races := make([]string, 0, len(u.catalog.Races()))
	for _, r := range u.catalog.Races() {
		races = append(races, r.Name)
	}
	classes := make([]string, 0, len(u.catalog.Classes()))
	for _, c := range u.catalog.Classes() {
		classes = append(classes, c.Name)
	}
	var out []Item
	for _, r := range domain.StockList(u.pack, races, classes, u.catalog.MonsterKeys()) {
		if len(kinds) > 0 && !slices.Contains(kinds, r.Kind) {
			continue
		}
		out = append(out, Item{ArtRequest: r, Done: u.store.Exists(r.Key)})
	}
	return out
}

// Input picks what to paint. Empty Kinds means everything; Limit caps how many
// images this run paints (0 = no cap). Workers paint in parallel (at least 1).
type Input struct {
	Kinds   []domain.ArtKind
	Limit   int
	Workers int
}

// Output reports a run.
type Output struct {
	Painted int
	Failed  int
	Skipped int
}

// Execute paints the missing stock images. Failures are logged and counted,
// and the run carries on; it stops early only if ctx ends.
func (u *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	var todo []domain.ArtRequest
	out := Output{}
	for _, it := range u.List(in.Kinds...) {
		if it.Done || (in.Limit > 0 && len(todo) >= in.Limit) {
			out.Skipped++
			continue
		}
		todo = append(todo, it.ArtRequest)
	}
	jobs := make(chan domain.ArtRequest)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range max(in.Workers, 1) {
		wg.Go(func() {
			for r := range jobs {
				ok := u.paint(ctx, r)
				mu.Lock()
				out.Painted, out.Failed = out.Painted+btoi(ok), out.Failed+btoi(!ok)
				mu.Unlock()
			}
		})
	}
	for _, r := range todo {
		if ctx.Err() != nil {
			break
		}
		jobs <- r
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return out, fmt.Errorf("stock painting stopped: %w", err)
	}
	return out, nil
}

func (u *UseCase) paint(ctx context.Context, r domain.ArtRequest) bool {
	img, err := u.gen.Generate(ctx, "", r.Prompt, r.Size)
	if err == nil {
		err = u.store.Save(r.Key, img)
	}
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			u.log.Warn("stock art failed", "pack", u.pack.Key, "key", r.Key, "err", err)
		}
		return false
	}
	u.log.Info("stock art painted", "pack", u.pack.Key, "key", r.Key, "bytes", len(img))
	return true
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
