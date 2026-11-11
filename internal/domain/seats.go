package domain

import (
	"fmt"
	"slices"
	"time"
)

// Pace sets how long an adventure waits for live players before their agents act.
// Zero durations mean "wait for ever".
type Pace struct {
	Key     string        `json:"key"`
	Name    string        `json:"name"`
	Explore time.Duration `json:"explore"`
	Combat  time.Duration `json:"combat"`
}

// Paces are the presets offered when starting an adventure.
var Paces = []Pace{
	{Key: "daily", Name: "Daily", Explore: 12 * time.Hour, Combat: 2 * time.Hour},
	{Key: "relaxed", Name: "Relaxed", Explore: 24 * time.Hour, Combat: 4 * time.Hour},
	{Key: "brisk", Name: "Brisk", Explore: time.Hour, Combat: 10 * time.Minute},
	{Key: "fast", Name: "Fast (testing)", Explore: 2 * time.Minute, Combat: time.Minute},
	{Key: "off", Name: "Always wait for me"},
}

// PaceFor returns the preset with the given key, defaulting to Daily.
func PaceFor(key string) Pace {
	for _, p := range Paces {
		if p.Key == key {
			return p
		}
	}
	return Paces[0]
}

// Live reports whether a party member is played by its owner rather than its agent.
func Live(c *Creature) bool {
	return c.Kind == KindPC && c.Controller == ControlHuman
}

// Playing reports whether the story can move: the leader, whose key runs the
// DM, must be playing one of their own living characters (an agent covering
// while they're away still counts; the seat is theirs). Otherwise the story
// pauses, so nobody plays on with someone else's DM.
func (g *Game) Playing() bool {
	for _, c := range g.PartyMembers() {
		if Live(c) && !c.Dead && g.LeaderPlays(c) {
			return true
		}
	}
	return false
}

// PlayerOf is the account that plays a character: its player if a friend
// holds the seat, else its owner.
func PlayerOf(c *Creature) string {
	if c.PlayerID != "" {
		return c.PlayerID
	}
	return c.OwnerID
}

// LeaderPlays reports whether the party's leader plays a character
// (characters from before accounts count as the leader's).
func (g *Game) LeaderPlays(c *Creature) bool {
	p := PlayerOf(c)
	return p == g.LeaderID || p == ""
}

// SeatOpen reports whether the leader has offered a character's seat to friends.
func (g *Game) SeatOpen(id string) bool {
	return slices.Contains(g.OpenSeats, id)
}

// LiveSeats returns party members whose owners are playing them and who can act.
func (g *Game) LiveSeats() []*Creature {
	var out []*Creature
	for _, c := range g.PartyMembers() {
		if Live(c) && c.CanAct() {
			out = append(out, c)
		}
	}
	return out
}

// Owes reports whether a live seat still has to act this exploration round.
func (g *Game) Owes(c *Creature) bool {
	_, done := g.Pending[c.ID]
	return !done
}

// WaitingSeats returns the live characters the game is waiting on right now.
func (g *Game) WaitingSeats() []*Creature {
	var out []*Creature
	switch g.Mode {
	case ModeExploration:
		for _, c := range g.LiveSeats() {
			if g.Owes(c) {
				out = append(out, c)
			}
		}
	case ModeCombat:
		if c, err := g.Creature(g.Encounter.Current()); err == nil && Live(c) && c.CanAct() {
			out = append(out, c)
		}
	}
	return out
}

// WaitingFor returns the accounts the game is waiting on right now.
func (g *Game) WaitingFor() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range g.WaitingSeats() {
		if p := PlayerOf(c); p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// SeatFor finds the party member an account plays. With characterID set it
// must be that one.
func (g *Game) SeatFor(accountID, characterID string) (*Creature, error) {
	for _, c := range g.PartyMembers() {
		if PlayerOf(c) != accountID || (characterID != "" && c.ID != characterID) {
			continue
		}
		return c, nil
	}
	return nil, fmt.Errorf("%w: you have no character in this party", ErrForbidden)
}

// MaxParty is the most characters an adventuring party can hold.
const MaxParty = 4

// TavernScenes are the places where a party can take on or send home members:
// somewhere with a tavern to recruit from.
var TavernScenes = []string{"tavern", "town", "village"}

// AtTavern reports whether the party can change now: before setting out, or
// in a tavern, town or village outside combat.
func (g *Game) AtTavern() bool {
	if !g.Started {
		return g.Mode != ModeOver
	}
	return g.Mode == ModeExploration && slices.Contains(TavernScenes, g.Scene.Key)
}

// Join adds a roster character's sheet to the party as an agent-played
// companion. The store embarks the character when the game is saved.
func (g *Game) Join(ch Character) {
	c := *ch.Sheet
	c.ID, c.OwnerID, c.Controller = ch.ID, ch.OwnerID, ControlAI
	g.Creatures = append(g.Creatures, &c)
	g.Party = append(g.Party, c.ID)
	g.Joined = append(g.Joined, c.ID)
}

// Leave takes a member out of the party. The store settles the character back
// into its owner's roster when the game is saved.
func (g *Game) Leave(c *Creature) {
	g.Party = slices.DeleteFunc(g.Party, func(id string) bool { return id == c.ID })
	g.Creatures = slices.DeleteFunc(g.Creatures, func(x *Creature) bool { return x.ID == c.ID })
	delete(g.Transcripts, c.ID)
	delete(g.Context, c.ID)
	delete(g.Pending, c.ID)
	delete(g.Covering, c.ID)
	delete(g.Hold, c.ID)
	g.OpenSeats = slices.DeleteFunc(g.OpenSeats, func(id string) bool { return id == c.ID })
	c.PlayerID = ""
	g.Left = append(g.Left, c)
}
