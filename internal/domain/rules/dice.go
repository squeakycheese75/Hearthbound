// Package rules implements the deterministic D&D 5e (SRD 5.1) mechanics.
// All randomness comes from an injected Roller so results are testable.
package rules

import (
	"dnd/internal/domain"
	"fmt"
)

// Roller rolls a single die with the given number of sides, returning 1..sides.
type Roller interface {
	Roll(sides int) int
}

// Advantage modifies a d20 roll.
type Advantage int

// Advantage states.
const (
	Normal Advantage = iota
	Adv
	Disadv
)

// String names the state for display.
func (a Advantage) String() string {
	switch a {
	case Adv:
		return "advantage"
	case Disadv:
		return "disadvantage"
	}
	return ""
}

// Combine resolves advantage and disadvantage sources: any of both cancels out.
func Combine(adv, dis bool) Advantage {
	switch {
	case adv && dis:
		return Normal
	case adv:
		return Adv
	case dis:
		return Disadv
	}
	return Normal
}

// ParseAdvantage reads "advantage", "disadvantage" or anything else as normal.
func ParseAdvantage(s string) Advantage {
	switch s {
	case "advantage":
		return Adv
	case "disadvantage":
		return Disadv
	}
	return Normal
}

// RollDice rolls every die in d and returns the total and the individual dice.
func RollDice(r Roller, d domain.Dice) (int, []int) {
	rolls := make([]int, 0, d.Count)
	total := d.Bonus
	for range d.Count {
		v := r.Roll(d.Sides)
		rolls = append(rolls, v)
		total += v
	}
	return total, rolls
}

// D20 rolls a d20 with advantage state and returns the kept die and every die rolled.
func D20(r Roller, a Advantage) (int, []int) {
	first := r.Roll(20)
	if a == Normal {
		return first, []int{first}
	}
	second := r.Roll(20)
	rolls := []int{first, second}
	if a == Adv {
		return max(first, second), rolls
	}
	return min(first, second), rolls
}

func d20Label(mod int) string {
	switch {
	case mod > 0:
		return fmt.Sprintf("1d20+%d", mod)
	case mod < 0:
		return fmt.Sprintf("1d20%d", mod)
	}
	return "1d20"
}

func boolPtr(b bool) *bool { return &b }
