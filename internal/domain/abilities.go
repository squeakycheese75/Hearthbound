// Package domain holds the entities, value objects and errors of the game.
// It depends on the standard library only.
package domain

import (
	"fmt"
	"strings"
)

// Ability is one of the six D&D ability scores.
type Ability string

// The six abilities.
const (
	STR Ability = "STR"
	DEX Ability = "DEX"
	CON Ability = "CON"
	INT Ability = "INT"
	WIS Ability = "WIS"
	CHA Ability = "CHA"
)

// Abilities lists every ability in sheet order.
var Abilities = []Ability{STR, DEX, CON, INT, WIS, CHA}

// ParseAbility converts a name like "dex" or "Dexterity" into an Ability.
func ParseAbility(s string) (Ability, error) {
	key := strings.ToUpper(strings.TrimSpace(s))
	if len(key) >= 3 {
		key = key[:3]
	}
	for _, a := range Abilities {
		if string(a) == key {
			return a, nil
		}
	}
	return "", fmt.Errorf("%w: ability %q", ErrUnknown, s)
}

// AbilityScores holds a creature's six scores.
type AbilityScores map[Ability]int

// Score returns the score for a, defaulting to 10 when unset.
func (s AbilityScores) Score(a Ability) int {
	v, ok := s[a]
	if !ok {
		return 10
	}
	return v
}

// Mod returns the ability modifier for a.
func (s AbilityScores) Mod(a Ability) int {
	return Modifier(s.Score(a))
}

// Modifier converts an ability score into its modifier (floor((score-10)/2)).
func Modifier(score int) int {
	d := score - 10
	if d < 0 {
		return (d - 1) / 2
	}
	return d / 2
}

// ProficiencyBonus returns the proficiency bonus for a character level or challenge-derived level.
func ProficiencyBonus(level int) int {
	if level < 1 {
		return 2
	}
	return 2 + (level-1)/4
}
