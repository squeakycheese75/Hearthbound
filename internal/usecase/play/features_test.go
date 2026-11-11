package play_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"errors"
	"slices"
	"strings"
	"testing"
)

// fighting starts a fight against one sturdy dummy, on the human hero's turn.
func fighting(t *testing.T, g *domain.Game) *harness {
	t.Helper()
	h := setup(t, g)
	h.cat.monster = dummy(500, 0)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Steel rings."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestABonusActionLeavesTheTurnOpen(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	hero := g.Creatures[0]
	hero.Level, hero.HP = 2, 3 // Second Wind (hurt) and Action Surge (level 2)
	h := fighting(t, g)

	opts, _ := h.uc.Options(ctx, me, "g1")
	wind := findOption(t, opts, rules.SecondWindID)
	if wind.Cost != rules.CostBonus || findOption(t, opts, rules.ActionSurgeID).Cost != rules.CostFree {
		t.Fatalf("costs: %+v", opts)
	}
	for _, id := range []string{rules.SecondWindID, rules.ActionSurgeID, "longsword"} {
		if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, id)); err != nil {
			t.Fatal(err)
		}
		if opts, _ = h.uc.Options(ctx, me, "g1"); len(opts) == 0 {
			t.Fatalf("after %s the turn should still be open", id)
		}
	}
	g = h.store.game(t)
	if g.Encounter.Current() != "pc1" || g.Encounter.Spent.ActionsLeft() != 1 || !g.Encounter.Spent.Bonus {
		t.Fatalf("one action left after surging: %+v", g.Encounter.Spent)
	}
	if !h.pub.has(domain.EvRoll, "Second Wind") || !h.pub.has(domain.EvSystem, "one more action") {
		t.Fatal("the features are reported")
	}
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if hero, _ := g.Creature("pc1"); hero.FeatureLeft(domain.FeatSecondWind) != 0 || hero.FeatureLeft(domain.FeatActionSurge) != 0 {
		t.Fatalf("uses spent: %+v", hero.Features)
	}
	if g.Encounter.Round < 2 {
		t.Fatal("with nothing left to do, the turn passed on")
	}
}

