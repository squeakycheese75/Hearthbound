package rules

import (
	"dnd/internal/domain"
	"fmt"
	"slices"
)

// Outcome is the result of resolving one action.
type Outcome struct {
	Rolls  []domain.Roll
	Notes  []string
	Hit    bool
	Amount int
	// TurnOver is set by TakeTurn when the actor's turn has ended.
	TurnOver bool
}

// turnCtx is the battlefield an action happens on, for features that look
// around (Sneak Attack, Cutting Words) or happen once a turn. Nil outside a fight.
type turnCtx struct {
	g *domain.Game
}

func (t *turnCtx) spent(actor *domain.Creature) *domain.TurnSpent {
	if t == nil || t.g == nil {
		return &domain.TurnSpent{}
	}
	return spentFor(t.g, actor)
}

// AttackAdvantage works out advantage for an attack from conditions on both sides.
func AttackAdvantage(attacker, target *domain.Creature, act domain.Action) Advantage {
	adjacent := attacker.Pos.DistanceFt(target.Pos) <= 5
	return Combine(hasAdvantage(attacker, target, act, adjacent), hasDisadvantage(attacker, target, act, adjacent))
}

func hasAdvantage(attacker, target *domain.Creature, act domain.Action, adjacent bool) bool {
	reckless := attacker.Has(domain.Reckless) && act.Weapon && !act.Ranged() && act.Stat == domain.STR
	return targetGrantsAdvantage(target) || (target.Has(domain.Prone) && adjacent) ||
		attacker.Has(domain.Hidden) || reckless || target.Has(domain.Reckless)
}

func hasDisadvantage(attacker, target *domain.Creature, act domain.Action, adjacent bool) bool {
	return attackerHindered(attacker) || target.Has(domain.Dodging) ||
		(target.Has(domain.Prone) && !adjacent) ||
		(act.Ranged() && adjacent && !target.Down())
}

func targetGrantsAdvantage(t *domain.Creature) bool {
	return t.Has(domain.Restrained) || t.Has(domain.Unconscious) || t.Has(domain.Blinded)
}

func attackerHindered(a *domain.Creature) bool {
	for _, c := range []domain.Condition{domain.Blinded, domain.Poisoned, domain.Frightened, domain.Prone, domain.Restrained} {
		if a.Has(c) {
			return true
		}
	}
	return false
}

// InRange reports whether target is within the action's reach.
func InRange(actor, target *domain.Creature, act domain.Action) bool {
	reach := max(act.RangeFt, 5)
	return actor.Pos.DistanceFt(target.Pos) <= reach
}

// Resolve carries out an action from actor on target, applying damage or
// healing. Outside a fight, features that need the battlefield do not apply.
func Resolve(r Roller, actor *domain.Creature, act domain.Action, target *domain.Creature) (Outcome, error) {
	return resolve(r, nil, actor, act, target)
}

func resolve(r Roller, t *turnCtx, actor *domain.Creature, act domain.Action, target *domain.Creature) (Outcome, error) {
	if act.SlotLevel > 0 {
		used, err := actor.UseSlot(act.SlotLevel)
		if err != nil {
			return Outcome{}, err
		}
		act = upcast(act, used)
	}
	return resolveKind(r, t, actor, act, target)
}

// resolveKind resolves an action by how it works: dodge, heal, damage or attack.
func resolveKind(r Roller, t *turnCtx, actor *domain.Creature, act domain.Action, target *domain.Creature) (Outcome, error) {
	switch act.Kind {
	case domain.ActDodge:
		actor.AddCondition(domain.Dodging)
		return Outcome{Notes: []string{actor.Name + " takes the Dodge action."}}, nil
	case domain.ActHeal:
		if act.Mass {
			return resolveMassHeal(r, t, actor, act), nil
		}
		return resolveHeal(r, actor, act, target), nil
	case domain.ActAutoHit:
		return resolveAutoHit(r, act, target), nil
	case domain.ActSave:
		if act.Radius > 0 && t != nil && t.g != nil {
			return resolveArea(r, t.g, actor, act, target), nil
		}
		return resolveSave(r, actor, act, target), nil
	case domain.ActAttack:
		return resolveAttack(r, t, actor, act, target), nil
	}
	return Outcome{}, fmt.Errorf("%w: action kind %q", domain.ErrInvalid, act.Kind)
}

