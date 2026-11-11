package main

import (
	"context"
	"dnd/internal/adapter/hub"
	"dnd/internal/adapter/media"
	"dnd/internal/adapter/openai"
	"dnd/internal/adapter/secure"
	"dnd/internal/adapter/sqlite"
	"dnd/internal/adapter/srd"
	"dnd/internal/adapter/system"
	"dnd/internal/domain"
	"dnd/internal/usecase/accounts"
	"dnd/internal/usecase/artwork"
	"dnd/internal/usecase/market"
	"dnd/internal/usecase/stockart"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// Art modes (-art).
const (
	artStock = "stock" // shipped stock images only: no painting, no image cost
	artPaint = "paint" // paint custom art with players' OpenAI keys, stock until ready
	artOff   = "off"   // SVG fallbacks only
)

// newArt builds the illustration service for the chosen -art mode.
func newArt(ctx context.Context, cfg config, keys keyring, pictures *media.Store, ui fs.FS, events *hub.Hub, store *sqlite.Store, log *slog.Logger) (*artwork.UseCase, error) {
	pack, err := domain.ArtPackFor(cfg.artPack)
	if err != nil {
		return nil, err
	}
	var gen artwork.Generator // nil interface: no painting
	var stock artwork.Stock = media.NewShelf(ui, pack.Key)
	switch cfg.art {
	case artPaint:
		gen = painter{keys: keys, model: cfg.imageModel, quality: cfg.imageQuality}
		log.Info("art: painting custom illustrations with each player's OpenAI key, stock until ready", "model", cfg.imageModel, "quality", cfg.imageQuality)
	case artOff:
		stock = nil
		log.Info("art: off (built-in drawings only)")
	default:
		log.Info("art: stock images (no painting); use -art paint for custom illustrations", "pack", pack.Key, "style", pack.Name)
	}
	art := artwork.New(gen, pictures, stock, events, store, log, 64)
	for range max(cfg.artWorkers, 1) {
		go art.Run(ctx)
	}
	return art, nil
}

// stockTool handles -stock-list and -paint-stock, which run instead of the server.
func stockTool(ctx context.Context, cfg config, log *slog.Logger) error {
	pack, err := domain.ArtPackFor(cfg.artPack)
	if err != nil {
		return err
	}
	shelf, err := media.New(filepath.Join(cfg.stockDir, pack.Key))
	if err != nil {
		return err
	}
	gen := &keyPainter{model: cfg.imageModel, quality: cfg.imageQuality}
	uc := stockart.New(pack, srd.New(), gen, shelf, log)
	if cfg.stockList {
		fmt.Printf("Art pack %q (%s)\n%s\n\n", pack.Key, pack.Name, pack.Style)
	}
	kinds, err := parseKinds(cfg.paintStock)
	if err != nil {
		return err
	}
	if cfg.stockList {
		return printStock(uc.List(kinds...))
	}
	if gen.key, err = stockKey(ctx, cfg); err != nil {
		return err
	}
	out, err := uc.Execute(ctx, stockart.Input{Kinds: kinds, Limit: cfg.stockLimit, Workers: cfg.artWorkers})
	log.Info("stock painting done", "painted", out.Painted, "failed", out.Failed, "skipped", out.Skipped, "dir", cfg.stockDir)
	return err
}

func parseKinds(s string) ([]domain.ArtKind, error) {
	var out []domain.ArtKind
	for k := range strings.SplitSeq(s, ",") {
		switch k = strings.TrimSuffix(strings.TrimSpace(k), "s"); k {
		case "", "all":
		case string(domain.ArtScene), string(domain.ArtBattlemap), string(domain.ArtMonster), string(domain.ArtPortrait), string(domain.ArtNPC):
			out = append(out, domain.ArtKind(k))
		default:
			return nil, fmt.Errorf("%w: stock kind %q (use scenes, battlemaps, monsters, portraits, npcs or all)", domain.ErrUnknown, k)
		}
	}
	return out, nil
}

func printStock(items []stockart.Item) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	done := 0
	for _, it := range items {
		state := "missing"
		if it.Done {
			state, done = "done", done+1
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", it.Kind, it.Key, state)
	}
	_, _ = fmt.Fprintf(w, "\n%d of %d painted\n", done, len(items)) // tabwriter buffers; Flush reports errors
	return w.Flush()
}

// stockKey finds the OpenAI key to paint stock art with: OPENAI_API_KEY, else
// the key saved in Settings by the -stock-account user (default "local").
func stockKey(ctx context.Context, cfg config) (string, error) {
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return k, nil
	}
	store, err := sqlite.Open(ctx, cfg.db)
	if err != nil {
		return "", err
	}
	defer func() { _ = store.Close() }()
	vault, err := openVault(filepath.Dir(cfg.db), slog.New(slog.DiscardHandler))
	if err != nil {
		return "", err
	}
	people := accounts.New(store, secure.Passwords{}, vault, secure.Tokens{}, system.Clock{})
	a, err := store.AccountByUsername(ctx, cfg.stockAccount)
	if err != nil {
		return "", fmt.Errorf("painting stock art needs OPENAI_API_KEY or an OpenAI key saved in Settings by %q: %w", cfg.stockAccount, err)
	}
	key, err := people.Credential(ctx, a.ID, domain.KeyOpenAI)
	if err != nil {
		return "", fmt.Errorf("painting stock art needs OPENAI_API_KEY or an OpenAI key saved in Settings by %q: %w", cfg.stockAccount, err)
	}
	return key, nil
}

// keyPainter paints with the server operator's key: stock art is painted once,
// by whoever builds the server, and bills no player.
type keyPainter struct {
	key     string
	model   string
	quality string
}

func (p *keyPainter) Generate(ctx context.Context, _, prompt, size string) ([]byte, error) {
	c := openai.New(openai.Config{APIKey: p.key, Model: p.model, Quality: p.quality})
	return c.Generate(ctx, prompt, size)
}

// marketStore lets the market run trades in SQLite transactions (the adapter
// can't name the use case's Tx type, so the bridge lives here).
type marketStore struct{ *sqlite.Store }

func (m marketStore) Atomically(ctx context.Context, fn func(market.Tx) error) error {
	return m.InTx(ctx, func(tx *sqlite.Tx) error { return fn(tx) })
}
