// Package play runs the table: the DM agent narrates and adjudicates through
// rules-engine tools, AI companions speak and act in character, and the human
// hero acts through the UI.
package play

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Agent is one LLM-backed seat at the table.
type Agent interface {
	Act(ctx context.Context, turn Turn) (Reply, error)
}

// Turn is one request to an agent. History is append-only and already contains
// the latest user message. OnText receives streamed narration as it arrives.
type Turn struct {
	Seat string
	// Role names the agent definition this call runs under (domain.Role*).
	Role string
	// Model is the role's model; the bridge may swap in the payer's own choice.
	Model string
	// Payer is the account whose API key and budget this call uses.
	Payer   string
	System  string
	Tools   []domain.ToolSpec
	History []domain.AgentMessage
	Effort  string
	OnText  func(delta string)
}

// Reply is the agent's completed message plus token usage and the model that
// actually answered.
type Reply struct {
	Message domain.AgentMessage
	Usage   domain.TokenUsage
	Model   string
}

// Roles supplies each kind of agent's definition: model, effort, tools and
// prompt template.
type Roles interface {
	Role(name string) domain.AgentRole
}

// Store loads and saves games, and reads roster characters for recruiting.
// Save must embark g.Joined and settle g.Left in the same transaction.
type Store interface {
	Load(ctx context.Context, id string) (*domain.Game, error)
	Save(ctx context.Context, g *domain.Game) error
	Character(ctx context.Context, id string) (domain.Character, error)
	// Economy is the realm owner's settings (the loot rate scales gold rewards).
	Economy(ctx context.Context) (domain.Economy, error)
}

// Publisher pushes live events to whoever is watching a game.
type Publisher interface {
	Publish(gameID string, e domain.Event)
}

// Catalog supplies monsters, DM styles and templates for levelling up.
type Catalog interface {
	Monster(key, id string) (*domain.Creature, error)
	MonsterSummary() string
	DMStyle(key string) domain.DMStyle
	Class(name string) (domain.ClassTemplate, error)
	Items() []domain.BaseItem
	Item(key string) (domain.BaseItem, error)
	ItemByName(name string) (domain.BaseItem, bool)
}

// Illustrator paints portraits and backdrops in the background. Requests never block.
type Illustrator interface {
	Request(r domain.ArtRequest)
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// Config tunes the agents. Models, effort and prompts come from Roles.
type Config struct {
	MaxToolSteps int
	// CompactAt is the DM context size (tokens) that closes a chapter; companions
	// close one at CompactAt*5/12. Zero means 60,000.
	CompactAt int
}

// UseCase runs games.
type UseCase struct {
	store   Store
	agent   Agent
	roles   Roles
	pub     Publisher
	roller  rules.Roller
	catalog Catalog
	clock   Clock
	art     Illustrator
	cfg     Config
	log     *slog.Logger

	mu        sync.Mutex
	locks     map[string]*sync.Mutex
	active    map[string]bool
	templates map[string]*template.Template
}

// New wires the use case.
// Logs carry metadata only (seats, tools, timings, usage), never story text;
// read transcripts through the store when you need the words.
func New(store Store, agent Agent, roles Roles, pub Publisher, roller rules.Roller, catalog Catalog, art Illustrator, clock Clock, log *slog.Logger, cfg Config) *UseCase {
	if cfg.MaxToolSteps <= 0 {
		cfg.MaxToolSteps = 12
	}
	if cfg.CompactAt <= 0 {
		cfg.CompactAt = 60_000
	}
	return &UseCase{
		store: store, agent: agent, roles: roles, pub: pub, roller: roller, catalog: catalog, art: art, clock: clock, cfg: cfg, log: log,
		locks: map[string]*sync.Mutex{}, active: map[string]bool{}, templates: map[string]*template.Template{},
	}
}

// Options returns the legal combat options for the viewer's character, or nil
// when it is not one of the viewer's live seats' turn.
func (u *UseCase) Options(ctx context.Context, accountID, gameID string) ([]rules.Option, error) {
	g, err := u.store.Load(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("load game: %w", err)
	}
	if !g.CanView(accountID) {
		return nil, fmt.Errorf("%w: game %s", domain.ErrNotFound, gameID)
	}
	if g.Mode != domain.ModeCombat {
		return nil, nil
	}
	c, err := g.Creature(g.Encounter.Current())
	if err != nil || domain.PlayerOf(c) != accountID || !domain.Live(c) {
		return nil, nil
	}
	return rules.Options(g, c.ID), nil
}

// Begin has the DM open the adventure. It does nothing if the game already started.
func (u *UseCase) Begin(ctx context.Context, accountID, gameID string) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.Started {
			return nil
		}
		s.g.Started = true
		for _, c := range s.g.PartyMembers() {
			s.u.art.Request(domain.PortraitFor(s.g, c))
		}
		return s.dmTurn(ctx, openingMessage(s.g))
	})
}

