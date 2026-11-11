// Package artwork paints character portraits, monster portraits and scene
// backdrops in the background, so play never waits on an image.
package artwork

import (
	"context"
	"dnd/internal/domain"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// Generator turns a prompt into encoded image bytes, billed to the payer account.
// Ready reports whether the payer can be billed at all (for example, has a key).
type Generator interface {
	Ready(ctx context.Context, payer string) bool
	Generate(ctx context.Context, payer, prompt, size string) ([]byte, error)
}

// Media stores finished images by art key.
type Media interface {
	Exists(key string) bool
	Save(key string, data []byte) error
	URL(key string) (string, bool)
}

// Stock finds pre-painted art shipped with the server.
type Stock interface {
	URL(key string) (string, bool)
}

// Publisher tells a game's viewers that an image is ready.
type Publisher interface {
	Publish(gameID string, e domain.Event)
}

// GameLoader reads a game, for rerolls.
type GameLoader interface {
	Load(ctx context.Context, id string) (*domain.Game, error)
}

type job struct {
	req   domain.ArtRequest
	force bool
}

// UseCase queues and paints illustrations. A nil Generator disables it.
type UseCase struct {
	gen   Generator
	media Media
	stock Stock
	pub   Publisher
	games GameLoader
	log   *slog.Logger
	queue chan job

	mu      sync.Mutex
	pending map[string]bool
}

// New wires the use case. queueSize bounds how many paintings may wait. A nil
// Stock means no stock art.
func New(gen Generator, media Media, stock Stock, pub Publisher, games GameLoader, log *slog.Logger, queueSize int) *UseCase {
	return &UseCase{
		gen: gen, media: media, stock: stock, pub: pub, games: games, log: log,
		queue: make(chan job, max(queueSize, 1)), pending: map[string]bool{},
	}
}

// Enabled reports whether images are being generated at all.
func (u *UseCase) Enabled() bool {
	return u.gen != nil
}

// CanPaint reports whether images can be painted on this account's key, so the
// UI only offers repaints to players who can pay for them.
func (u *UseCase) CanPaint(ctx context.Context, accountID string) bool {
	return u.Enabled() && u.gen.Ready(ctx, accountID)
}

// Request queues an illustration unless it already exists or is being painted.
func (u *UseCase) Request(r domain.ArtRequest) {
	if !u.Enabled() || u.media.Exists(r.Key) || !u.gen.Ready(context.Background(), r.Payer) {
		return
	}
	u.enqueue(job{req: r})
}

func (u *UseCase) enqueue(j job) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.pending[j.req.Key] {
		return true
	}
	select {
	case u.queue <- j:
		u.pending[j.req.Key] = true
		u.log.Debug("art queued", "key", j.req.Key, "kind", j.req.Kind, "force", j.force)
		return true
	default:
		u.log.Warn("art queue full, skipping", "key", j.req.Key)
		return false
	}
}

// Ensure requests any art the game is missing: party portraits, portraits for
// monsters on the field and the current scene. It is cheap to call on every view,
// and it is what retries paintings lost to a restart.
func (u *UseCase) Ensure(g *domain.Game) {
	if !u.Enabled() {
		return
	}
	for _, c := range g.Creatures {
		if c.Dead {
			continue
		}
		if c.Kind == domain.KindMonster {
			u.Request(domain.MonsterFor(g, c))
			continue
		}
		u.Request(domain.PortraitFor(g, c))
	}
	if g.Scene.Key != "" {
		u.Request(domain.SceneFor(g))
	}
	for _, n := range g.NPCsHere() {
		u.Request(domain.NPCPortrait(g, n))
	}
}

// EnsureCharacter requests a roster character's portrait if it has none.
func (u *UseCase) EnsureCharacter(ch *domain.Character) {
	if !u.Enabled() || ch.Sheet == nil {
		return
	}
	u.Request(domain.CharacterPortrait(ch))
}

// Pending reports whether an image is queued or being painted.
func (u *UseCase) Pending(key string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.pending[key]
}

// Picture returns where to find a subject's image: its own painting if there
// is one, else the first stock image the pack has (best match first).
func (u *UseCase) Picture(key string, stockKeys ...string) (string, bool) {
	if url, ok := u.media.URL(key); ok {
		return url, true
	}
	if u.stock == nil {
		return "", false
	}
	for _, k := range stockKeys {
		if url, ok := u.stock.URL(k); ok && k != "" {
			return url, true
		}
	}
	return "", false
}

// Reroll repaints a party member's or monster's portrait, or the current scene
// when subjectID is domain.SceneSubject.
func (u *UseCase) Reroll(ctx context.Context, accountID, gameID, subjectID string) error {
	if !u.Enabled() {
		return fmt.Errorf("%w: image generation is not configured", domain.ErrInvalid)
	}
	g, err := u.games.Load(ctx, gameID)
	if err != nil {
		return fmt.Errorf("load game: %w", err)
	}
	if !g.CanView(accountID) {
		return fmt.Errorf("%w: game %s", domain.ErrNotFound, gameID)
	}
	req, err := requestFor(g, subjectID)
	if err != nil {
		return err
	}
	if !u.gen.Ready(ctx, req.Payer) {
		return fmt.Errorf("%w: the paying player needs an OpenAI key in Settings", domain.ErrNoKey)
	}
	if !u.enqueue(job{req: req, force: true}) {
		return fmt.Errorf("%w: too many images are being painted, try again shortly", domain.ErrBusy)
	}
	return nil
}

func requestFor(g *domain.Game, subjectID string) (domain.ArtRequest, error) {
	if subjectID == domain.SceneSubject {
		return domain.SceneFor(g), nil
	}
	c, err := g.Creature(subjectID)
	if err != nil {
		return domain.ArtRequest{}, err
	}
	if c.Kind == domain.KindMonster {
		return domain.MonsterFor(g, c), nil
	}
	return domain.PortraitFor(g, c), nil
}

// Run paints queued images until ctx ends. Start one per parallel worker.
func (u *UseCase) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-u.queue:
			u.paint(ctx, j)
		}
	}
}

func (u *UseCase) paint(ctx context.Context, j job) {
	defer u.done(j.req.Key)
	if !j.force && u.media.Exists(j.req.Key) {
		return
	}
	img, err := u.gen.Generate(ctx, j.req.Payer, j.req.Prompt, j.req.Size)
	if errors.Is(err, context.Canceled) {
		u.log.Info("art cancelled by shutdown; it will be retried when the game is next opened", "key", j.req.Key)
		return
	}
	if err != nil {
		u.log.Warn("art failed", "key", j.req.Key, "err", err)
		return
	}
	if err := u.media.Save(j.req.Key, img); err != nil {
		u.log.Warn("art not saved", "key", j.req.Key, "err", err)
		return
	}
	u.log.Debug("art ready", "key", j.req.Key, "bytes", len(img))
	u.pub.Publish(j.req.GameID, domain.Event{Type: domain.EvImage, Text: j.req.Key})
}

func (u *UseCase) done(key string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.pending, key)
}
