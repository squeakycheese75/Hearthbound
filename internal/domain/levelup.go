package domain

import "fmt"

// SheetSnapshot is the part of a sheet a level-up can change, taken before it.
type SheetSnapshot struct {
	MaxHP    int
	Prof     int
	Sneak    int
	Features map[FeatureKey]Feature
	Slots    map[int]int
	Actions  map[string]bool
}

// Snapshot records a sheet for LevelGains.
func Snapshot(c *Creature) SheetSnapshot {
	s := SheetSnapshot{
		MaxHP: c.MaxHP, Prof: c.ProfBonus(), Sneak: c.SneakDice(),
		Features: map[FeatureKey]Feature{}, Slots: map[int]int{}, Actions: map[string]bool{},
	}
	for _, f := range c.Features {
		s.Features[f.Key] = f
	}
	for lvl, n := range c.SpellSlots {
		s.Slots[lvl] = n
	}
	for _, a := range c.Actions {
		s.Actions[a.ID] = true
	}
	return s
}

// LevelGains lists, in plain words, what changed since the snapshot.
func LevelGains(before SheetSnapshot, c *Creature) []string {
	var out []string
	if d := c.MaxHP - before.MaxHP; d > 0 {
		out = append(out, fmt.Sprintf("+%d maximum hit points (now %d)", d, c.MaxHP))
	}
	if p := c.ProfBonus(); p > before.Prof {
		out = append(out, fmt.Sprintf("Proficiency bonus +%d", p))
	}
	out = append(out, featureGains(before, c)...)
	out = append(out, slotGains(before, c)...)
	for _, a := range c.Actions {
		if !before.Actions[a.ID] && a.SlotLevel > 0 {
			out = append(out, fmt.Sprintf("New spell: %s (level %d). %s", a.Name, a.SlotLevel, a.Description))
		}
	}
	return out
}

func featureGains(before SheetSnapshot, c *Creature) []string {
	var out []string
	for _, f := range c.Features {
		old, had := before.Features[f.Key]
		switch {
		case !had:
			out = append(out, fmt.Sprintf("New: %s. %s", f.Name, f.Description))
		case f.Max > old.Max:
			out = append(out, fmt.Sprintf("%s: %d uses (was %d)", f.Name, f.Max, old.Max))
		}
	}
	if n := c.SneakDice(); n > before.Sneak && before.Sneak > 0 {
		out = append(out, fmt.Sprintf("Sneak Attack: %dd6 (was %dd6)", n, before.Sneak))
	}
	return out
}

func slotGains(before SheetSnapshot, c *Creature) []string {
	var out []string
	for lvl := 1; lvl <= 9; lvl++ {
		now, was := c.SpellSlots[lvl], before.Slots[lvl]
		switch {
		case now > 0 && was == 0:
			out = append(out, fmt.Sprintf("Level %d spell slots unlocked (%d)", lvl, now))
		case now > was:
			out = append(out, fmt.Sprintf("Level %d spell slots: %d (was %d)", lvl, now, was))
		}
	}
	return out
}
