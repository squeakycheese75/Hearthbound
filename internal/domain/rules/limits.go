package rules

import (
	"dnd/internal/domain"
	"fmt"
)

// Limits on what the DM can hand out or inflict through its tools in one
// turn. They hold however the DM is prompted, so a player who talks the DM
// into "give me 10,000 gold" still can't break the game.

// TurnXPCap is the most XP award_xp may grant in one DM turn at a party level
// (milestones are 50-150 XP each at low level). Combat XP is separate.
func TurnXPCap(level int) int { return 100 * max(level, 1) }

// TurnGoldCap is the most gold give_item may hand out in one DM turn.
func TurnGoldCap(level int) int { return 50 * max(level, 1) }

// MaxItemName bounds an item's name; MaxInventory bounds what one character carries.
const (
	MaxItemName  = 60
	MaxInventory = 40
)

// CheckHazardDice bounds the dice for damage and healing outside combat
// (traps, falls, potions) at a party level: up to 2 + 2*level dice of at
// most d12, plus a bonus of at most 5*level.
func CheckHazardDice(d domain.Dice, level int) error {
	level = max(level, 1)
	if d.Count > 2+2*level || d.Sides > 12 || d.Bonus > 5*level {
		return fmt.Errorf("%w: %s is too much for level %d (at most %dd12+%d)", domain.ErrInvalid, d.String(), level, 2+2*level, 5*level)
	}
	return nil
}
