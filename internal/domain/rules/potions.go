package rules

import (
	"dnd/internal/domain"
	"fmt"
	"slices"
)

// Option IDs for using items and fleeing.
const (
	PotionID = "potion_healing"
	FleeID   = "flee"
)

// healingPotion is the catalogue key of a Potion of Healing.
const healingPotion = "potion-healing"

// potionDice is what a Potion of Healing restores.
var potionDice = domain.Dice{Count: 2, Sides: 4, Bonus: 2}

// potionOptions offer a Potion of Healing, as an action: drink it when hurt,
// or pour it down the throat of a hurt or dying ally within reach.
func potionOptions(g *domain.Game, a *domain.Creature) []Option {
	if !slices.ContainsFunc(a.Items, isHealingPotion) {
		return nil
	}
	var out []Option
	for _, t := range g.Creatures {
		if t.Dead || t.HP >= t.MaxHP || domain.Hostile(a, t) || a.Pos.DistanceFt(t.Pos) > 5 {
			continue
		}
		summary := "Drink a Potion of Healing (2d4+2 HP)"
		if t != a {
			summary = fmt.Sprintf("Give %s a Potion of Healing (2d4+2 HP)", t.Name)
		}
		out = append(out, Option{ActionID: PotionID, ActionName: "Potion of Healing", TargetID: t.ID, TargetName: t.Name, Summary: summary})
	}
	return out
}

func isHealingPotion(it domain.Item) bool {
	return it.Base == healingPotion && it.Qty > 0
}

// usePotion uses up one potion and heals its target.
func usePotion(r Roller, g *domain.Game, a *domain.Creature, opt Option) (Outcome, error) {
	t, err := g.Creature(opt.TargetID)
	if err != nil {
		return Outcome{}, err
	}
	i := slices.IndexFunc(a.Items, isHealingPotion)
	a.Items[i].Qty--
	if a.Items[i].Qty == 0 {
		a.Items = slices.Delete(a.Items, i, i+1)
	}
	total, rolls := RollDice(r, potionDice)
	label := a.Name + " drinks a Potion of Healing"
	if t != a {
		label = a.Name + " gives " + t.Name + " a Potion of Healing"
	}
	roll := domain.Roll{Label: label, Dice: potionDice.String(), Rolls: rolls, Modifier: potionDice.Bonus, Total: total}
	return Outcome{Rolls: []domain.Roll{roll}, Hit: true, Amount: total, Notes: Heal(t, total)}, nil
}

// fleeOptions are all a turned creature may do: run from its foes.
func fleeOptions(sp *domain.TurnSpent) []Option {
	if sp.ActionsLeft() <= 0 {
		return nil
	}
	return []Option{{ActionID: FleeID, ActionName: "Flee", Summary: "Turned: flee as far as possible"}}
}

// flee dashes as far from the nearest foe as the creature can get.
func flee(r Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	g.Encounter.Spent.Dash += SpeedOf(a)
	to, _ := bestSquare(g, a, func(p domain.Position) (int, bool) {
		return -nearestFoe(g, a, p), true
	})
	out, err := moveTo(r, g, a, to)
	out.Notes = append(out.Notes, a.Name+" flees in terror.")
	return out, err
}

// nearestFoe is how far p is from the closest standing foe of a.
func nearestFoe(g *domain.Game, a *domain.Creature, p domain.Position) int {
	nearest := 1 << 30
	for _, c := range g.Creatures {
		if domain.Hostile(a, c) && !c.Down() {
			nearest = min(nearest, p.DistanceFt(c.Pos))
		}
	}
	return nearest
}
