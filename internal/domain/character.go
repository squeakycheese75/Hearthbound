package domain

import (
	"fmt"
	"time"
)

// CharacterStatus says where a character is in the realm.
type CharacterStatus string

// Character statuses.
const (
	CharIdle        CharacterStatus = "idle"
	CharAdventuring CharacterStatus = "adventuring"
	CharFallen      CharacterStatus = "fallen"
)

// HirePolicy says whether other players may recruit a character from the pub.
type HirePolicy string

// Hire policies.
const (
	HireNo   HirePolicy = "no"
	HireAsk  HirePolicy = "ask"
	HireAuto HirePolicy = "auto"
)

// ParseHirePolicy validates a hire policy.
func ParseHirePolicy(s string) (HirePolicy, error) {
	switch HirePolicy(s) {
	case HireNo, HireAsk, HireAuto:
		return HirePolicy(s), nil
	}
	return "", fmt.Errorf("%w: hire policy %q", ErrUnknown, s)
}

// Character is a persistent hero owned by an account. Sheet holds the game
// stats; it is copied into an adventure and written back as the adventure is saved.
type Character struct {
	ID          string          `json:"id"`
	OwnerID     string          `json:"ownerId"`
	Sheet       *Creature       `json:"sheet"`
	Status      CharacterStatus `json:"status"`
	AdventureID string          `json:"adventureId,omitempty"`
	Hire        HirePolicy      `json:"hire"`
	History     []string        `json:"history,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// Available reports whether the character can join a new adventure.
func (c *Character) Available() bool {
	return c.Status == CharIdle && c.Sheet != nil && !c.Sheet.Dead
}

// Embark marks the character as away on an adventure.
func (c *Character) Embark(adventureID string, at time.Time) error {
	if !c.Available() {
		return fmt.Errorf("%w: %s is %s", ErrInvalid, c.Sheet.Name, c.Status)
	}
	c.Sheet.RestInTown() // heroes leave town rested
	c.Status, c.AdventureID, c.UpdatedAt = CharAdventuring, adventureID, at
	return nil
}

// Settle copies the adventure's latest sheet back into the character and, when
// the adventure is over (or abandoned), returns the character to the realm.
func (c *Character) Settle(sheet *Creature, over bool, title string, at time.Time) {
	if sheet != nil {
		copied := *sheet
		c.Sheet = &copied
	}
	c.UpdatedAt = at
	if c.Sheet != nil && c.Sheet.Dead {
		c.Status = CharFallen
	}
	if !over {
		return
	}
	if c.Status != CharFallen {
		c.Status = CharIdle
		c.Sheet.RestInTown()
	}
	c.AdventureID = ""
	if title != "" {
		c.History = append(c.History, title)
	}
}

// CharacterArtKey is where a character's portrait lives; it follows them across adventures.
func CharacterArtKey(characterID string) string {
	return "characters/" + artSlug(characterID)
}
