package rules_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"strings"
	"testing"
)

// skirmish puts a fighter beside goblin g and a second goblin far off.
func skirmish(t *testing.T) (*domain.Game, *domain.Creature, *domain.Creature, *domain.Creature) {
	t.Helper()
	f, g, far := fighter(), goblin(), goblin()
	far.ID, far.Name = "g2", "Goblin 2"
	f.HP, f.MaxHP = 40, 40
	game := &domain.Game{Creatures: []*domain.Creature{f, g, far}, Party: []string{"f"}}
	rules.StartEncounter(dice(20, 1, 1), game)
	f.Pos, g.Pos, far.Pos = domain.Position{X: 2, Y: 2}, domain.Position{X: 3, Y: 2}, domain.Position{X: 9, Y: 7}
	return game, f, g, far
}

func TestLeavingReachProvokes(t *testing.T) {
	t.Parallel()
	game, f, g, far := skirmish(t)
	opt := rules.Option{ActionID: rules.ApproachID, TargetID: far.ID}
	out, err := rules.TakeTurn(dice(15, 4), game, "f", opt)
	if err != nil || !strings.HasPrefix(out.Rolls[0].Label, "Opportunity attack: Goblin attacks Fighter") || f.HP != 40-6 {
		t.Fatalf("walking away from the goblin draws its strike: %+v %v hp %d", out, err, f.HP)
	}
	if f.Pos.DistanceFt(g.Pos) <= 5 || !game.Encounter.Reacted[g.ID] || !strings.Contains(strings.Join(out.Notes, " "), "makes an opportunity attack") {
		t.Fatalf("the fighter still moves on, and the goblin's reaction is spent: %v", out.Notes)
	}
}

func TestOnlyOneOpportunityAttackPerReaction(t *testing.T) {
	t.Parallel()
	game, f, g, far := skirmish(t)
	game.Encounter.Reacted = map[string]bool{g.ID: true}
	out, err := rules.TakeTurn(dice(15, 4), game, "f", rules.Option{ActionID: rules.ApproachID, TargetID: far.ID})
	if err != nil || len(out.Rolls) != 0 || f.HP != 40 {
		t.Fatalf("a spent reaction can't strike: %+v %v", out, err)
	}
}

func TestDisengageAvoidsOpportunityAttacks(t *testing.T) {
	t.Parallel()
	game, f, g, _ := skirmish(t)
	d, ok := has(rules.Options(game, "f"), rules.DisengageID)
	if !ok {
		t.Fatal("Disengage is offered next to a foe")
	}
	out, err := rules.TakeTurn(dice(), game, "f", d)
	if err != nil || len(out.Rolls) != 0 || f.HP != 40 || f.Pos.DistanceFt(g.Pos) <= 5 || !out.TurnOver {
		t.Fatalf("falls back safely, as the action: %+v %v", out, err)
	}
	game.Encounter.Spent = domain.TurnSpent{}
	if _, ok := has(rules.Options(game, "f"), rules.DisengageID); ok {
		t.Fatal("nothing to disengage from")
	}
	if choice, _ := rules.MonsterChoice(dice(), game, g.ID); choice.ActionID == rules.DisengageID {
		t.Fatal("the engine's monsters press the attack")
	}
}

func TestAnOpportunityAttackCanStopAMove(t *testing.T) {
	t.Parallel()
	game, f, g, far := skirmish(t)
	f.HP = 3
	start := f.Pos
	out, err := rules.TakeTurn(dice(15, 6), game, "f", rules.Option{ActionID: "longsword", TargetID: far.ID})
	if err != nil || !f.Down() || f.Pos != start || len(out.Rolls) != 2 {
		t.Fatalf("struck down on the way, the fighter neither moves nor swings: %+v %v", out, err)
	}
	if g.HP != g.MaxHP {
		t.Fatal("nobody was attacked but the fighter")
	}
}

func TestNoOpportunityAttackWithoutAMeleeAttack(t *testing.T) {
	t.Parallel()
	game, _, g, far := skirmish(t)
	g.Actions = []domain.Action{{ID: "sling", Name: "Sling", Kind: domain.ActAttack, ToHit: 4, Dice: domain.MustDice("1d4"), RangeFt: 30}}
	if out, err := rules.TakeTurn(dice(15, 4), game, "f", rules.Option{ActionID: rules.ApproachID, TargetID: far.ID}); err != nil || len(out.Rolls) != 0 {
		t.Fatalf("a sling can't strike in reach: %+v %v", out, err)
	}
}

func TestTurnedUndeadProvokeAsTheyFlee(t *testing.T) {
	t.Parallel()
	f, z := fighter(), goblin()
	z.ID, z.Name, z.Type = "z", "Zombie", "undead"
	z.HP, z.MaxHP = 30, 30
	game := &domain.Game{Creatures: []*domain.Creature{f, z}, Party: []string{"f"}}
	rules.StartEncounter(dice(1, 20), game)
	f.Pos, z.Pos = domain.Position{X: 4, Y: 4}, domain.Position{X: 5, Y: 4}
	z.AddCondition(domain.Turned)
	out, err := rules.TakeTurn(dice(15, 4), game, "z", rules.Option{ActionID: rules.FleeID})
	if err != nil || !strings.HasPrefix(out.Rolls[0].Label, "Opportunity attack: Fighter attacks Zombie") {
		t.Fatalf("the fighter strikes the fleeing zombie: %+v %v", out, err)
	}
}