// Act submits the player's action for this exploration round. The round
// resolves once every live seat has acted (or its deadline passes).
func (u *UseCase) Act(ctx context.Context, accountID, gameID, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("%w: say or do something", domain.ErrInvalid)
	}
	if n := len([]rune(text)); n > domain.MaxAction {
		return fmt.Errorf("%w: keep it under %d characters (this is %d)", domain.ErrInvalid, domain.MaxAction, n)
	}
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.Mode != domain.ModeExploration {
			return fmt.Errorf("%w: exploration actions are not allowed in %s", domain.ErrWrongMode, s.g.Mode)
		}
		if !s.g.MayType(accountID) && !slices.Contains(s.g.ChoicesNow(), text) {
			return fmt.Errorf("%w: this table plays from the quick choices", domain.ErrForbidden)
		}
		c, err := s.owedSeat(accountID)
		if err != nil {
			return err
		}
		if s.g.Pending == nil {
			s.g.Pending = map[string]string{}
		}
		s.g.Pending[c.ID] = text
		s.back(c)
		s.emit(domain.Event{Type: domain.EvPlayer, Actor: c.ID, Text: text})
		if waiting := s.unsubmitted(); len(waiting) > 0 {
			s.system("Waiting for " + strings.Join(waiting, ", ") + " to act.")
			return nil
		}
		if !s.g.Playing() {
			return s.pause() // the action waits for the leader to return
		}
		return s.resolveRound(ctx)
	})
}

// SetTyping sets who may type free-text actions at the table (the leader
// only decides). Everyone else picks from the quick choices.
func (u *UseCase) SetTyping(ctx context.Context, accountID, gameID string, mode domain.TypingMode) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.LeaderID != accountID {
			return fmt.Errorf("%w: only the party's leader sets this", domain.ErrForbidden)
		}
		s.g.Typing = mode
		s.system(typingNote[mode])
		return nil
	})
}

var typingNote = map[domain.TypingMode]string{
	domain.TypeEveryone: "Everyone can type their own actions.",
	domain.TypeLeader:   "Only the leader types actions now; everyone else picks from the quick choices.",
	domain.TypeChoices:  "This table now plays from the quick choices only.",
}

// Fight carries out the player's chosen option on their character's combat turn.
func (u *UseCase) Fight(ctx context.Context, accountID, gameID string, opt rules.Option) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.Mode != domain.ModeCombat {
			return fmt.Errorf("%w: not in combat", domain.ErrWrongMode)
		}
		c, err := s.g.Creature(s.g.Encounter.Current())
		if err != nil || domain.PlayerOf(c) != accountID || !domain.Live(c) {
			return fmt.Errorf("%w: it is not your character's turn", domain.ErrNotYourTurn)
		}
		s.back(c)
		over, err := s.takeTurn(c, opt, "")
		if err != nil || !over {
			return err // a bonus action or free feature leaves the turn open
		}
		if err := s.advanceAndNarrate(ctx); err != nil {
			return err
		}
		return s.runCombat(ctx)
	})
}

// Autopilot hands one of the player's characters to its agent (on), or makes
// it the character they play (off), handing their others to agents. Handing
// over mid-turn lets the agent act straight away. When no one is left playing,
// the story pauses until someone takes control again.
func (u *UseCase) Autopilot(ctx context.Context, accountID, gameID, characterID string, on bool) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		c, err := s.g.SeatFor(accountID, characterID)
		if err != nil {
			return err
		}
		if on {
			s.handOver(c)
			return s.proceed(ctx)
		}
		if err := s.takeControl(accountID, c); err != nil {
			return err
		}
		return s.proceed(ctx)
	})
}

