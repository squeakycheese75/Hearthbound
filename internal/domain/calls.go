package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// CallKind says which way a call goes.
type CallKind string

// Call kinds.
const (
	// CallAsk is a party's leader asking for someone else's hero.
	CallAsk CallKind = "ask"
	// CallOffer is a hero's owner offering them to a party.
	CallOffer CallKind = "offer"
)

// Call is an open question between a party and a hero's owner, waiting for
// an answer: the owner answers an ask, the leader answers an offer.
type Call struct {
	Kind        CallKind `json:"kind"`
	CharacterID string   `json:"characterId"`
	OwnerID     string   `json:"ownerId"`
	// Name and Summary describe the hero, so the board needn't load them.
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// MaxWanted is the longest a party-wanted post may be.
const MaxWanted = 200

// ParseWanted cleans a party-wanted post. Empty takes the post down.
func ParseWanted(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > MaxWanted {
		return "", fmt.Errorf("%w: keep the post under %d characters", ErrInvalid, MaxWanted)
	}
	return s, nil
}

// Answerer is the account that answers a call in a game led by leaderID.
func (c Call) Answerer(leaderID string) string {
	if c.Kind == CallAsk {
		return c.OwnerID
	}
	return leaderID
}

// Asker is the account that made a call in a game led by leaderID.
func (c Call) Asker(leaderID string) string {
	if c.Kind == CallAsk {
		return leaderID
	}
	return c.OwnerID
}

// CallFor finds the open call about a hero.
func (g *Game) CallFor(characterID string) (Call, bool) {
	i := slices.IndexFunc(g.Calls, func(c Call) bool { return c.CharacterID == characterID })
	if i < 0 {
		return Call{}, false
	}
	return g.Calls[i], true
}

// DropCall removes the call about a hero.
func (g *Game) DropCall(characterID string) {
	g.Calls = slices.DeleteFunc(g.Calls, func(c Call) bool { return c.CharacterID == characterID })
}

// OnTheBoard reports whether the game has business on the tavern's notice
// board: open seats, a party-wanted post, or calls waiting for an answer.
func (g *Game) OnTheBoard() bool {
	return g.Mode != ModeOver && (len(g.OpenSeats) > 0 || g.Wanted != "" || len(g.Calls) > 0)
}

// CallAbout describes a roster hero for a call.
func CallAbout(kind CallKind, ch Character) Call {
	c := ch.Sheet
	return Call{
		Kind: kind, CharacterID: ch.ID, OwnerID: ch.OwnerID, Name: c.Name,
		Summary: fmt.Sprintf("level %d %s %s", c.Level, c.Race, c.Class),
	}
}
