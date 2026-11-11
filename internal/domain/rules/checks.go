package rules

import (
	"dnd/internal/domain"
	"fmt"
)

// CheckModifier returns the bonus a creature adds to an ability or skill check.
// skill may be empty for a raw ability check.
func CheckModifier(c *domain.Creature, ability domain.Ability, skill domain.Skill) int {
	mod := c.Abilities.Mod(ability)
	switch {
	case skill != "" && c.Expert(skill):
		mod += 2 * c.ProfBonus()
	case skill != "" && c.SkillProficient(skill):
		mod += c.ProfBonus()
	case c.HasFeature(domain.FeatJackOfAllTrades):
		mod += c.ProfBonus() / 2
	}
	return mod
}

// SaveModifier returns the bonus a creature adds to a saving throw.
func SaveModifier(c *domain.Creature, ability domain.Ability) int {
	mod := c.Abilities.Mod(ability)
	if c.SaveProficient(ability) {
		mod += c.ProfBonus()
	}
	return mod
}

// AbilityCheck rolls a skill check (or plain ability check when skill is empty) against dc.
func AbilityCheck(r Roller, c *domain.Creature, ability domain.Ability, skill domain.Skill, dc int, a Advantage) domain.Roll {
	if skill != "" {
		ability = skill.Ability()
	}
	label := fmt.Sprintf("%s %s check", c.Name, ability)
	if skill != "" {
		label = fmt.Sprintf("%s %s (%s) check", c.Name, skill, ability)
	}
	if c.Has(domain.Exhausted) {
		a = Combine(a == Adv, true)
	}
	roll := test(r, label, CheckModifier(c, ability, skill), dc, a)
	useInspiration(r, c, &roll)
	return roll
}

// SavingThrow rolls a saving throw against dc. Unconscious creatures auto-fail STR and DEX saves.
func SavingThrow(r Roller, c *domain.Creature, ability domain.Ability, dc int, a Advantage) domain.Roll {
	label := fmt.Sprintf("%s %s save", c.Name, ability)
	if c.Has(domain.Unconscious) && (ability == domain.STR || ability == domain.DEX) {
		return domain.Roll{Label: label + " (auto-fail)", Target: dc, Success: boolPtr(false)}
	}
	if ability == domain.DEX && c.HasFeature(domain.FeatDangerSense) && !c.Has(domain.Blinded) {
		a = Combine(true, a == Disadv)
	}
	roll := test(r, label, SaveModifier(c, ability), dc, a)
	useInspiration(r, c, &roll)
	return roll
}

// Initiative rolls a DEX check for turn order.
func Initiative(r Roller, c *domain.Creature) domain.Roll {
	return test(r, c.Name+" initiative", CheckModifier(c, domain.DEX, ""), 0, Normal)
}

func test(r Roller, label string, mod, dc int, a Advantage) domain.Roll {
	kept, rolls := D20(r, a)
	roll := domain.Roll{
		Label:     label,
		Dice:      d20Label(mod),
		Rolls:     rolls,
		Kept:      kept,
		Modifier:  mod,
		Total:     kept + mod,
		Target:    dc,
		Advantage: a.String(),
	}
	if dc > 0 {
		roll.Success = boolPtr(roll.Total >= dc)
	}
	return roll
}