// upcast strengthens a spell cast with a higher slot than its own.
func upcast(act domain.Action, used int) domain.Action {
	extra := used - act.SlotLevel
	if extra <= 0 || act.Upcast.Count == 0 {
		return act
	}
	act.Dice.Count += act.Upcast.Count * extra
	act.Dice.Bonus += act.Upcast.Bonus * extra
	act.Name = fmt.Sprintf("%s (level %d)", act.Name, used)
	return act
}

// resolveArea is a save spell that bursts around the target: everyone in the
// radius saves against one damage roll. An evoker's allies are spared.
func resolveArea(r Roller, g *domain.Game, actor *domain.Creature, act domain.Action, target *domain.Creature) Outcome {
	dmg := rollDamage(r, act, false)
	out := Outcome{Rolls: []domain.Roll{dmg}}
	for _, c := range InArea(g, target.Pos, act.Radius) {
		if !domain.Hostile(actor, c) && actor.HasFeature(domain.FeatSculptSpells) {
			out.Notes = append(out.Notes, fmt.Sprintf("%s's sculpted spell spares %s.", actor.Name, c.Name))
			continue
		}
		save := SavingThrow(r, c, act.SaveAbility, act.SaveDC, Normal)
		out.Rolls = append(out.Rolls, save)
		amount := dmg.Total
		if *save.Success {
			amount /= 2
		}
		out.Hit = out.Hit || amount > 0
		out.Amount += amount
		out.Notes = append(out.Notes, hurt(c, amount, act.DamageType, false)...)
	}
	return out
}

// InArea lists the standing creatures within radius feet of a point.
func InArea(g *domain.Game, p domain.Position, radius int) []*domain.Creature {
	var out []*domain.Creature
	for _, c := range g.Creatures {
		if !c.Down() && c.Pos.DistanceFt(p) <= radius {
			out = append(out, c)
		}
	}
	return out
}

// resolveMassHeal heals every wounded ally in range of the caster.
func resolveMassHeal(r Roller, t *turnCtx, actor *domain.Creature, act domain.Action) Outcome {
	out := Outcome{Hit: true}
	for _, c := range woundedAllies(t, actor, act.RangeFt) {
		one := resolveHeal(r, actor, act, c)
		out.Rolls = append(out.Rolls, one.Rolls...)
		out.Notes = append(out.Notes, one.Notes...)
		out.Amount += one.Amount
	}
	return out
}

// woundedAllies are the actor's living, hurt allies within range (the actor
// alone outside a fight).
func woundedAllies(t *turnCtx, actor *domain.Creature, rangeFt int) []*domain.Creature {
	all := []*domain.Creature{actor}
	if t != nil && t.g != nil {
		all = t.g.Creatures
	}
	var out []*domain.Creature
	for _, c := range all {
		if !c.Dead && !domain.Hostile(actor, c) && c.HP < c.MaxHP && actor.Pos.DistanceFt(c.Pos) <= rangeFt {
			out = append(out, c)
		}
	}
	return out
}

// critOn is the lowest d20 roll that is a critical hit for this attack.
func critOn(actor *domain.Creature, act domain.Action) int {
	if act.Weapon && actor.HasFeature(domain.FeatImprovedCritical) {
		return 19
	}
	return 20
}

func resolveAttack(r Roller, t *turnCtx, actor *domain.Creature, act domain.Action, target *domain.Creature) Outcome {
	a := AttackAdvantage(actor, target, act)
	kept, rolls := D20(r, a)
	attack := domain.Roll{
		Label:     fmt.Sprintf("%s attacks %s with %s", actor.Name, target.Name, act.Name),
		Dice:      d20Label(act.ToHit),
		Rolls:     rolls,
		Kept:      kept,
		Modifier:  act.ToHit,
		Total:     kept + act.ToHit,
		Target:    target.AC,
		Advantage: a.String(),
	}
	notes := cuttingWords(r, t, actor, target, &attack)
	notes = append(notes, useInspiration(r, actor, &attack)...)
	hit := kept != 1 && (kept == 20 || attack.Total >= target.AC)
	crit := hit && (kept >= critOn(actor, act) || (target.Has(domain.Unconscious) && actor.Pos.DistanceFt(target.Pos) <= 5))
	actor.RemoveCondition(domain.Hidden) // attacking gives you away
	attack.Success = boolPtr(hit)
	attack.Critical = crit
	out := Outcome{Rolls: []domain.Roll{attack}, Hit: hit, Notes: notes}
	if !hit {
		out.Notes = append(out.Notes, fmt.Sprintf("%s misses %s.", actor.Name, target.Name))
		return out
	}
	wounded := target.HP < target.MaxHP
	dmg := rollDamage(r, act, crit)
	if raging(actor, act) {
		dmg.Modifier += 2
		dmg.Total += 2
		dmg.Label += " (rage +2)"
	}
	out.Rolls = append(out.Rolls, dmg)
	out.Rolls = append(out.Rolls, riders(r, t, actor, act, target, a, crit, wounded)...)
	for _, roll := range out.Rolls[1:] {
		out.Amount += roll.Total
	}
	out.Notes = append(out.Notes, uncannyDodge(t, target, &out.Amount)...)
	out.Notes = append(out.Notes, hurt(target, out.Amount, act.DamageType, crit)...)
	return out
}

