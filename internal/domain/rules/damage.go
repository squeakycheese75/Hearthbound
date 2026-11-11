package rules

import (
	"dnd/internal/domain"
	"fmt"
)

// physical damage types, which a raging barbarian resists.
var physical = map[string]bool{"bludgeoning": true, "piercing": true, "slashing": true}

// hurt applies damage of a type, after resistances: a raging barbarian
// takes half from bludgeoning, piercing and slashing.
func hurt(c *domain.Creature, amount int, dmgType string, crit bool) []string {
	if c.Has(domain.Raging) && physical[dmgType] && amount > 0 {
		amount /= 2
		return append([]string{c.Name + "'s rage shrugs off half the blow."}, ApplyDamage(c, amount, crit)...)
	}
	return ApplyDamage(c, amount, crit)
}

// DamageOf applies damage of a type (for the DM's tools), with resistances.
func DamageOf(c *domain.Creature, amount int, dmgType string) []string {
	return hurt(c, amount, dmgType, false)
}

// ApplyDamage removes hit points and handles dropping to 0. It returns notes for the log.
// PCs fall unconscious (or die from massive damage); monsters die at 0.
func ApplyDamage(c *domain.Creature, amount int, crit bool) []string {
	if amount <= 0 || c.Dead {
		return nil
	}
	c.RemoveCondition(domain.Turned) // being hurt breaks a turning
	if c.HP <= 0 {
		return damageWhileDown(c, amount, crit)
	}
	overflow := amount - c.HP
	c.HP = max(0, c.HP-amount)
	notes := []string{fmt.Sprintf("%s takes %d damage (%d/%d HP).", c.Name, amount, c.HP, c.MaxHP)}
	if c.HP > 0 {
		return notes
	}
	return append(notes, dropToZero(c, overflow)...)
}

func dropToZero(c *domain.Creature, overflow int) []string {
	if c.Kind == domain.KindMonster {
		c.Dead = true
		return []string{c.Name + " is slain!"}
	}
	if overflow >= c.MaxHP {
		c.Dead = true
		return []string{c.Name + " is killed outright by massive damage!"}
	}
	c.RemoveCondition(domain.Raging) // a rage ends when you fall
	c.AddCondition(domain.Unconscious)
	c.AddCondition(domain.Prone)
	c.DeathSaves = domain.DeathSaves{}
	c.Stable = false
	return []string{c.Name + " falls unconscious and is dying!"}
}

func damageWhileDown(c *domain.Creature, amount int, crit bool) []string {
	if c.Kind == domain.KindMonster || amount >= c.MaxHP {
		c.Dead = true
		return []string{c.Name + " dies."}
	}
	c.Stable = false
	fails := 1
	if crit {
		fails = 2
	}
	return addDeathFailures(c, fails)
}

func addDeathFailures(c *domain.Creature, n int) []string {
	c.DeathSaves.Failures += n
	if c.DeathSaves.Failures >= 3 {
		c.Dead = true
		return []string{c.Name + " has died."}
	}
	return []string{fmt.Sprintf("%s suffers a death save failure (%d/3).", c.Name, c.DeathSaves.Failures)}
}

// Heal restores hit points, waking a dying creature.
func Heal(c *domain.Creature, amount int) []string {
	if c.Dead || amount <= 0 {
		return nil
	}
	c.HP = min(c.MaxHP, c.HP+amount)
	c.RemoveCondition(domain.Unconscious)
	c.DeathSaves = domain.DeathSaves{}
	c.Stable = false
	return []string{fmt.Sprintf("%s regains %d HP (%d/%d).", c.Name, amount, c.HP, c.MaxHP)}
}

// DeathSave rolls a death saving throw for a dying PC.
func DeathSave(r Roller, c *domain.Creature) (domain.Roll, []string) {
	kept, rolls := D20(r, Normal)
	roll := domain.Roll{Label: c.Name + " death save", Dice: "1d20", Rolls: rolls, Kept: kept, Total: kept, Target: 10}
	roll.Success = boolPtr(kept >= 10)
	switch {
	case kept == 20:
		roll.Critical = true
		return roll, Heal(c, 1)
	case kept == 1:
		return roll, addDeathFailures(c, 2)
	case kept < 10:
		return roll, addDeathFailures(c, 1)
	}
	c.DeathSaves.Successes++
	if c.DeathSaves.Successes >= 3 {
		c.Stable = true
		return roll, []string{c.Name + " is stable."}
	}
	return roll, []string{fmt.Sprintf("%s succeeds a death save (%d/3).", c.Name, c.DeathSaves.Successes)}
}

// Stabilize makes a dying creature stable (e.g. after a Medicine check).
func Stabilize(c *domain.Creature) {
	if c.HP > 0 || c.Dead {
		return
	}
	c.Stable = true
}

// ComeRound wakes a downed, living creature with 1 HP once the danger has
// passed (for example after a won fight), so it can act again.
func ComeRound(c *domain.Creature) []string {
	if c.Dead || c.HP > 0 {
		return nil
	}
	Heal(c, 1)
	return []string{c.Name + " comes round, battered but alive (1 HP)."}
}

// Dying reports whether the creature must roll death saves.
func Dying(c *domain.Creature) bool {
	return c.Kind == domain.KindPC && !c.Dead && c.HP <= 0 && !c.Stable
}