// Cover sets whether a player's agent may stand in for one of their live
// characters when they miss a deadline. Without it the character holds back
// until its player returns.
func (u *UseCase) Cover(ctx context.Context, accountID, gameID, characterID string, allow bool) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		c, err := s.g.SeatFor(accountID, characterID)
		if err != nil {
			return err
		}
		if allow {
			delete(s.g.Hold, c.ID)
			return nil
		}
		if s.g.Hold == nil {
			s.g.Hold = map[string]bool{}
		}
		s.g.Hold[c.ID] = true
		return nil
	})
}

// Limits returns the prompt sizes (tokens) at which the DM and a companion
// close a chapter.
func (u *UseCase) Limits() (dm, companion int) {
	return u.cfg.CompactAt, u.cfg.CompactAt * 5 / 12
}

// Equip puts one of the account's characters' items in a slot (an empty slot
// takes it off), outside combat, and recalculates their numbers.
func (u *UseCase) Equip(ctx context.Context, accountID, gameID, characterID, itemID string, slot domain.Slot) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		c, err := s.g.SeatFor(accountID, characterID)
		if err != nil {
			return err
		}
		if s.g.Mode == domain.ModeCombat {
			return fmt.Errorf("%w: change gear once the fight is over", domain.ErrWrongMode)
		}
		cls, err := u.catalog.Class(c.Class)
		if err != nil {
			return fmt.Errorf("class: %w", err)
		}
		if err := c.Equip(itemID, slot, cls, u.itemLookup); err != nil {
			return err
		}
		c.Recalculate(cls, u.itemLookup)
		return nil
	})
}

// Improve spends one of a hero's Ability Score Improvements: +2 to one
// ability, or +1 to two.
func (u *UseCase) Improve(ctx context.Context, accountID, gameID, characterID string, inc map[domain.Ability]int) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		c, err := s.g.SeatFor(accountID, characterID)
		if err != nil {
			return err
		}
		cls, err := u.catalog.Class(c.Class)
		if err != nil {
			return fmt.Errorf("class: %w", err)
		}
		if err := c.Improve(inc); err != nil {
			return err
		}
		c.Recalculate(cls, u.itemLookup)
		s.system(fmt.Sprintf("%s grows stronger: %s.", c.Name, domain.ImprovementText(inc)))
		return nil
	})
}

// outfit brings a hero's sheet up to date on load: equipment for older
// sheets, then derived numbers and class features.
func (u *UseCase) outfit(c *domain.Creature) {
	if c.Kind != domain.KindPC {
		return
	}
	cls, err := u.catalog.Class(c.Class)
	if err != nil {
		return // older or odd sheets keep their numbers
	}
	if !c.Outfitted {
		c.Outfit(cls, u.catalog.ItemByName, u.itemLookup)
	}
	c.Recalculate(cls, u.itemLookup)
}

func (u *UseCase) itemLookup(key string) (domain.BaseItem, bool) {
	b, err := u.catalog.Item(key)
	return b, err == nil
}

// Expire lets agents act for live players who missed the deadline. It is called
// by the deadline sweeper and does nothing if the deadline has not passed.
func (u *UseCase) Expire(ctx context.Context, gameID string) error {
	return u.run(ctx, gameID, nil, func(s *session) error {
		if s.g.Deadline.IsZero() || u.clock.Now().Before(s.g.Deadline) {
			return nil
		}
		s.g.Deadline = time.Time{}
		if s.g.Mode == domain.ModeOver || !s.g.Playing() {
			return nil
		}
		s.system("Time is up: agents act for anyone who is away.")
		if s.g.Mode == domain.ModeExploration {
			return s.resolveRound(ctx)
		}
		s.autoTurn = s.g.Encounter.Current()
		return s.runCombat(ctx)
	})
}

// Busy reports whether a turn is being resolved for the game right now. The
// saved game cannot say this itself because it is only written when a turn ends.
func (u *UseCase) Busy(gameID string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.active[gameID]
}

// lock claims the game for one turn. The returned release must be called when done.
func (u *UseCase) lock(gameID string) (func(), bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	m, ok := u.locks[gameID]
	if !ok {
		m = &sync.Mutex{}
		u.locks[gameID] = m
	}
	if !m.TryLock() {
		return nil, false
	}
	u.active[gameID] = true
	return func() {
		u.mu.Lock()
		delete(u.active, gameID)
		u.mu.Unlock()
		m.Unlock()
	}, true
}

