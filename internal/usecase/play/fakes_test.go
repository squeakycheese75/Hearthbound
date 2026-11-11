package play_test

import (
	"context"
	"dnd/internal/adapter/agents"
	"dnd/internal/adapter/srd"
	"dnd/internal/domain"
	"dnd/internal/usecase/play"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memStore keeps JSON copies so tests see exactly what would be persisted.
type memStore struct {
	mu    sync.Mutex
	data  map[string][]byte
	chars map[string]domain.Character
	// joined and left record the party changes each save carried.
	joined  []string
	left    []string
	loadErr error
	saveErr error
	econ    *domain.Economy
	econErr error
	// gold records the ledger entries each save carried.
	gold []domain.LedgerEntry
}

func newStore(g *domain.Game) *memStore {
	s := &memStore{data: map[string][]byte{}}
	_ = s.Save(context.Background(), g)
	return s
}

func (s *memStore) Load(_ context.Context, id string) (*domain.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	var g domain.Game
	if err := json.Unmarshal(s.data[id], &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *memStore) Save(_ context.Context, g *domain.Game) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	b, _ := json.Marshal(g)
	s.data[g.ID] = b
	s.joined = append(s.joined, g.Joined...)
	s.gold = append(s.gold, g.GoldMoves...)
	g.GoldMoves = nil
	for _, c := range g.Left {
		s.left = append(s.left, c.ID)
	}
	g.Joined, g.Left = nil, nil
	return nil
}

func (s *memStore) Economy(context.Context) (domain.Economy, error) {
	if s.econ != nil {
		return *s.econ, s.econErr
	}
	return domain.DefaultEconomy, s.econErr
}

func (s *memStore) Character(_ context.Context, id string) (domain.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.chars[id]
	if !ok {
		return domain.Character{}, domain.ErrNotFound
	}
	return ch, nil
}

func (s *memStore) game(t *testing.T) *domain.Game {
	t.Helper()
	g, err := s.Load(context.Background(), "g1")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// recorder captures published events.
type recorder struct {
	mu     sync.Mutex
	events []domain.Event
}

func (r *recorder) Publish(_ string, e domain.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) texts(typ domain.EventType) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		if e.Type == typ {
			out = append(out, e.Text)
		}
	}
	return out
}

func (r *recorder) all() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

// rollLabels are the labels of every roll shown.
func (r *recorder) rollLabels() []string {
	var out []string
	for _, e := range r.all() {
		if e.Roll != nil {
			out = append(out, e.Roll.Label)
		}
	}
	return out
}

// cue is the sound cue on the system message containing substr.
func (r *recorder) cue(substr string) domain.Cue {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if (e.Type == domain.EvSystem || e.Type == domain.EvLevelUp) && strings.Contains(e.Text, substr) {
			return e.Cue
		}
	}
	return ""
}

func (r *recorder) has(typ domain.EventType, substr string) bool {
	for _, t := range r.texts(typ) {
		if strings.Contains(t, substr) {
			return true
		}
	}
	return false
}

// scriptAgent replies from a per-seat queue, falling back to a default line.
type scriptAgent struct {
	mu     sync.Mutex
	queue  map[string][]play.Reply
	errs   map[string]error
	turns  []play.Turn
	block  chan struct{}
	called chan struct{}
}

func newAgent() *scriptAgent {
	return &scriptAgent{queue: map[string][]play.Reply{}, errs: map[string]error{}}
}

func (a *scriptAgent) on(seat string, replies ...play.Reply) *scriptAgent {
	a.queue[seat] = append(a.queue[seat], replies...)
	return a
}

func (a *scriptAgent) Act(_ context.Context, t play.Turn) (play.Reply, error) {
	if a.called != nil {
		a.called <- struct{}{}
		<-a.block
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.turns = append(a.turns, t)
	if err := a.errs[t.Seat]; err != nil {
		return play.Reply{}, err
	}
	if t.OnText != nil {
		t.OnText("…")
	}
	q := a.queue[t.Seat]
	reply := say("The story continues.")
	if len(q) > 0 {
		reply, a.queue[t.Seat] = q[0], q[1:]
	}
	reply.Model = t.Model // the bridge reports the model that answered
	return reply, nil
}

func (a *scriptAgent) seatTurns(seat string) []play.Turn {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []play.Turn
	for _, t := range a.turns {
		if t.Seat == seat {
			out = append(out, t)
		}
	}
	return out
}

func say(text string) play.Reply {
	return play.Reply{
		Message: domain.AgentMessage{Role: domain.RoleAssistant, Text: text, Raw: json.RawMessage(`{}`)},
		Usage:   domain.TokenUsage{Input: 10, Output: 5},
	}
}

var callSeq atomic.Int64

func call(name, input string) play.Reply {
	r := say("")
	r.Message.ToolCalls = []domain.ToolCall{{ID: fmt.Sprintf("t%d", callSeq.Add(1)), Name: name, Input: json.RawMessage(input)}}
	return r
}

// dice returns a fixed value, or values from a sequence first.
type dice struct {
	mu    sync.Mutex
	seq   []int
	fixed int
}

func (d *dice) Roll(sides int) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	v := d.fixed
	if len(d.seq) > 0 {
		v, d.seq = d.seq[0], d.seq[1:]
	}
	return max(1, min(v, sides))
}