func TestAgentHeroesUseTheirFeatures(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	persona := g.Creatures[1].Persona
	brute := build(t, "pc3", "Krusk", "Fighter", "Half-Orc", domain.ControlAI, persona)
	brute.OwnerID, brute.Level = me, 2
	g.Creatures = append(g.Creatures, brute)
	g.Party = append(g.Party, "pc3")
	cleric := g.Creatures[1]
	cleric.Level = 2
	g.Creatures[0].HP = 2 // the hero is badly hurt: the cleric should Preserve Life
	h := fighting(t, g)
	g = h.store.game(t)
	brute, _ = g.Creature("pc3")
	brute.HP = 1 // and the brute is hurt: Second Wind
	_ = h.store.Save(ctx, g)

	opts, _ := h.uc.Options(ctx, me, "g1")
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); err != nil {
		t.Fatal(err)
	}
	// Still hurt, the hero could catch a Second Wind; they end the turn instead.
	opts, _ = h.uc.Options(ctx, me, "g1")
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, rules.EndTurnID)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"surges with renewed vigour", "preserve life"} {
		if !h.pub.has(domain.EvSystem, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !h.pub.has(domain.EvRoll, "Krusk catches a Second Wind") {
		t.Error("the hurt brute catches its breath")
	}
	if n := countRolls(h, "Krusk attacks"); n < 2 {
		t.Errorf("Action Surge gives a second attack: %d attacks", n)
	}
}

func countRolls(h *harness, prefix string) int {
	n := 0
	for _, text := range h.pub.rollLabels() {
		if strings.HasPrefix(text, prefix) {
			n++
		}
	}
	return n
}

func TestLevelUpListsWhatIsNew(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Creatures[0].XP = 250
	h := setup(t, g)
	h.agent.on(domain.DMSeat, call("award_xp", `{"amount":50,"reason":"a clever plan"}`), say("Well done."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	var up *domain.LevelUp
	for _, e := range h.pub.all() {
		if e.Type == domain.EvLevelUp && e.Actor == "pc1" {
			up = e.LevelUp
		}
	}
	if up == nil || up.Level != 2 || !containsText(up.Gains, "New: Action Surge") || !containsText(up.Gains, "maximum hit points") {
		t.Fatalf("level-up gains: %+v", up)
	}
	if msg := lastToolResult(t, h); !containsText([]string{msg}, "reached level 2 and gained") {
		t.Fatalf("the DM hears about it: %s", msg)
	}
}

func TestRestsRechargeFeatures(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	persona := g.Creatures[1].Persona
	wizard := build(t, "pc3", "Quill", "Wizard", "Gnome", domain.ControlAI, persona)
	bard := build(t, "pc4", "Lark", "Bard", "Half-Elf", domain.ControlAI, persona)
	wizard.Level, bard.Level = 3, 2
	wizard.SlotsUsed = map[int]int{1: 2}
	g.Creatures = append(g.Creatures, wizard, bard)
	g.Party = append(g.Party, "pc3", "pc4")
	hero := g.Creatures[0]
	hero.Level = 2
	h := setup(t, g)
	g = h.store.game(t)
	hero, _ = g.Creature("pc1")
	hero.Features = []domain.Feature{{Key: domain.FeatActionSurge, Max: 1, Used: 1, Recharge: domain.RechargeShort}}
	hero.HP = 1
	_ = h.store.Save(ctx, g)
	h.agent.on(domain.DMSeat, call("rest", `{"kind":"short"}`), say("You rest."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	hero, _ = g.Creature("pc1")
	wizard, _ = g.Creature("pc3")
	if hero.FeatureLeft(domain.FeatActionSurge) != 1 || wizard.SlotsUsed[1] != 0 || wizard.FeatureLeft(domain.FeatArcaneRecovery) != 0 {
		t.Fatalf("short rest: surge %d, wizard slots used %v", hero.FeatureLeft(domain.FeatActionSurge), wizard.SlotsUsed)
	}
	if !h.pub.has(domain.EvSystem, "Arcane Recovery") || !h.pub.has(domain.EvSystem, "A song soothes") {
		t.Fatal("arcane recovery and song of rest are reported")
	}
}

func containsText(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// lastToolResult is the most recent tool result sent back to the DM.
func lastToolResult(t *testing.T, h *harness) string {
	t.Helper()
	for _, turn := range slices.Backward(h.agent.seatTurns(domain.DMSeat)) {
		for _, msg := range slices.Backward(turn.History) {
			if n := len(msg.ToolResults); n > 0 {
				return msg.ToolResults[n-1].Content
			}
		}
	}
	t.Fatal("no tool results")
	return ""
}

func TestLevelFourBringsAnAbilityScoreImprovement(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	hero := g.Creatures[0]
	hero.Level, hero.XP = 3, 2650
	h := setup(t, g)
	h.agent.on(domain.DMSeat, call("award_xp", `{"amount":50,"reason":"the crypt is cleared"}`), say("Onward."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	hero, _ = g.Creature("pc1")
	if hero.Level != 4 || hero.ASI != 1 || !containsText(lastLevelUp(h).Gains, "Ability Score Improvement") {
		t.Fatalf("level %d asi %d", hero.Level, hero.ASI)
	}
	maxHP, con := hero.MaxHP, hero.Abilities[domain.CON]
	cases := []struct {
		name, account, character string
		inc                      map[domain.Ability]int
		want                     error
	}{
		{"not yours", "someone", "pc1", map[domain.Ability]int{domain.CON: 2}, domain.ErrNotFound},
		{"no such hero", me, "pc9", map[domain.Ability]int{domain.CON: 2}, domain.ErrForbidden},
		{"too much", me, "pc1", map[domain.Ability]int{domain.CON: 1}, domain.ErrInvalid},
		{"chosen", me, "pc1", map[domain.Ability]int{domain.CON: 2}, nil},
		{"none left", me, "pc1", map[domain.Ability]int{domain.CON: 2}, domain.ErrInvalid},
	}
	for _, tc := range cases {
		if err := h.uc.Improve(ctx, tc.account, "g1", tc.character, tc.inc); !errors.Is(err, tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	hero, _ = h.store.game(t).Creature("pc1")
	if hero.Abilities[domain.CON] != con+2 || hero.MaxHP != maxHP+4 || !h.pub.has(domain.EvSystem, "grows stronger: CON +2") {
		t.Fatalf("CON %d max HP %d (was %d): a higher CON modifier adds HP for every level", hero.Abilities[domain.CON], hero.MaxHP, maxHP)
	}
	h.cat.failClass = true
	hero.ASI = 1
	if err := h.uc.Improve(ctx, me, "g1", "pc1", map[domain.Ability]int{domain.STR: 2}); err == nil {
		t.Fatal("an unknown class can't be recalculated")
	}
}

func lastLevelUp(h *harness) *domain.LevelUp {
	var up *domain.LevelUp
	for _, e := range h.pub.all() {
		if e.Type == domain.EvLevelUp {
			up = e.LevelUp
		}
	}
	if up == nil {
		return &domain.LevelUp{}
	}
	return up
}

func TestExtraAttackAtLevelFive(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	persona := g.Creatures[1].Persona
	brute := build(t, "pc3", "Krusk", "Fighter", "Half-Orc", domain.ControlAI, persona)
	brute.OwnerID, brute.Level = me, 5
	g.Creatures = append(g.Creatures, brute)
	g.Party = append(g.Party, "pc3")
	h := fighting(t, g)
	opts, _ := h.uc.Options(ctx, me, "g1")
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); err != nil {
		t.Fatal(err)
	}
	if n := countRolls(h, "Krusk attacks"); n < 2 || n%2 != 0 {
		t.Fatalf("Extra Attack: two attacks per Attack action, got %d", n)
	}
}

func TestAFirstAdventureGuidesTheDM(t *testing.T) {
	t.Parallel()
	for _, tutorial := range []bool{true, false} {
		g := newGame(t)
		g.Tutorial = tutorial
		h := setup(t, g)
		h.agent.on(domain.DMSeat, say("The tavern is warm."))
		if err := h.uc.Begin(ctx, me, "g1"); err != nil {
			t.Fatal(err)
		}
		system := h.agent.seatTurns(domain.DMSeat)[0].System
		if got := strings.Contains(system, "This is the player's first adventure"); got != tutorial {
			t.Errorf("tutorial %v: guidance in the DM's prompt = %v", tutorial, got)
		}
	}
}