// withGame runs fn for a player who must be part of the game.
func (u *UseCase) withGame(ctx context.Context, accountID, gameID string, fn func(*session) error) error {
	return u.run(ctx, gameID, func(g *domain.Game) error {
		if !g.CanView(accountID) {
			return fmt.Errorf("%w: game %s", domain.ErrNotFound, gameID)
		}
		return nil
	}, fn)
}

// run serialises work on a game, then schedules the next deadline, saves it and
// tells the clients they may act. check (optional) guards access.
func (u *UseCase) run(ctx context.Context, gameID string, check func(*domain.Game) error, fn func(*session) error) error {
	release, ok := u.lock(gameID)
	if !ok {
		return domain.ErrBusy
	}
	defer release()
	s, err := u.open(ctx, gameID, check)
	if err != nil {
		return err
	}
	g := s.g
	g.Awaiting = false
	runErr := fn(s)
	if runErr != nil {
		s.emit(domain.Event{Type: domain.EvError, Text: runErr.Error()})
	}
	if runErr == nil {
		s.compact(ctx)
	}
	s.schedule()
	g.Awaiting = g.Mode != domain.ModeOver
	g.UpdatedAt = u.clock.Now()
	u.log.Debug("saving game", "game", g.ID, "mode", g.Mode, "awaiting", g.Awaiting, "log_events", len(g.Log),
		"tokens_in", g.Usage.Input, "tokens_out", g.Usage.Output, "cache_read", g.Usage.CacheRead, "cache_write", g.Usage.CacheWrite)
	if err := u.store.Save(ctx, g); err != nil {
		return fmt.Errorf("save game: %w", err)
	}
	s.emit(domain.Event{Type: domain.EvState})
	if g.Awaiting {
		s.emit(domain.Event{Type: domain.EvAwaiting})
	}
	return runErr
}

// open loads a game for a turn: checks access, brings older sheets up to
// date (items, equipment) and reads the realm's economy.
func (u *UseCase) open(ctx context.Context, gameID string, check func(*domain.Game) error) (*session, error) {
	g, err := u.store.Load(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("load game: %w", err)
	}
	if check != nil {
		if err := check(g); err != nil {
			return nil, err
		}
	}
	for _, c := range g.Creatures {
		c.AdoptLegacyItems(u.catalog.ItemByName) // in case the store didn't
		u.outfit(c)
	}
	econ, err := u.store.Economy(ctx)
	if err != nil {
		return nil, fmt.Errorf("load economy: %w", err)
	}
	return &session{u: u, g: g, econ: econ}, nil
}

// session is the state of one in-flight request against a loaded game.
type session struct {
	u *UseCase
	g *domain.Game
	// endTurn is set by a tool that finishes the agent's turn (offer_choices),
	// so the loop stops without another model call.
	endTurn bool
	// spoke is true once the current agent run has shown players some text.
	spoke bool
	// autoTurn names a live seat whose current combat turn its agent plays
	// because the owner missed the deadline.
	autoTurn string
	// xpGiven and goldGiven count this turn's DM rewards, which are capped
	// by party level (rules.TurnXPCap, rules.TurnGoldCap).
	xpGiven, goldGiven int
	// lootGiven counts this turn's rolled items (rules.TurnLootCap).
	lootGiven int
	// econ is the realm's economy settings for this turn.
	econ domain.Economy
}

func (s *session) emit(e domain.Event) {
	if !e.Transient() {
		s.g.AppendLog(e)
	}
	s.u.pub.Publish(s.g.ID, e)
}

func (s *session) system(text string) {
	s.emit(domain.Event{Type: domain.EvSystem, Text: text})
}

// cue is a system message with a sound effect.
func (s *session) cue(cue domain.Cue, text string) {
	s.emit(domain.Event{Type: domain.EvSystem, Text: text, Cue: cue})
}

func (s *session) notes(notes []string) {
	for _, n := range notes {
		s.system(n)
	}
}

func (s *session) roll(actor string, r domain.Roll) {
	s.emit(domain.Event{Type: domain.EvRoll, Actor: actor, Roll: &r, Text: r.Label})
}