// uncannyDodge is a rogue's reaction: halve the damage of a hit.
func uncannyDodge(t *turnCtx, target *domain.Creature, amount *int) []string {
	if t == nil || t.g == nil || !target.HasFeature(domain.FeatUncannyDodge) || !target.CanAct() || t.g.Encounter.Reacted[target.ID] {
		return nil
	}
	react(t.g, target)
	*amount /= 2
	return []string{target.Name + " twists aside with an uncanny dodge, halving the blow."}
}

// react marks a creature's reaction as used until its next turn.
func react(g *domain.Game, c *domain.Creature) {
	if g.Encounter.Reacted == nil {
		g.Encounter.Reacted = map[string]bool{}
	}
	g.Encounter.Reacted[c.ID] = true
}

// raging reports whether a raging barbarian's attack gets the rage bonus.
func raging(actor *domain.Creature, act domain.Action) bool {
	return actor.Has(domain.Raging) && act.Weapon && !act.Ranged() && act.Stat == domain.STR
}

// riders are once-per-turn extra damage dice: Sneak Attack and Colossus Slayer.
func riders(r Roller, t *turnCtx, actor *domain.Creature, act domain.Action, target *domain.Creature, a Advantage, crit, wounded bool) []domain.Roll {
	sp := t.spent(actor)
	var out []domain.Roll
	if !sp.Sneak && sneakable(t, actor, act, target, a) {
		sp.Sneak = true
		out = append(out, extraDice(r, "Sneak Attack", actor.SneakDice(), 6, crit))
	}
	if !sp.Colossus && wounded && act.Weapon && actor.HasFeature(domain.FeatColossusSlayer) {
		sp.Colossus = true
		out = append(out, extraDice(r, "Colossus Slayer", 1, 8, crit))
	}
	return out
}

// sneakable: a finesse or ranged weapon, with advantage, or with an ally of
// the rogue next to the target (and no disadvantage).
func sneakable(t *turnCtx, actor *domain.Creature, act domain.Action, target *domain.Creature, a Advantage) bool {
	if actor.SneakDice() == 0 || !act.Weapon || (!act.Finesse && !act.Ranged()) {
		return false
	}
	return a == Adv || (a == Normal && t != nil && t.g != nil && allyBeside(t.g, actor, target))
}

func allyBeside(g *domain.Game, actor, target *domain.Creature) bool {
	return slices.ContainsFunc(g.Creatures, func(c *domain.Creature) bool {
		return c != actor && !c.Down() && !domain.Hostile(actor, c) && c.Pos.DistanceFt(target.Pos) <= 5
	})
}

func extraDice(r Roller, label string, count, sides int, crit bool) domain.Roll {
	d := domain.Dice{Count: count, Sides: sides}
	if crit {
		d.Count *= 2
	}
	total, rolls := RollDice(r, d)
	return domain.Roll{Label: label, Dice: d.String(), Rolls: rolls, Total: total, Critical: crit}
}

// useInspiration spends a Bardic Inspiration die on a roll that would fail
// but could still succeed with it. It reports what happened.
func useInspiration(r Roller, c *domain.Creature, roll *domain.Roll) []string {
	if c.Inspiration == 0 || roll.Kept == 1 || roll.Total >= roll.Target || roll.Total+c.Inspiration < roll.Target {
		return nil
	}
	die, _ := RollDice(r, domain.Dice{Count: 1, Sides: c.Inspiration})
	c.Inspiration = 0
	roll.Total += die
	roll.Modifier += die
	roll.Label += fmt.Sprintf(" + inspiration %d", die)
	if roll.Success != nil {
		roll.Success = boolPtr(roll.Total >= roll.Target)
	}
	return []string{fmt.Sprintf("%s draws on Bardic Inspiration (+%d).", c.Name, die)}
}

