package domain

import (
	"fmt"
	"strings"
)

// Skill is a 5e skill, governed by one ability.
type Skill string

// The 5e SRD skills.
const (
	Acrobatics     Skill = "acrobatics"
	AnimalHandling Skill = "animal_handling"
	Arcana         Skill = "arcana"
	Athletics      Skill = "athletics"
	Deception      Skill = "deception"
	History        Skill = "history"
	Insight        Skill = "insight"
	Intimidation   Skill = "intimidation"
	Investigation  Skill = "investigation"
	Medicine       Skill = "medicine"
	Nature         Skill = "nature"
	Perception     Skill = "perception"
	Performance    Skill = "performance"
	Persuasion     Skill = "persuasion"
	Religion       Skill = "religion"
	SleightOfHand  Skill = "sleight_of_hand"
	Stealth        Skill = "stealth"
	Survival       Skill = "survival"
)

var skillAbility = map[Skill]Ability{
	Acrobatics: DEX, AnimalHandling: WIS, Arcana: INT, Athletics: STR,
	Deception: CHA, History: INT, Insight: WIS, Intimidation: CHA,
	Investigation: INT, Medicine: WIS, Nature: INT, Perception: WIS,
	Performance: CHA, Persuasion: CHA, Religion: INT, SleightOfHand: DEX,
	Stealth: DEX, Survival: WIS,
}

// Skills returns every skill name, for tool schemas.
func Skills() []Skill {
	return []Skill{
		Acrobatics, AnimalHandling, Arcana, Athletics, Deception, History,
		Insight, Intimidation, Investigation, Medicine, Nature, Perception,
		Performance, Persuasion, Religion, SleightOfHand, Stealth, Survival,
	}
}

// Ability returns the ability that governs the skill.
func (s Skill) Ability() Ability {
	return skillAbility[s]
}

// ParseSkill converts "Sleight of Hand" or "stealth" into a Skill.
func ParseSkill(s string) (Skill, error) {
	key := Skill(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_"))
	if _, ok := skillAbility[key]; ok {
		return key, nil
	}
	return "", fmt.Errorf("%w: skill %q", ErrUnknown, s)
}
