// Package newgame starts an adventure with characters from the player's roster.
package newgame

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"strings"
	"time"
)

// Catalog supplies SRD templates and presets for the setup screens.
type Catalog interface {
	Classes() []domain.ClassTemplate
	Races() []domain.RaceTemplate
	Archetypes() []domain.Persona
	DMStyles() []domain.DMStyle
	DMStyle(key string) domain.DMStyle
}

// Store loads characters and saves the new adventure together with their
// updated statuses, atomically.
type Store interface {
	Character(ctx context.Context, id string) (domain.Character, error)
	CreateAdventure(ctx context.Context, g *domain.Game, party []domain.Character) error
}

// Illustrator paints portraits in the background. Requests never block.
type Illustrator interface {
	Request(r domain.ArtRequest)
}

// Agents reports whether an account has registered the AI (an Anthropic key)
// that will run its table: the leader's key runs the DM and their companions.
type Agents interface {
	Ready(ctx context.Context, accountID string) bool
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// IDGen makes unique IDs.
type IDGen interface {
	New() string
}

// Input describes the adventure to start. AccountID is the signed-in leader.
type Input struct {
	AccountID    string   `json:"-"`
	Title        string   `json:"title"`
	DMStyle      string   `json:"dmStyle"`
	Pace         string   `json:"pace"`
	HeroID       string   `json:"heroId"`
	CompanionIDs []string `json:"companionIds"`
	// Tutorial starts a new player's guided first adventure.
	Tutorial bool `json:"tutorial"`
}

// Output identifies the new game.
type Output struct {
	GameID string `json:"gameId"`
}

// Options are the choices offered in the setup and character screens.
type Options struct {
	Classes    []domain.ClassTemplate `json:"classes"`
	Races      []domain.RaceTemplate  `json:"races"`
	Archetypes []domain.Persona       `json:"archetypes"`
	DMStyles   []domain.DMStyle       `json:"dmStyles"`
	Paces      []domain.Pace          `json:"paces"`
}

// MaxCompanions is the largest AI party allowed alongside the hero.
const MaxCompanions = domain.MaxParty - 1

// UseCase starts adventures.
type UseCase struct {
	catalog Catalog
	store   Store
	art     Illustrator
	agents  Agents
	clock   Clock
	ids     IDGen
}

// New wires the use case.
func New(catalog Catalog, store Store, art Illustrator, agents Agents, clock Clock, ids IDGen) *UseCase {
	return &UseCase{catalog: catalog, store: store, art: art, agents: agents, clock: clock, ids: ids}
}

// Options returns everything the setup screens need.
func (u *UseCase) Options() Options {
	return Options{
		Classes: u.catalog.Classes(), Races: u.catalog.Races(),
		Archetypes: u.catalog.Archetypes(), DMStyles: u.catalog.DMStyles(), Paces: domain.Paces,
	}
}

// Execute sends the chosen characters on a new adventure.
func (u *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.AccountID == "" {
		return Output{}, fmt.Errorf("%w: sign in to start a game", domain.ErrUnauthorized)
	}
	if !u.agents.Ready(ctx, in.AccountID) {
		return Output{}, fmt.Errorf("%w: add your Anthropic key in Settings first; your AI runs the DM and your companions", domain.ErrNoKey)
	}
	ids, err := partyIDs(in)
	if err != nil {
		return Output{}, err
	}
	party, err := u.load(ctx, in.AccountID, ids)
	if err != nil {
		return Output{}, err
	}
	g := u.game(in, party)
	for i := range party {
		if err := party[i].Embark(g.ID, g.CreatedAt); err != nil {
			return Output{}, err
		}
	}
	if err := u.store.CreateAdventure(ctx, g, party); err != nil {
		return Output{}, fmt.Errorf("save new game: %w", err)
	}
	for i := range party {
		u.art.Request(domain.CharacterPortrait(&party[i]))
	}
	return Output{GameID: g.ID}, nil
}

func partyIDs(in Input) ([]string, error) {
	if len(in.CompanionIDs) > MaxCompanions {
		return nil, fmt.Errorf("%w: at most %d companions", domain.ErrInvalid, MaxCompanions)
	}
	if in.HeroID == "" {
		return nil, fmt.Errorf("%w: choose a hero", domain.ErrInvalid)
	}
	ids := append([]string{in.HeroID}, in.CompanionIDs...)
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, fmt.Errorf("%w: a character can only fill one seat", domain.ErrInvalid)
		}
		seen[id] = true
	}
	return ids, nil
}

func (u *UseCase) load(ctx context.Context, accountID string, ids []string) ([]domain.Character, error) {
	party := make([]domain.Character, 0, len(ids))
	for _, id := range ids {
		ch, err := u.store.Character(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load character: %w", err)
		}
		if ch.OwnerID != accountID {
			return nil, fmt.Errorf("%w: character %s", domain.ErrNotFound, id)
		}
		party = append(party, ch)
	}
	return party, nil
}

func (u *UseCase) game(in Input, party []domain.Character) *domain.Game {
	now := u.clock.Now()
	creatures := make([]*domain.Creature, 0, len(party))
	ids := make([]string, 0, len(party))
	for i, ch := range party {
		c := *ch.Sheet
		c.ID, c.OwnerID, c.Controller = ch.ID, ch.OwnerID, domain.ControlAI
		if i == 0 {
			c.Controller = domain.ControlHuman
		}
		creatures = append(creatures, &c)
		ids = append(ids, ch.ID)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "The Adventures of " + creatures[0].Name
	}
	return &domain.Game{
		ID: u.ids.New(), LeaderID: in.AccountID, Title: title, DMStyle: u.catalog.DMStyle(in.DMStyle).Key,
		CreatedAt: now, UpdatedAt: now, Mode: domain.ModeExploration, Roster: true, Pace: domain.PaceFor(in.Pace),
		Scene:     domain.Scene{Key: "tavern", Title: "A Quiet Tavern", Mood: "warm"},
		Creatures: creatures, Party: ids, Transcripts: map[string][]domain.AgentMessage{},
		Tutorial: in.Tutorial,
	}
}
