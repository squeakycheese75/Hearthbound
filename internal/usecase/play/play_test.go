package play_test

import (
	"context"
	"dnd/internal/adapter/srd"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"dnd/internal/usecase/play"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

var ctx = context.Background()

func calls(pairs ...string) play.Reply {
	r := say("")
	for i := 0; i+1 < len(pairs); i += 2 {
		r.Message.ToolCalls = append(r.Message.ToolCalls, domain.ToolCall{
			ID: fmt.Sprintf("c%d", callSeq.Add(1)), Name: pairs[i], Input: json.RawMessage(pairs[i+1]),
		})
	}
	return r
}

func TestBeginSetsSceneAndIsIdempotent(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.agent.on(domain.DMSeat,
		call("set_scene", `{"scene_key":"forest","title":"The Whispering Wood","mood":"eerie"}`),
		say("Mist curls between the trees."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if !h.art.has("games/g1/pc1") || !h.art.has("games/g1/pc2") || !h.art.has("games/g1/scene-forest-the-whispering-wood") {
		t.Fatalf("expected party portraits and scene art to be requested: %v", h.art.keys)
	}
	if g.Scene.Key != "forest" || !g.Started || !g.Awaiting || g.Usage.Input != 40 { // DM twice, the cast check, the choice helper
		t.Fatalf("game after begin: scene=%+v started=%v awaiting=%v usage=%+v", g.Scene, g.Started, g.Awaiting, g.Usage)
	}
	if !h.pub.has(domain.EvNarration, "Mist curls") || len(h.pub.texts(domain.EvNarrationDelta)) == 0 {
		t.Fatal("expected streamed narration")
	}
	turns := h.agent.seatTurns(domain.DMSeat)
	if len(turns) != 2 || turns[0].Effort != "medium" || !strings.Contains(turns[0].System, "Aria") || len(turns[1].History) != 3 {
		t.Fatalf("dm turns %d", len(turns))
	}
	if err := h.uc.Begin(ctx, me, "g1"); err != nil || len(h.agent.seatTurns(domain.DMSeat)) != 2 {
		t.Fatal("second Begin must not call the DM again")
	}
}

func TestActRunsCompanionsThenEveryDMTool(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	wiz := build(t, "pc3", "Old Tom", "Wizard", "Gnome", domain.ControlAI, nil)
	wiz.Dead = true
	g.Creatures = append(g.Creatures, wiz)
	g.Party = append(g.Party, "pc3")
	g.Creatures[0].XP, g.Creatures[1].XP = 250, 250 // 50 more reaches level 2, within the per-turn cap
	h := setup(t, g)
	h.agent.on("pc2", say(`"Stay behind me." *raises shield*`))
	h.agent.on(domain.DMSeat, calls(
		"set_scene", `{"scene_key":"forest","title":"Wood","mood":"dark"}`,
		"set_scene", `{"scene_key":"moon","title":"Moon","mood":"x"}`,
		"ability_check", `{"character_id":"pc1","skill":"athletics","dc":12,"reason":"climb"}`,
		"ability_check", `{"character_id":"pc2","ability":"wis","dc":25,"reason":"sense"}`,
		"ability_check", `{"character_id":"pc1","skill":"juggling","dc":10,"reason":"x"}`,
		"ability_check", `{"character_id":"pc1","ability":"luck","dc":10,"reason":"x"}`,
		"ability_check", `{"character_id":"nobody","dc":10,"reason":"x"}`,
		"saving_throw", `{"character_id":"pc1","ability":"CON","dc":10,"reason":"poison gas"}`,
		"saving_throw", `{"character_id":"pc1","ability":"XYZ","dc":10,"reason":"x"}`,
		"saving_throw", `{"character_id":"nobody","ability":"CON","dc":10,"reason":"x"}`,
		"deal_damage", `{"character_id":"pc2","dice":"1d4","damage_type":"fire","reason":"brazier"}`,
		"deal_damage", `{"character_id":"pc2","dice":"lots","damage_type":"fire","reason":"x"}`,
		"deal_damage", `{"character_id":"nobody","dice":"1d4","damage_type":"fire","reason":"x"}`,
		"heal", `{"character_id":"pc2","dice":"1d4","reason":"potion"}`,
		"heal", `{"character_id":"nobody","dice":"1d4","reason":"x"}`,
		"set_condition", `{"character_id":"pc1","condition":"poisoned","active":true}`,
		"set_condition", `{"character_id":"pc1","condition":"prone","active":true}`,
		"set_condition", `{"character_id":"pc1","condition":"prone","active":false}`,
		"set_condition", `{"character_id":"nobody","condition":"prone","active":true}`,
		"give_item", `{"character_id":"pc1","item":"Rope","gold":5}`,
		"give_item", `{"character_id":"pc1","gold":3}`,
		"give_item", `{"character_id":"pc1","gold":-1000}`,
		"give_item", `{"character_id":"nobody","item":"x"}`,
		"rest", `{"kind":"short"}`,
		"award_xp", `{"amount":50,"reason":"bravery"}`,
		"start_combat", `{"monsters":[{"key":"tarrasque","count":1}]}`,
		"no_such_tool", `{}`,
		"ability_check", `{"dc":"high"}`,
	), say("The forest falls silent."))

	if err := h.uc.Act(ctx, me, "g1", "I climb the old oak."); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	hero, _ := g.Hero()
	cleric, _ := g.Creature("pc2")
	if g.Scene.Key != "forest" || hero.Gold != 18 || len(hero.Items) != 7 || hero.Items[6].Name != "Rope (50 ft)" || len(hero.Inventory) != 0 {
		t.Fatalf("scene=%s gold=%d items=%+v", g.Scene.Key, hero.Gold, hero.Items)
	}
	if _, err := g.Creature("pc3"); err == nil || !h.pub.has(domain.EvSystem, "Old Tom has fallen and is laid to rest.") {
		t.Fatal("the fallen wizard is retired from the party")
	}
	if !hero.Has(domain.Poisoned) || hero.Has(domain.Prone) || hero.Level != 2 || cleric.Level != 2 {
		t.Fatalf("hero conds=%v level=%d cleric level=%d", hero.Conditions, hero.Level, cleric.Level)
	}
	if !h.pub.has(domain.EvLevelUp, "reaches level 2") {
		t.Error("missing level-up event")
	}
	for _, want := range []string{"short rest", "receives Rope (50 ft) and +5 gold", "is no longer prone"} {
		if !h.pub.has(domain.EvSystem, want) {
			t.Errorf("missing system event %q", want)
		}
	}
	for text, want := range map[string]domain.Cue{
		"reaches level 2": domain.CueLevel, "short rest": domain.CueRest, "receives Rope": domain.CueLoot,
		"laid to rest": domain.CueFall, "is no longer prone": "", "receives +3 gold": domain.CueCoins,
	} {
		if got := h.pub.cue(text); got != want {
			t.Errorf("%q: cue %q, want %q", text, got, want)
		}
	}
	dmTurns := h.agent.seatTurns(domain.DMSeat)
	first := dmTurns[0].History[0].Text
	if !strings.Contains(first, "Stay behind me") || !strings.Contains(first, "I climb the old oak") || !strings.Contains(first, "DEAD") {
		t.Fatalf("DM did not see the party's actions: %s", first)
	}
	results := dmTurns[1].History[len(dmTurns[1].History)-1].ToolResults
	errCount := 0
	for _, r := range results {
		if r.IsError {
			errCount++
		}
	}
	if errCount != 15 {
		t.Fatalf("expected 15 tool errors, got %d: %+v", errCount, results)
	}
	compTurn := h.agent.seatTurns("pc2")[0]
	if compTurn.Effort != "low" || !strings.Contains(compTurn.History[0].Text, "(nothing yet)") {
		t.Fatalf("companion turn %+v", compTurn.History[0].Text)
	}
}

func TestLongRestRestoresEverything(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	cleric, _ := g.Creature("pc2")
	cleric.HP, cleric.SlotsUsed, cleric.Conditions = 1, map[int]int{1: 2}, []domain.Condition{domain.Poisoned}
	h := setup(t, g)
	h.agent.on(domain.DMSeat, call("rest", `{"kind":"long"}`), say("Dawn."))
	if err := h.uc.Act(ctx, me, "g1", "We sleep."); err != nil {
		t.Fatal(err)
	}
	c, _ := h.store.game(t).Creature("pc2")
	if c.HP != c.MaxHP || c.SlotsLeft(1) != 2 || len(c.Conditions) != 0 {
		t.Fatalf("after long rest %+v", c)
	}
}

func TestLevelUpNeedsTemplates(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"class"} {
		g := newGame(t)
		g.Creatures[0].XP = 250
		h := setup(t, g)
		h.cat.failClass = mode == "class"
		h.agent.on(domain.DMSeat, call("award_xp", `{"amount":50,"reason":"x"}`), say("ok"))
		if err := h.uc.Act(ctx, me, "g1", "go"); err != nil {
			t.Fatal(err)
		}
		hero, _ := h.store.game(t).Hero()
		if hero.XP != 300 || hero.Level != 1 || !h.pub.has(domain.EvSystem, "could not level up") {
			t.Fatalf("%s: xp=%d level=%d", mode, hero.XP, hero.Level)
		}
	}
}

func dummy(hp, xp int) func(key, id string) (*domain.Creature, error) {
	return func(key, id string) (*domain.Creature, error) {
		return &domain.Creature{
			ID: id, Name: "Training Dummy", Kind: domain.KindMonster, Portrait: key, Controller: domain.ControlDM, Level: 1,
			Abilities: domain.AbilityScores{domain.DEX: 10}, MaxHP: hp, HP: hp, AC: 5, Speed: 30, XPValue: xp,
			Actions: []domain.Action{{ID: "flail", Name: "Flail", Kind: domain.ActAttack, ToHit: 0, Dice: domain.MustDice("1d4"), RangeFt: 5}},
		}, nil
	}
}

func TestCombatVictory(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Creatures[0].XP = 250 // 50 XP from this fight levels the hero up
	h := setup(t, g)
	h.cat.monster = dummy(20, 100)
	h.agent.on(domain.DMSeat,
		call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`),
		say("The dummy lurches to life!"),
		call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), // round narration: not allowed mid-fight
		say("Splinters fly."),
		say("Victory is yours."),
	)
	h.agent.on("pc2",
		call("choose_action", `{"option":99,"line":"?"}`),
		call("choose_action", `{"option":0,"line":"For the light!"}`),
		say("Done."))

	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if g.Mode != domain.ModeCombat || g.Encounter.Current() != "pc1" || !g.Awaiting {
		t.Fatalf("expected the hero's turn, mode=%s current=%s", g.Mode, g.Encounter.Current())
	}
	opts, err := h.uc.Options(ctx, me, "g1")
	if err != nil || len(opts) == 0 {
		t.Fatalf("options %v %v", opts, err)
	}
	swing := findOption(t, opts, "longsword")
	if err := h.uc.Fight(ctx, me, "g1", swing); err != nil {
		t.Fatal(err)
	}
	if !h.pub.has(domain.EvSpeech, "For the light!") || !h.pub.has(domain.EvNarration, "Splinters fly") {
		t.Fatal("expected companion battle cry and round narration")
	}
	if err := h.uc.Fight(ctx, me, "g1", swing); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	hero, _ := g.Hero()
	if g.Mode != domain.ModeExploration || g.Encounter != nil || hero.XP != 300 || hero.Level != 2 {
		t.Fatalf("after victory mode=%s xp=%d level=%d", g.Mode, hero.XP, hero.Level)
	}
	if !h.pub.has(domain.EvNarration, "Victory is yours") || len(g.Monsters()) != 0 {
		t.Fatal("expected aftermath narration and cleared monsters")
	}
	if opts, _ := h.uc.Options(ctx, me, "g1"); opts != nil {
		t.Fatal("no options outside combat")
	}
}

func findOption(t *testing.T, opts []rules.Option, action string) rules.Option {
	t.Helper()
	for _, o := range opts {
		if o.ActionID == action {
			return o
		}
	}
	t.Fatalf("no %s option in %+v", action, opts)
	return rules.Option{}
}

func TestCombatDefeat(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	for _, c := range g.Creatures {
		c.HP = 1
	}
	h := setup(t, g)
	h.cat.monster = func(key, id string) (*domain.Creature, error) {
		return &domain.Creature{
			ID: id, Name: "Brute", Kind: domain.KindMonster, Portrait: key, Controller: domain.ControlDM, Level: 1,
			Abilities: domain.AbilityScores{domain.DEX: 30}, MaxHP: 100, HP: 100, AC: 30, Speed: 30,
			Actions: []domain.Action{{ID: "smash", Name: "Smash", Kind: domain.ActAttack, ToHit: 20, Dice: domain.MustDice("1d4+10"), RangeFt: 5}},
		}, nil
	}
	h.agent.on(domain.DMSeat,
		call("start_combat", `{"monsters":[{"key":"brute","count":2,"name":"Ogre Twin"}]}`),
		say("Two brutes charge!"),
		say("Bones crack."),
		call("end_adventure", `{"epilogue":"Your names fade into legend."}`),
		say("The end."),
	)
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if g.Mode != domain.ModeOver || g.Awaiting {
		t.Fatalf("mode=%s awaiting=%v", g.Mode, g.Awaiting)
	}
	if !h.art.has("monsters/brute") {
		t.Fatalf("monster portrait not requested: %v", h.art.keys)
	}
	if !h.pub.has(domain.EvNarration, "fade into legend") || !h.pub.has(domain.EvSystem, "Ogre Twin 2") {
		t.Fatal("expected epilogue and named twins")
	}
}

func TestDyingHeroAndEndlessFight(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	hero, _ := g.Hero()
	hero.HP = 0 // still played live, so the fight goes on while they make death saves
	hero.AddCondition(domain.Unconscious)
	cleric, _ := g.Creature("pc2")
	cleric.Persona, cleric.Actions, cleric.Class = nil, nil, "Mime" // no attacks, and an unknown class keeps its sheet as is
	h := setup(t, g)
	h.cat.monster = func(key, id string) (*domain.Creature, error) {
		m, _ := dummy(50, 0)(key, id)
		m.Actions[0].ToHit, m.Actions[0].Dice = -50, domain.Dice{}
		return m, nil
	}
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Stalemate."))
	err := h.uc.Begin(ctx, me, "g1")
	if err == nil || !strings.Contains(err.Error(), "combat ran too long") {
		t.Fatalf("expected endless-combat guard, got %v", err)
	}
	g = h.store.game(t)
	if c, _ := g.Creature("pc1"); !c.Stable {
		t.Fatal("hero should have stabilised from death saves")
	}
	if opts, err := h.uc.Options(ctx, me, "g1"); err != nil || opts != nil {
		t.Fatalf("an unconscious hero has no options: %v %v", opts, err)
	}
	if err := h.uc.Act(ctx, me, "g1", "hello"); !errors.Is(err, domain.ErrWrongMode) {
		t.Fatalf("act during combat: %v", err)
	}
}

func TestActWithoutHeroAndBadInput(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Creatures[0].Controller = domain.ControlAI
	h := setup(t, g)
	if err := h.uc.Act(ctx, me, "g1", "hi"); !errors.Is(err, domain.ErrNotYourTurn) {
		t.Fatalf("got %v", err)
	}
	if err := h.uc.Act(ctx, me, "g1", "   "); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	if err := h.uc.Fight(ctx, me, "g1", rules.Option{}); !errors.Is(err, domain.ErrWrongMode) {
		t.Fatalf("got %v", err)
	}
}

func TestFightRejectsIllegalOption(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.cat.monster = dummy(20, 0)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	err := h.uc.Fight(ctx, me, "g1", rules.Option{ActionID: "fireball", TargetID: "dummy-1"})
	if !errors.Is(err, domain.ErrInvalid) || !h.pub.has(domain.EvError, "not available") {
		t.Fatalf("got %v", err)
	}
}

func TestAgentAndStoreFailures(t *testing.T) {
	t.Parallel()
	boom := errors.New("overloaded")

	h := setup(t, newGame(t))
	h.agent.errs[domain.DMSeat] = boom
	if err := h.uc.Begin(ctx, me, "g1"); !errors.Is(err, boom) || !h.pub.has(domain.EvError, "overloaded") {
		t.Fatalf("dm failure: %v", err)
	}

	h = setup(t, newGame(t))
	h.agent.errs["pc2"] = boom
	if err := h.uc.Act(ctx, me, "g1", "hello"); err != nil || !h.pub.has(domain.EvSystem, "Mae hesitates") {
		t.Fatalf("companion failure should not stop the turn: %v", err)
	}

	h = setup(t, newGame(t))
	h.store.loadErr = boom
	if err := h.uc.Begin(ctx, me, "g1"); !errors.Is(err, boom) {
		t.Fatalf("load: %v", err)
	}
	if _, err := h.uc.Options(ctx, me, "g1"); !errors.Is(err, boom) {
		t.Fatalf("options load: %v", err)
	}

	h = setup(t, newGame(t))
	h.store.saveErr = boom
	if err := h.uc.Begin(ctx, me, "g1"); !errors.Is(err, boom) {
		t.Fatalf("save: %v", err)
	}
}

func TestCompanionCombatAgentFailureFallsBack(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.cat.monster = dummy(200, 0)
	h.agent.errs["pc2"] = errors.New("timeout")
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	opts, _ := h.uc.Options(ctx, me, "g1")
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); err != nil {
		t.Fatal(err)
	}
	if !h.pub.has(domain.EvRoll, "Mae attacks") {
		t.Fatal("cleric should still act using the engine's choice")
	}
}

func TestToolStepBudget(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	store, agent, pub := newStore(g), newAgent(), &recorder{}
	for range 5 {
		agent.on(domain.DMSeat, call("rest", `{"kind":"short"}`))
	}
	uc := play.New(store, agent, newRoles(t), pub, &dice{fixed: 10}, &catalog{Catalog: srd.New()}, &easel{}, newClock(), slog.New(slog.DiscardHandler), play.Config{MaxToolSteps: 2})
	if err := uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if n := len(agent.seatTurns(domain.DMSeat)); n != 2 {
		t.Fatalf("expected 2 DM calls, got %d", n)
	}
}

func TestBusyTable(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.agent.called, h.agent.block = make(chan struct{}, 100), make(chan struct{})
	done := make(chan error)
	go func() { done <- h.uc.Begin(ctx, me, "g1") }()
	<-h.agent.called
	if !h.uc.Busy("g1") {
		t.Fatal("game should report busy while a turn runs")
	}
	if err := h.uc.Act(ctx, me, "g1", "wait!"); !errors.Is(err, domain.ErrBusy) {
		t.Fatalf("got %v", err)
	}
	close(h.agent.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if h.uc.Busy("g1") {
		t.Fatal("busy flag must clear when the turn ends")
	}
}

func TestCompanionSeesLastNarrationAndCannotUseTools(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.agent.on(domain.DMSeat, say("A hooded stranger waves you over."))
	h.agent.on("pc2", call("choose_action", `{"option":0,"line":"x"}`), say(`"Careful."`))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Act(ctx, me, "g1", "I approach the stranger."); err != nil {
		t.Fatal(err)
	}
	turns := h.agent.seatTurns("pc2")
	if !strings.Contains(turns[0].History[0].Text, "hooded stranger") {
		t.Fatal("companion should see the DM's last narration")
	}
	if r := turns[1].History[2].ToolResults[0]; !r.IsError {
		t.Fatalf("tool use outside combat should be refused: %+v", r)
	}
}

func TestCorruptEncounterIsReported(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	m, _ := dummy(10, 0)("dummy", "dummy-1")
	g.Creatures = append(g.Creatures, m)
	g.Mode, g.Encounter = domain.ModeCombat, &domain.Encounter{Round: 1, Order: []string{"ghost"}}
	h := setup(t, g)
	if err := h.uc.Begin(ctx, me, "g1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestMonsterWithBrokenActionStopsCombat(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.cat.monster = func(key, id string) (*domain.Creature, error) {
		m, _ := dummy(10, 0)(key, id)
		m.Abilities[domain.DEX] = 30
		m.Actions[0].Kind = "interpretive_dance"
		return m, nil
	}
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestDMFailsDuringRoundNarration(t *testing.T) {
	t.Parallel()
	boom := errors.New("overloaded")
	// Hero acts last in the round, so the narration runs straight after Fight.
	g := newGame(t)
	g.Creatures, g.Party = g.Creatures[:1], g.Party[:1]
	h := setup(t, g)
	h.cat.monster = func(key, id string) (*domain.Creature, error) {
		m, _ := dummy(100, 0)(key, id)
		m.Abilities[domain.DEX] = 30
		return m, nil
	}
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	opts, _ := h.uc.Options(ctx, me, "g1")
	h.agent.errs[domain.DMSeat] = boom
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); !errors.Is(err, boom) {
		t.Fatalf("fight: %v", err)
	}

	// With the cleric in the party the round ends on a monster turn inside the combat loop.
	h = setup(t, newGame(t))
	h.cat.monster = dummy(100, 0)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	opts, _ = h.uc.Options(ctx, me, "g1")
	h.agent.errs[domain.DMSeat] = boom
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); !errors.Is(err, boom) {
		t.Fatalf("combat loop: %v", err)
	}
}

func TestOnlyPartyMembersMayPlayAndPayersAreRight(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	cleric, _ := g.Creature("pc2")
	cleric.OwnerID = "acct-friend"
	h := setup(t, g)
	for name, err := range map[string]error{
		"begin":   h.uc.Begin(ctx, "stranger", "g1"),
		"act":     h.uc.Act(ctx, "stranger", "g1", "hi"),
		"fight":   h.uc.Fight(ctx, "stranger", "g1", rules.Option{}),
		"options": func() error { _, err := h.uc.Options(ctx, "stranger", "g1"); return err }(),
	} {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s by a stranger: %v", name, err)
		}
	}
	if err := h.uc.Act(ctx, "acct-friend", "g1", "hello"); !errors.Is(err, domain.ErrNotYourTurn) {
		t.Fatalf("an agent-played seat cannot act by hand: %v", err)
	}
	if err := h.uc.Act(ctx, me, "g1", "hello"); err != nil {
		t.Fatalf("the hero's owner acts: %v", err)
	}
	if dm := h.agent.seatTurns(domain.DMSeat); dm[0].Payer != me {
		t.Fatalf("DM billed to %q, want the leader", dm[0].Payer)
	}
	if comp := h.agent.seatTurns("pc2"); comp[0].Payer != "acct-friend" {
		t.Fatalf("companion billed to %q, want its owner", comp[0].Payer)
	}
}

func TestDMOffersQuickChoicesAndEndsItsTurn(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	narrate := say("The miller wrings his hands.")
	narrate.Message.ToolCalls = []domain.ToolCall{{ID: "oc1", Name: "offer_choices", Input: json.RawMessage(
		`{"choices":["  Ask about Elsie ","","` + strings.Repeat("x", 200) + `","Search the mill","Head for the hill","Buy a round","Sing a song"]}`)}}
	h.agent.on(domain.DMSeat, narrate)
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	want := []string{"Ask about Elsie", "Search the mill", "Head for the hill", "Buy a round"}
	if strings.Join(g.Choices, "|") != strings.Join(want, "|") {
		t.Fatalf("choices %q", g.Choices)
	}
	if n := len(h.agent.seatTurns(domain.DMSeat)); n != 1 {
		t.Fatalf("offer_choices must end the DM's turn without another call; got %d calls", n)
	}
	if err := h.uc.Act(ctx, me, "g1", "Ask about Elsie"); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); len(g.Choices) != 0 {
		t.Fatalf("old choices must clear when the DM takes its next turn: %q", g.Choices)
	}
	dm := h.agent.seatTurns(domain.DMSeat)
	last := dm[len(dm)-1].History
	if !strings.Contains(dm[0].System, "offer_choices") || last[len(last)-2].ToolResults[0].IsError {
		t.Fatal("tool result should be recorded before the next party message")
	}

	bad := say("The mill looms.")
	bad.Message.ToolCalls = []domain.ToolCall{{ID: "oc2", Name: "offer_choices", Input: json.RawMessage(`{"choices":["  "]}`)}}
	h2 := setup(t, newGame(t))
	h2.agent.on(domain.DMSeat, bad, say("Fine."))
	if err := h2.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if g := h2.store.game(t); len(g.Choices) != 0 || len(h2.agent.seatTurns(domain.DMSeat)) != 2 {
		t.Fatal("empty choices are rejected and the DM carries on")
	}
}

func TestChoicesAreSuggestedWhenTheDMForgets(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.agent.on(domain.DMSeat, say("The miller wrings his hands."))
	h.agent.on("choices",
		call("set_scene", `{"scene_key":"forest","title":"x","mood":"y","visual":"z"}`),
		call("offer_choices", `{"choices":["Ask about Elsie","Search the mill"]}`))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if strings.Join(g.Choices, "|") != "Ask about Elsie|Search the mill" || g.Scene.Key == "forest" {
		t.Fatalf("fallback choices %q, scene %q (the helper may only offer choices)", g.Choices, g.Scene.Key)
	}
	if _, kept := g.Transcripts["choices"]; kept {
		t.Fatal("the helper's conversation is thrown away")
	}
	turns := h.agent.seatTurns("choices")
	if turns[0].OnText != nil || turns[0].Payer != me || !strings.Contains(turns[0].History[0].Text, "The miller wrings his hands.") {
		t.Fatal("the helper is quiet, billed to the leader and sees the latest narration")
	}
	if !strings.Contains(h.agent.seatTurns(domain.DMSeat)[0].History[0].Text, "offer_choices") {
		t.Fatal("every DM request should carry the choices reminder")
	}

	h2 := setup(t, newGame(t))
	h2.agent.errs["choices"] = errors.New("overloaded")
	if err := h2.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatalf("a failed suggestion must not fail the turn: %v", err)
	}
	g2 := newGame(t)
	g2.Creatures[0].Controller = domain.ControlAI
	h3 := setup(t, g2)
	if err := h3.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h3.agent.seatTurns("choices")[0].History[0].Text, "the party could do") {
		t.Fatal("with no live seat the helper suggests moves for the party")
	}
}

func TestChoicesNeedNarrationFirst(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	narrate := say("Rain hammers the mill wheel.")
	narrate.Message.ToolCalls = []domain.ToolCall{{ID: "oc9", Name: "offer_choices", Input: json.RawMessage(`{"choices":["Knock","Climb in"]}`)}}
	h.agent.on(domain.DMSeat, call("offer_choices", `{"choices":["Knock","Climb in"]}`), narrate)
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	dm := h.agent.seatTurns(domain.DMSeat)
	if len(dm) != 2 {
		t.Fatalf("the DM should be asked to narrate before offering choices; %d calls", len(dm))
	}
	hist := dm[1].History
	if r := hist[len(hist)-1].ToolResults[0]; !r.IsError || !strings.Contains(r.Content, "never see your thinking") {
		t.Fatalf("expected a nudge to narrate, got %+v", r)
	}
	if g := h.store.game(t); strings.Join(g.Choices, "|") != "Knock|Climb in" || !h.pub.has(domain.EvNarration, "Rain hammers") {
		t.Fatal("after narrating, the choices are shown")
	}
}

func TestStartCombatRefusesDeadlyFights(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	for _, c := range g.Creatures {
		c.Level = 3 // two level 3 heroes: a hard fight is at most 450 adjusted XP
	}
	h := setup(t, g)
	h.cat.monster = func(key, id string) (*domain.Creature, error) {
		m, _ := dummy(7, 50)(key, id)
		if key == "owlbear" {
			m.CR = "3"
		}
		return m, nil
	}
	h.agent.on(domain.DMSeat,
		call("start_combat", `{"monsters":[{"key":"owlbear","count":1,"name":"Necromancer"}]}`),
		call("start_combat", `{"monsters":[{"key":"goblin","count":4}]}`),
		call("start_combat", `{"monsters":[{"key":"goblin","count":2},{"key":"wolf","count":1}]}`),
		say("Two shapes lunge from the dark."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	dm := h.agent.seatTurns(domain.DMSeat)
	if boss := dm[1].History[len(dm[1].History)-1].ToolResults[0]; !strings.Contains(boss.Content, "Necromancer (CR 3) is too dangerous at this level; the toughest monster allowed is CR 2") {
		t.Fatalf("a renamed big monster should be refused: %+v", boss)
	}
	refused := dm[2].History[len(dm[2].History)-1].ToolResults[0]
	if !refused.IsError || !strings.Contains(refused.Content, "too deadly for this party (500 adjusted XP; a hard fight is at most 450)") {
		t.Fatalf("four goblins should be refused: %+v", refused)
	}
	g = h.store.game(t)
	ids := []string{}
	for _, m := range g.Monsters() {
		ids = append(ids, m.ID)
	}
	if g.Mode != domain.ModeCombat || strings.Join(ids, ",") != "goblin-1,goblin-2,wolf-1" {
		t.Fatalf("a fair fight starts with only its own monsters: mode=%s monsters=%v", g.Mode, ids)
	}
}

func TestCompanionsKnowHowTheirClassFights(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	odd := build(t, "pc3", "Zed", "Fighter", "Human", domain.ControlAI, nil)
	odd.Class = "Pilot" // no template: falls back to general advice
	g.Creatures, g.Party = append(g.Creatures, odd), append(g.Party, "pc3")
	h := setup(t, g)
	if err := h.uc.Act(ctx, me, "g1", "Onward."); err != nil {
		t.Fatal(err)
	}
	cleric := h.agent.seatTurns("pc2")[0].System
	if !strings.Contains(cleric, "## How you fight") || !strings.Contains(cleric, "Healing Word at range") {
		t.Fatalf("the cleric should know its role: %s", cleric)
	}
	if zed := h.agent.seatTurns("pc3")[0].System; !strings.Contains(zed, "fragile ones stay back") {
		t.Fatal("unknown classes get general advice")
	}
}

func TestAgentsRunUnderTheirRoleDefinitions(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	dm := h.agent.seatTurns(domain.DMSeat)[0]
	if dm.Role != domain.RoleDM || dm.Model != "claude-opus-5-5" || dm.Effort != "medium" || len(dm.Tools) != 16 ||
		!strings.Contains(dm.System, `titled "Test"`) || !strings.Contains(dm.System, "- pc2: Mae, Dwarf Cleric. AI companion") {
		t.Fatalf("DM turn from dm.md: role %s model %s effort %s tools %d\n%s", dm.Role, dm.Model, dm.Effort, len(dm.Tools), dm.System)
	}
	choices := h.agent.seatTurns("choices")[0]
	if choices.Role != domain.RoleChoices || len(choices.Tools) != 1 || !strings.HasPrefix(choices.System, "You suggest next moves") {
		t.Fatalf("choices turn from choices.md: %+v", choices.Role)
	}
	if err := h.uc.Act(ctx, me, "g1", "I wave."); err != nil {
		t.Fatal(err)
	}
	mae := h.agent.seatTurns("pc2")[0]
	if mae.Role != domain.RoleCharacter || mae.Effort != "low" || !strings.Contains(mae.System, "You are Mae, a level 1 Dwarf Cleric") {
		t.Fatalf("character turn from character.md:\n%s", mae.System)
	}
	if g := h.store.game(t); g.Spend["pc2"].Model != "claude-opus-5-5" {
		t.Fatalf("the ledger records the model that answered: %+v", g.Spend["pc2"])
	}
}

func TestBrokenPromptTemplatesFailTheTurn(t *testing.T) {
	t.Parallel()
	for name, prompt := range map[string]string{"parse": "{{.Oops", "render": "{{.NoSuchField}}"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := setup(t, newGame(t))
			h.roles.prompts[domain.RoleDM] = prompt
			if err := h.uc.Begin(ctx, me, "g1"); err == nil || !strings.Contains(err.Error(), "dm prompt") {
				t.Fatalf("got %v", err)
			}
			if len(h.agent.turns) != 0 {
				t.Fatal("no model call with a broken prompt")
			}
		})
	}
}

func TestNPCsComeAndGo(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	long := strings.Repeat("x", 300)
	h.agent.on(domain.DMSeat,
		call("show_npc", `{"name":"Snik","type":"kobold","role":"kobold prisoner turned guide","attitude":"wary","gender":"m","look":"A scrawny kobold in chains.","follows":true}`),
		call("show_npc", `{"name":"Mara","type":"innkeeper","role":"`+long+`","attitude":"friendly","gender":"f"}`),
		call("show_npc", `{"name":"Bad","type":"innkeeper","role":"x","attitude":"smug","gender":"m"}`),
		call("show_npc", `{"name":"Genderless","type":"innkeeper","role":"x","attitude":"neutral"}`),
		call("show_npc", `{"name":"Odd","type":"dragon-god","role":"x","attitude":"neutral","gender":"m"}`),
		call("show_npc", `{"name":"  ","type":"guard","role":"x","attitude":"neutral"}`),
		call("show_npc", `{"name":"snik","type":"kobold","role":"kobold guide","attitude":"friendly","gender":"m"}`),
		call("npc_leaves", `{"name":"Nobody"}`),
		say("The kobold blinks at you."),
	)
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if len(g.NPCs) != 2 {
		t.Fatalf("only valid NPCs join the table: %+v", g.NPCs)
	}
	snik, mara := g.FindNPC("SNIK"), g.FindNPC("Mara")
	if !snik.Monster || !snik.Follows || snik.Attitude != "friendly" || snik.Role != "kobold guide" || snik.Look != "A scrawny kobold in chains." {
		t.Fatalf("an update keeps the look and follow flag: %+v", snik)
	}
	if snik.Gender != domain.Male || mara.Gender != domain.Female {
		t.Fatalf("genders: %q %q", snik.Gender, mara.Gender)
	}
	if mara.Monster || len(mara.Role) != maxNPCTextForTest || !mara.Here {
		t.Fatalf("role is capped: %d", len(mara.Role))
	}
	if !h.pub.has(domain.EvSystem, "Snik (kobold prisoner turned guide) is here.") || !h.art.has("games/g1/npc-snik") {
		t.Fatal("arrivals are announced and portraits requested")
	}
	h.agent.on(domain.DMSeat,
		call("set_scene", `{"scene_key":"road","title":"The Old Road","mood":"grey","visual":"mud"}`),
		call("npc_leaves", `{"name":"Snik"}`),
		say("Snik slips away."),
	)
	if err := h.uc.Act(ctx, me, "g1", "We leave with Snik."); err != nil {
		t.Fatal(err)
	}
	if msg := lastDMMessage(t, h); !strings.Contains(msg, "NPCs here:\n- Snik (kobold, friendly): kobold guide; travels with the party") {
		t.Fatalf("the DM sees who is here: %s", msg)
	}
	g = h.store.game(t)
	if g.FindNPC("Mara").Here || g.FindNPC("Snik").Here || g.FindNPC("Snik").Follows || !h.pub.has(domain.EvSystem, "Snik leaves.") {
		t.Fatal("a new scene leaves Mara behind; npc_leaves takes Snik off the table")
	}
	if len(g.NPCsHere()) != 0 {
		t.Fatal("nobody is here now")
	}
}

// maxNPCTextForTest mirrors play.maxNPCText (black-box tests cannot see it).
const maxNPCTextForTest = 200

func TestCastCheckShowsNPCsTheDMForgot(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.agent.on(domain.DMSeat, say("A hooded elf named Ilvra watches you from the corner."))
	h.agent.on("cast",
		call("start_combat", `{"monsters":[{"key":"goblin","count":1}]}`),
		call("show_npc", `{"name":"Ilvra","type":"commoner","role":"hooded watcher in the corner","attitude":"wary","gender":"f"}`),
		say("Done."),
	)
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if n := g.FindNPC("Ilvra"); n == nil || !n.Here || g.Mode != domain.ModeExploration {
		t.Fatalf("the cast check puts the forgotten NPC on the table, and nothing else: %+v mode %s", g.NPCs, g.Mode)
	}
	cast := h.agent.seatTurns("cast")[0]
	if cast.Role != domain.RoleCast || len(cast.Tools) != 2 || !strings.Contains(cast.History[0].Text, "Player characters (not NPCs): Aria, Mae") ||
		!strings.Contains(cast.History[0].Text, "On the table now: nobody") || !strings.Contains(cast.History[0].Text, "Ilvra watches you") {
		t.Fatalf("cast check request: %+v", cast.History[0].Text)
	}
	if _, kept := g.Transcripts["cast"]; kept || g.Spend["cast"].Calls != 3 {
		t.Fatal("the cast seat is throwaway but still ledgered")
	}
	if !strings.Contains(lastDMMessage(t, h), "Call show_npc for anyone new in the scene") {
		t.Fatal("the DM is reminded every turn")
	}
	h.agent.errs["cast"] = errors.New("overloaded")
	if err := h.uc.Act(ctx, me, "g1", "I wave at Ilvra."); err != nil {
		t.Fatalf("a failed cast check never fails the turn: %v", err)
	}
	casts := h.agent.seatTurns("cast")
	if latest := casts[len(casts)-1].History[0].Text; !strings.Contains(latest, "- Ilvra (commoner, wary)") {
		t.Fatalf("the check knows who is already on the table: %q", latest)
	}
	h.agent.errs["cast"] = nil
	before := len(h.agent.seatTurns("cast"))
	h.agent.on(domain.DMSeat, call("end_adventure", `{"epilogue":"The end."}`), say("Farewell."))
	if err := h.uc.Act(ctx, me, "g1", "We retire."); err != nil {
		t.Fatal(err)
	}
	if len(h.agent.seatTurns("cast")) != before {
		t.Fatal("no cast check once the adventure is over")
	}
}

func TestChoicesOnlyFromWhatThePartyCanDo(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	mae, _ := g.Creature("pc2")
	mae.SlotsUsed = map[int]int{1: 1}
	mae.Inventory = []string{"holy symbol", "rope"}
	h := setup(t, g)
	h.agent.on(domain.DMSeat, say("The goblins are asleep. What do you do?")) // no offer_choices: the helper steps in
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	dm := h.agent.seatTurns(domain.DMSeat)[0]
	status := dm.History[0].Text
	if !strings.Contains(status, "- pc2 Mae: L1 Cleric") || !strings.Contains(status, "Cure Wounds (level 1 spell)") ||
		!strings.Contains(status, "Sacred Flame (cantrip)") || !strings.Contains(status, "spell slots left: L1 1/2") ||
		!strings.Contains(status, "Carries: Holy Symbol, Rope (50 ft)") || strings.Contains(status, "Dodge") {
		t.Fatalf("the DM sees what each member can do: %s", status)
	}
	var offer string
	for _, tool := range dm.Tools {
		if tool.Name == "offer_choices" {
			offer = tool.Description
		}
	}
	if !strings.Contains(offer, "never invent spells, items or powers") || !strings.Contains(dm.System, "no \"sleep charm\"") {
		t.Fatal("the DM is told to offer only achievable actions")
	}
	helper := h.agent.seatTurns("choices")[0].History[0].Text
	if !strings.Contains(helper, "Cure Wounds (level 1 spell)") || !strings.Contains(helper, "Only suggest what the party can really do") {
		t.Fatalf("the choice helper gets the same list: %s", helper)
	}
}
