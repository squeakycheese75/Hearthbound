package rules

import (
	"dnd/internal/domain"
	"slices"
)

// DisengageID is the Disengage action: fall back out of reach safely.
const DisengageID = "disengage"

// disengageOptions offer Disengage to a creature standing next to a foe.
func disengageOptions(g *domain.Game, a *domain.Creature) []Option {
	if !slices.ContainsFunc(g.Creatures, func(c *domain.Creature) bool { return threatens(c, a, a.Pos) }) {
		return nil
	}
	return []Option{{ActionID: DisengageID, ActionName: "Disengage", Summary: "Disengage: fall back out of reach without provoking opportunity attacks"}}
}

// disengage moves as far from the nearest foe as the creature's speed allows,
// without provoking.
func disengage(_ Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	a.Pos, _ = bestSquare(g, a, func(p domain.Position) (int, bool) {
		return -nearestFoe(g, a, p), true
	})
	return Outcome{Notes: []string{a.Name + " disengages and falls back."}}, nil
}

// moveTo moves a creature, and every foe whose reach it leaves may make an
// opportunity attack with its reaction. A creature knocked down stops where
// it stands.
func moveTo(r Roller, g *domain.Game, mover *domain.Creature, to domain.Position) (Outcome, error) {
	var out Outcome
	for _, c := range g.Creatures {
		act, ok := reachAttack(c)
		if mover.Down() || !ok || !threatens(c, mover, mover.Pos) || threatens(c, mover, to) || g.Encounter.Reacted[c.ID] {
			continue
		}
		react(g, c)
		res, err := resolveKind(r, &turnCtx{g: g}, c, act, mover)
		if err != nil {
			return Outcome{}, err
		}
		for _, roll := range res.Rolls {
			roll.Label = "Opportunity attack: " + roll.Label
			out.Rolls = append(out.Rolls, roll)
		}
		out.Notes = append(append(out.Notes, c.Name+" makes an opportunity attack on "+mover.Name+"."), res.Notes...)
	}
	if !mover.Down() {
		mover.Pos = to
	}
	return out, nil
}

// threatens reports whether c could strike mover standing at p: a standing,
// hostile creature within 5 ft that can still act.
func threatens(c, mover *domain.Creature, p domain.Position) bool {
	return c != mover && domain.Hostile(c, mover) && c.CanAct() && c.Pos.DistanceFt(p) <= 5
}

// reachAttack is the melee attack a creature makes an opportunity attack with.
func reachAttack(c *domain.Creature) (domain.Action, bool) {
	i := slices.IndexFunc(c.Actions, func(a domain.Action) bool {
		return a.Kind == domain.ActAttack && a.RangeFt <= 5 && a.SlotLevel == 0
	})
	if i < 0 {
		return domain.Action{}, false
	}
	return c.Actions[i], true
}

// merge puts what happened on the way before what happened on arrival.
func merge(first, then Outcome) Outcome {
	then.Rolls = append(first.Rolls, then.Rolls...)
	then.Notes = append(first.Notes, then.Notes...)
	return then
}
