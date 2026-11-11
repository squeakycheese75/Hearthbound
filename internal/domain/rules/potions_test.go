package rules_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"slices"
	"strings"
	"testing"
)

func potion(qty int) domain.Item {
	return domain.Item{ID: "p1", Base: "potion-healing", Name: "Potion of Healing", Kind: domain.ItemPotion, Qty: qty}
}

func TestPotionsInAFight(t *testing.T) {
	t.Parallel()
	f, g := fighter(), goblin()
	ally := fighter()
	ally.ID, ally.Name, ally.Pos = "a", "Ally", domain.Position{Y: 1}
	f.Items = []domain.Item{potion(2)}
	game := &domain.Game{Creatures: []*domain.Creature{f, ally, g}, Party: []string{"f", "a"}}
	rules.StartEncounter(dice(20, 1, 1), game)
	ally.Pos = domain.Position{X: f.Pos.X, Y: f.Pos.Y + 1} // the encounter places everyone
	if _, ok := has(rules.Options(game, "f"), rules.PotionID); ok {
		t.Fatal("no potion while nobody is hurt")
	}
	f.HP, ally.HP = 5, 0
	ally.AddCondition(domain.Unconscious)
	var targets []string
	for _, o := range rules.Options(game, "f") {
		if o.ActionID == rules.PotionID {
			targets = append(targets, o.TargetID+":"+o.Summary)
		}
	}
	if len(targets) != 2 || !strings.HasPrefix(targets[1], "a:Give Ally a Potion") {
		t.Fatalf("drink one, or pour one for the dying ally beside you: %v", targets)
	}
	out, err := rules.TakeTurn(dice(4, 4), game, "f", rules.Option{ActionID: rules.PotionID, TargetID: "a"})
	if err != nil || out.Amount != 10 || ally.HP != 10 || ally.Has(domain.Unconscious) || !out.TurnOver {
		t.Fatalf("2d4+2 brings the ally round, as the action: %+v %v hp %d", out, err, ally.HP)
	}
	if f.Items[0].Qty != 1 || !strings.Contains(out.Rolls[0].Label, "gives Ally") {
		t.Fatalf("one potion used: %+v", f.Items)
	}
	game.Encounter.Spent = domain.TurnSpent{}
	if _, err := rules.TakeTurn(dice(1, 1), game, "f", rules.Option{ActionID: rules.PotionID, TargetID: "f"}); err != nil || f.HP != 9 || len(f.Items) != 0 {
		t.Fatalf("the last potion is drunk and gone: hp %d items %+v %v", f.HP, f.Items, err)
	}
	game.Encounter.Spent = domain.TurnSpent{}
	if _, ok := has(rules.Options(game, "f"), rules.PotionID); ok {
		t.Fatal("no potions left")
	}
	ally.Pos = domain.Position{X: f.Pos.X, Y: f.Pos.Y + 5}
	f.Items = []domain.Item{potion(1)}
	if opts := rules.Options(game, "f"); slices.ContainsFunc(opts, func(o rules.Option) bool { return o.TargetID == "a" && o.ActionID == rules.PotionID }) {
		t.Fatal("only allies within reach")
	}
}

func TestTurnedUndeadFlee(t *testing.T) {
	t.Parallel()
	f, z := fighter(), goblin()
	z.ID, z.Name, z.Type = "z", "Zombie", "undead"
	game := &domain.Game{Creatures: []*domain.Creature{f, z}, Party: []string{"f"}}
	rules.StartEncounter(dice(1, 20), game)
	z.AddCondition(domain.Turned)
	opts := rules.Options(game, "z")
	if len(opts) != 1 || opts[0].ActionID != rules.FleeID {
		t.Fatalf("a turned creature can only flee: %+v", opts)
	}
	choice, ok := rules.MonsterChoice(dice(), game, "z")
	if !ok || choice.ActionID != rules.FleeID {
		t.Fatalf("monsters flee when turned: %+v", choice)
	}
	before := z.Pos.DistanceFt(f.Pos)
	out, err := rules.TakeTurn(dice(), game, "z", choice)
	if err != nil || !out.TurnOver || z.Pos.DistanceFt(f.Pos) <= before+10 {
		t.Fatalf("it dashes away, to the edge of the map (from %d ft to %d ft): %v", before, z.Pos.DistanceFt(f.Pos), err)
	}
	if len(rules.Options(game, "z")) != 0 {
		t.Fatal("fleeing takes the action")
	}
}

func TestMultiattackMonstersStrikeTwice(t *testing.T) {
	t.Parallel()
	f, o := fighter(), goblin()
	o.ID, o.Name = "o", "Owlbear"
	o.Features = []domain.Feature{{Key: domain.FeatExtraAttack, Name: "Multiattack"}}
	f.HP, f.MaxHP = 100, 100
	game := &domain.Game{Creatures: []*domain.Creature{f, o}, Party: []string{"f"}}
	rules.StartEncounter(dice(1, 20), game)
	first, _ := rules.MonsterChoice(dice(), game, "o")
	out, err := rules.TakeTurn(dice(15, 3), game, "o", first)
	if err != nil || out.TurnOver {
		t.Fatalf("the first attack leaves the second: %+v %v", out, err)
	}
	second, ok := rules.MonsterChoice(dice(), game, "o")
	if !ok || second.ActionID != "scimitar" {
		t.Fatalf("second attack: %+v", second)
	}
	if out, err := rules.TakeTurn(dice(15, 3), game, "o", second); err != nil || !out.TurnOver {
		t.Fatalf("two attacks spend the action: %+v %v", out, err)
	}
}