// easel records art requests.
type easel struct {
	mu   sync.Mutex
	keys []string
}

func (e *easel) Request(r domain.ArtRequest) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.keys = append(e.keys, r.Key)
}

func (e *easel) has(key string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, k := range e.keys {
		if k == key {
			return true
		}
	}
	return false
}

// clock is a movable fake clock.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock { return &clock{now: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// catalog wraps the real SRD catalog with failure switches and custom monsters.
type catalog struct {
	*srd.Catalog
	failClass bool
	monster   func(key, id string) (*domain.Creature, error)
}

func (c *catalog) Class(name string) (domain.ClassTemplate, error) {
	if c.failClass {
		return domain.ClassTemplate{}, errors.New("no classes")
	}
	return c.Catalog.Class(name)
}

func (c *catalog) Monster(key, id string) (*domain.Creature, error) {
	if c.monster != nil {
		return c.monster(key, id)
	}
	return c.Catalog.Monster(key, id)
}

func build(t *testing.T, id, name, class, race string, ctl domain.Controller, persona *domain.Persona) *domain.Creature {
	t.Helper()
	cat := srd.New()
	cls, _ := cat.Class(class)
	r, _ := cat.Race(race)
	c, err := domain.BuildCharacter(cls, r, domain.BuildSpec{ID: id, Name: name, Level: 1, Controller: ctl, Persona: persona})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// newGame returns a game with a human Fighter and an AI Cleric.
func newGame(t *testing.T) *domain.Game {
	t.Helper()
	persona := srd.New().Archetypes()[0]
	hero := build(t, "pc1", "Aria", "Fighter", "Human", domain.ControlHuman, nil)
	cleric := build(t, "pc2", "Mae", "Cleric", "Dwarf", domain.ControlAI, &persona)
	hero.OwnerID, cleric.OwnerID = me, me
	return &domain.Game{
		ID: "g1", LeaderID: me, Title: "Test", DMStyle: "heroic", Mode: domain.ModeExploration,
		Scene:     domain.Scene{Key: "tavern", Title: "Tavern"},
		Creatures: []*domain.Creature{hero, cleric}, Party: []string{"pc1", "pc2"},
		Transcripts: map[string][]domain.AgentMessage{},
	}
}

// roleBook serves the real agent definitions, with optional prompt overrides.
type roleBook struct {
	lib     *agents.Library
	prompts map[string]string
}

func newRoles(t *testing.T) *roleBook {
	t.Helper()
	lib, err := agents.New()
	if err != nil {
		t.Fatal(err)
	}
	return &roleBook{lib: lib, prompts: map[string]string{}}
}

func (r *roleBook) Role(name string) domain.AgentRole {
	role := r.lib.Role(name)
	if p, ok := r.prompts[name]; ok {
		role.Prompt = p
	}
	return role
}

// me is the signed-in leader in tests.
const me = "acct-me"

type harness struct {
	uc    *play.UseCase
	roles *roleBook
	store *memStore
	agent *scriptAgent
	pub   *recorder
	dice  *dice
	cat   *catalog
	art   *easel
	clock *clock
}

func setup(t *testing.T, g *domain.Game) *harness {
	t.Helper()
	h := &harness{roles: newRoles(t), store: newStore(g), agent: newAgent(), pub: &recorder{}, dice: &dice{fixed: 10}, cat: &catalog{Catalog: srd.New()}, art: &easel{}, clock: newClock()}
	h.uc = play.New(h.store, h.agent, h.roles, h.pub, h.dice, h.cat, h.art, h.clock, slog.New(slog.DiscardHandler), play.Config{})
	return h
}
