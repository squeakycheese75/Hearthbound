package domain

import (
	"fmt"
	"slices"
	"strings"
)

// MaxLevel is the highest level heroes can reach.
const MaxLevel = 5

// ASILevels are the levels that bring an Ability Score Improvement.
var ASILevels = []int{4}

// MaxAbility is the highest an ability score can be raised to.
const MaxAbility = 20

// HPPerLevel is the hit points a level adds: the class's average hit die roll
// plus the CON modifier (at least 1).
func HPPerLevel(cls ClassTemplate, conMod int) int {
	return max(cls.HitDie/2+1+conMod, 1)
}

// Improve spends an Ability Score Improvement: +2 to one ability, or +1 to
// two, none above 20. A higher CON modifier raises hit points for every level.
func (c *Creature) Improve(inc map[Ability]int) error {
	if c.ASI <= 0 {
		return fmt.Errorf("%w: %s has no ability score improvement to spend", ErrInvalid, c.Name)
	}
	if err := c.checkImprovement(inc); err != nil {
		return err
	}
	if c.BaseAbilities == nil {
		c.BaseAbilities = AbilityScores{}
		for a, v := range c.Abilities {
			c.BaseAbilities[a] = v
		}
	}
	oldCon := c.BaseAbilities.Mod(CON)
	for a, n := range inc {
		c.BaseAbilities[a] = c.BaseAbilities.Score(a) + n
	}
	c.BaseMaxHP += (c.BaseAbilities.Mod(CON) - oldCon) * c.Level
	c.ASI--
	return nil
}

func (c *Creature) checkImprovement(inc map[Ability]int) error {
	total := 0
	for a, n := range inc {
		if !slices.Contains(Abilities, a) || n < 1 || n > 2 {
			return fmt.Errorf("%w: raise an ability by 1 or 2", ErrInvalid)
		}
		if c.baseScore(a)+n > MaxAbility {
			return fmt.Errorf("%w: %s can't go above %d", ErrInvalid, a, MaxAbility)
		}
		total += n
	}
	if total != 2 {
		return fmt.Errorf("%w: an improvement is +2 to one ability or +1 to two", ErrInvalid)
	}
	return nil
}

func (c *Creature) baseScore(a Ability) int {
	if c.BaseAbilities != nil {
		return c.BaseAbilities.Score(a)
	}
	return c.Abilities.Score(a)
}

// ImprovementText describes an improvement, e.g. "STR +2" or "DEX +1, CON +1".
func ImprovementText(inc map[Ability]int) string {
	var parts []string
	for _, a := range Abilities {
		if n := inc[a]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s +%d", a, n))
		}
	}
	return strings.Join(parts, ", ")
}

// ChallengeValue reads a challenge rating ("1/4", "2") as a number.
func ChallengeValue(cr string) float64 {
	var n, d int
	if _, err := fmt.Sscanf(cr, "%d/%d", &n, &d); err == nil && d > 0 {
		return float64(n) / float64(d)
	}
	if _, err := fmt.Sscanf(cr, "%d", &n); err == nil {
		return float64(n)
	}
	return 99
}
