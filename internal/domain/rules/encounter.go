package rules

import (
	"dnd/internal/domain"
	"fmt"
)

// hardXP is the 5e "hard" encounter threshold for one character, by level.
var hardXP = map[int]int{1: 75, 2: 150, 3: 225, 4: 375, 5: 750}

// HardBudget is the most adjusted XP a fair but hard fight may cost the
// living members of the party (5e encounter-building thresholds).
func HardBudget(party []*domain.Creature) int {
	total := 0
	for _, c := range party {
		if c.Dead {
			continue
		}
		xp, ok := hardXP[c.Level]
		if !ok {
			xp = 75 * max(c.Level, 1)
		}
		total += xp
	}
	return total
}

// halfMultipliers are the 5e group multipliers (x1, x1.5, x2, x2.5, x3, x4) in halves.
var halfMultipliers = []int{2, 3, 4, 5, 6, 8}

// AdjustedXP is what a group of monsters costs against a party of partySize:
// their total XP times the 5e multiplier for how many there are, one step
// harsher for parties under three and one step kinder for six or more.
func AdjustedXP(monsters []*domain.Creature, partySize int) int {
	total := 0
	for _, m := range monsters {
		total += m.XPValue
	}
	step := multiplierStep(len(monsters))
	switch {
	case partySize < 3:
		step++
	case partySize >= 6:
		step--
	}
	step = min(max(step, 0), len(halfMultipliers)-1)
	return total * halfMultipliers[step] / 2
}

func multiplierStep(count int) int {
	switch {
	case count <= 1:
		return 0
	case count == 2:
		return 1
	case count <= 6:
		return 2
	case count <= 10:
		return 3
	case count <= 14:
		return 4
	}
	return 5
}

// crValues maps challenge ratings to numbers. Unknown ratings count as 0.
var crValues = map[string]float64{"0": 0, "1/8": 0.125, "1/4": 0.25, "1/2": 0.5}

// CRValue turns a challenge rating such as "1/4" or "3" into a number.
func CRValue(cr string) float64 {
	if v, ok := crValues[cr]; ok {
		return v
	}
	var n float64
	if _, err := fmt.Sscanf(cr, "%g", &n); err != nil {
		return 0
	}
	return n
}

// MaxCR is the toughest single monster a party should meet, so danger ramps
// up with level: CR 1/2 at level 1, then one less than the party's level
// (CR 1 at level 2, CR 2 at level 3...).
func MaxCR(party []*domain.Creature) float64 {
	level := 0
	for _, c := range party {
		if !c.Dead {
			level = max(level, c.Level)
		}
	}
	return max(0.5, float64(level-1))
}