// cuttingWords is a Lore bard's reaction: when a foe's attack would hit an
// ally by less than the die, the bard subtracts an inspiration die from it.
func cuttingWords(r Roller, t *turnCtx, attacker, target *domain.Creature, roll *domain.Roll) []string {
	if t == nil || t.g == nil || roll.Kept == 20 || roll.Total < roll.Target {
		return nil
	}
	bard := cuttingBard(t.g, attacker, target)
	if bard == nil || roll.Total-bard.InspirationDie() >= roll.Target {
		return nil
	}
	_ = bard.SpendFeature(domain.FeatBardicInspiration)
	react(t.g, bard)
	die, _ := RollDice(r, domain.Dice{Count: 1, Sides: bard.InspirationDie()})
	roll.Total -= die
	roll.Modifier -= die
	roll.Label += fmt.Sprintf(" - cutting words %d", die)
	return []string{fmt.Sprintf("%s's cutting words throw %s off (-%d).", bard.Name, attacker.Name, die)}
}

// cuttingBard finds a bard on the target's side who can react with Cutting Words.
func cuttingBard(g *domain.Game, attacker, target *domain.Creature) *domain.Creature {
	for _, c := range g.Creatures {
		if c.HasFeature(domain.FeatCuttingWords) && c.FeatureLeft(domain.FeatBardicInspiration) > 0 && c.CanAct() &&
			!domain.Hostile(c, target) && domain.Hostile(c, attacker) && !g.Encounter.Reacted[c.ID] &&
			c.Pos.DistanceFt(attacker.Pos) <= 60 {
			return c
		}
	}
	return nil
}

func rollDamage(r Roller, act domain.Action, crit bool) domain.Roll {
	d := act.Dice
	label := act.Name + " damage"
	if crit {
		d.Count *= 2
		label = act.Name + " critical damage"
	}
	total, rolls := RollDice(r, d)
	total = max(total, 0)
	return domain.Roll{Label: label, Dice: d.String(), Rolls: rolls, Modifier: d.Bonus, Total: total, Critical: crit}
}

func resolveSave(r Roller, actor *domain.Creature, act domain.Action, target *domain.Creature) Outcome {
	save := SavingThrow(r, target, act.SaveAbility, act.SaveDC, Normal)
	saved := save.Success != nil && *save.Success
	dmg := rollDamage(r, act, false)
	amount := dmg.Total
	switch {
	case saved && act.HalfOnSave:
		amount /= 2
	case saved:
		amount = 0
	}
	dmg.Total = amount
	out := Outcome{Rolls: []domain.Roll{save, dmg}, Hit: amount > 0, Amount: amount}
	if amount == 0 {
		out.Notes = []string{fmt.Sprintf("%s resists %s's %s.", target.Name, actor.Name, act.Name)}
		return out
	}
	out.Notes = hurt(target, amount, act.DamageType, false)
	return out
}

func resolveAutoHit(r Roller, act domain.Action, target *domain.Creature) Outcome {
	dmg := rollDamage(r, act, false)
	out := Outcome{Rolls: []domain.Roll{dmg}, Hit: true, Amount: dmg.Total}
	out.Notes = hurt(target, dmg.Total, act.DamageType, false)
	return out
}

func resolveHeal(r Roller, actor *domain.Creature, act domain.Action, target *domain.Creature) Outcome {
	total, rolls := RollDice(r, act.Dice)
	roll := domain.Roll{Label: fmt.Sprintf("%s heals %s", actor.Name, target.Name), Dice: act.Dice.String(), Rolls: rolls, Modifier: act.Dice.Bonus, Total: total}
	if act.SlotLevel > 0 && actor.HasFeature(domain.FeatDiscipleOfLife) {
		bonus := 2 + act.SlotLevel
		roll.Total += bonus
		roll.Modifier += bonus
		roll.Label += fmt.Sprintf(" (Disciple of Life +%d)", bonus)
	}
	return Outcome{Rolls: []domain.Roll{roll}, Hit: true, Amount: roll.Total, Notes: Heal(target, roll.Total)}
}

// XPThresholds are the XP totals needed for levels 1-5.
var XPThresholds = []int{0, 300, 900, 2700, 6500}

// MaxLevel is the highest level this game supports.
const MaxLevel = domain.MaxLevel

// LevelForXP returns the level a character with xp has earned (capped at MaxLevel).
func LevelForXP(xp int) int {
	level := 1
	for i, t := range XPThresholds {
		if xp >= t {
			level = i + 1
		}
	}
	return min(level, MaxLevel)
}
