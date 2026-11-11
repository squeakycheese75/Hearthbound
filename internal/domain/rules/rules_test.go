package rules_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"strings"
	"testing"
)

// script is a Roller that returns pre-set values in order.
type script struct {
	vals []int
	i    int
}

func (s *script) Roll(sides int) int {
	if s.i >= len(s.vals) {
		return 1
	}
	v := s.vals[s.i]
	s.i++
	return min(v, sides)
}

func dice(vals ...int) *script { return &script{vals: vals} }

func fighter() *domain.Creature {
	return &domain.Creature{
		ID: "f", Name: "Fighter", Kind: domain.KindPC, Level: 1, MaxHP: 12, HP: 12, AC: 18, Speed: 30,
		Abilities:  domain.AbilityScores{domain.STR: 16, domain.DEX: 12, domain.CON: 14},
		SaveProfs:  []domain.Ability{domain.STR, domain.CON},
		SkillProfs: []domain.Skill{domain.Athletics},
		Actions: []domain.Action{
			{ID: "longsword", Name: "Longsword", Kind: domain.ActAttack, ToHit: 5, Dice: domain.MustDice("1d8+3"), RangeFt: 5},
			{ID: "crossbow", Name: "Crossbow", Kind: domain.ActAttack, ToHit: 3, Dice: domain.MustDice("1d8+1"), RangeFt: 80},
		},
	}
}

func goblin() *domain.Creature {
	return &domain.Creature{
		ID: "g", Name: "Goblin", Kind: domain.KindMonster, Level: 1, MaxHP: 7, HP: 7, AC: 15, Speed: 30, XPValue: 50,
		Pos:       domain.Position{X: 1},
		Abilities: domain.AbilityScores{domain.DEX: 14},
		Actions:   []domain.Action{{ID: "scimitar", Name: "Scimitar", Kind: domain.ActAttack, ToHit: 4, Dice: domain.MustDice("1d6+2"), RangeFt: 5}},
	}
}

func TestModifier(t *testing.T) {
	t.Parallel()
	cases := map[int]int{1: -5, 8: -1, 9: -1, 10: 0, 11: 0, 12: 1, 15: 2, 20: 5}
	for score, want := range cases {
		if got := domain.Modifier(score); got != want {
			t.Errorf("Modifier(%d) = %d, want %d", score, got, want)
		}
	}
}

func TestAttack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rolls   []int
		hit     bool
		crit    bool
		goblinH int
	}{
		{"hit on 10 (+5 vs AC15)", []int{10, 4}, true, false, 0},
		{"miss on 9", []int{9}, false, false, 7},
		{"natural 1 always misses", []int{1}, false, false, 7},
		{"natural 20 crits and doubles dice", []int{20, 1, 1}, true, true, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, g := fighter(), goblin()
			g.HP, g.MaxHP = 7, 7
			act, _ := f.Action("longsword")
			out, err := rules.Resolve(dice(tc.rolls...), f, act, g)
			if err != nil {
				t.Fatal(err)
			}
			if out.Hit != tc.hit || out.Rolls[0].Critical != tc.crit {
				t.Fatalf("hit=%v crit=%v, want %v %v", out.Hit, out.Rolls[0].Critical, tc.hit, tc.crit)
			}
			if g.HP != tc.goblinH {
				t.Fatalf("goblin HP %d, want %d", g.HP, tc.goblinH)
			}
		})
	}
}

func TestMonsterDiesAtZero(t *testing.T) {
	t.Parallel()
	f, g := fighter(), goblin()
	act, _ := f.Action("longsword")
	if _, err := rules.Resolve(dice(15, 8), f, act, g); err != nil {
		t.Fatal(err)
	}
	if !g.Dead {
		t.Fatal("goblin should be dead")
	}
}

func TestAdvantage(t *testing.T) {
	t.Parallel()
	f, g := fighter(), goblin()
	melee, _ := f.Action("longsword")
	bow, _ := f.Action("crossbow")
	if got := rules.AttackAdvantage(f, g, bow); got != rules.Disadv {
		t.Errorf("ranged adjacent = %v, want disadvantage", got)
	}
	g.AddCondition(domain.Prone)
	if got := rules.AttackAdvantage(f, g, melee); got != rules.Adv {
		t.Errorf("melee vs prone = %v, want advantage", got)
	}
	f.AddCondition(domain.Poisoned)
	if got := rules.AttackAdvantage(f, g, melee); got != rules.Normal {
		t.Errorf("poisoned vs prone = %v, want normal", got)
	}
}

func TestChecksAndSaves(t *testing.T) {
	t.Parallel()
	f := fighter()
	r := rules.AbilityCheck(dice(10), f, "", domain.Athletics, 15, rules.Normal)
	if r.Total != 15 || !*r.Success {
		t.Fatalf("athletics total %d success %v, want 15 true", r.Total, *r.Success)
	}
	r = rules.AbilityCheck(dice(3, 17), f, domain.DEX, "", 15, rules.Adv)
	if r.Kept != 17 || r.Total != 18 {
		t.Fatalf("advantage kept %d total %d", r.Kept, r.Total)
	}
	s := rules.SavingThrow(dice(10, 15), f, domain.CON, 14, rules.Disadv)
	if s.Total != 10+2+2 || !*s.Success {
		t.Fatalf("CON save total %d", s.Total)
	}
	f.AddCondition(domain.Unconscious)
	s = rules.SavingThrow(dice(20), f, domain.DEX, 5, rules.Normal)
	if *s.Success {
		t.Fatal("unconscious creature must fail DEX saves")
	}
}

func TestDyingAndDeathSaves(t *testing.T) {
	t.Parallel()
	f := fighter()
	rules.ApplyDamage(f, 14, false)
	if f.HP != 0 || !f.Has(domain.Unconscious) || !rules.Dying(f) {
		t.Fatalf("expected dying, got hp=%d conds=%v", f.HP, f.Conditions)
	}
	rules.DeathSave(dice(12), f)
	rules.DeathSave(dice(1), f)
	if f.DeathSaves.Successes != 1 || f.DeathSaves.Failures != 2 {
		t.Fatalf("death saves %+v", f.DeathSaves)
	}
	rules.ApplyDamage(f, 3, false)
	if !f.Dead {
		t.Fatal("third failure should kill")
	}
}

func TestNat20DeathSaveRevives(t *testing.T) {
	t.Parallel()
	f := fighter()
	rules.ApplyDamage(f, 12, false)
	rules.DeathSave(dice(20), f)
	if f.HP != 1 || f.Has(domain.Unconscious) {
		t.Fatalf("nat 20 should revive at 1 HP, hp=%d", f.HP)
	}
}

func TestMassiveDamage(t *testing.T) {
	t.Parallel()
	f := fighter()
	rules.ApplyDamage(f, 24, false)
	if !f.Dead {
		t.Fatal("damage >= HP + max HP kills outright")
	}
}

func TestSaveSpellAndHeal(t *testing.T) {
	t.Parallel()
	wiz := &domain.Creature{ID: "w", Name: "Wiz", Kind: domain.KindPC, Level: 1, HP: 6, MaxHP: 6, SpellSlots: map[int]int{1: 1}}
	burn := domain.Action{ID: "bh", Name: "Burning Hands", Kind: domain.ActSave, SaveDC: 13, SaveAbility: domain.DEX, HalfOnSave: true, Dice: domain.MustDice("3d6"), RangeFt: 15, SlotLevel: 1}
	g := goblin()
	g.HP, g.MaxHP = 30, 30
	out, err := rules.Resolve(dice(19, 6, 6, 6), wiz, burn, g)
	if err != nil {
		t.Fatal(err)
	}
	if out.Amount != 9 || g.HP != 21 {
		t.Fatalf("half damage: amount %d hp %d", out.Amount, g.HP)
	}
	if _, err := rules.Resolve(dice(1), wiz, burn, g); err == nil {
		t.Fatal("expected out-of-slots error")
	}
	f := fighter()
	f.HP = 2
	heal := domain.Action{ID: "cw", Name: "Cure", Kind: domain.ActHeal, Dice: domain.MustDice("1d8+3")}
	if _, err := rules.Resolve(dice(8), wiz, heal, f); err != nil {
		t.Fatal(err)
	}
	if f.HP != 12 {
		t.Fatalf("heal capped at max, hp %d", f.HP)
	}
}

func TestEncounterFlow(t *testing.T) {
	t.Parallel()
	f, g := fighter(), goblin()
	f.Controller = domain.ControlHuman
	game := &domain.Game{Creatures: []*domain.Creature{f, g}, Party: []string{"f"}}
	rules.StartEncounter(dice(15, 5), game)
	if game.Encounter.Current() != "f" || game.Mode != domain.ModeCombat {
		t.Fatalf("fighter should go first, order %v", game.Encounter.Order)
	}
	opts := rules.Options(game, "f")
	var swing rules.Option
	for _, o := range opts {
		if o.ActionID == "longsword" && o.TargetID == "g" {
			swing = o
		}
	}
	if swing.ActionID == "" {
		t.Fatalf("longsword option missing: %+v", opts)
	}
	if _, err := rules.TakeTurn(dice(1), game, "g", swing); err == nil {
		t.Fatal("expected not-your-turn error")
	}
	out, err := rules.TakeTurn(dice(18, 8), game, "f", swing)
	if err != nil {
		t.Fatal(err)
	}
	if f.Pos.DistanceFt(g.Pos) > 5 || !out.Hit {
		t.Fatalf("fighter should close in and hit; dist=%d", f.Pos.DistanceFt(g.Pos))
	}
	if rules.CombatResult(game) != rules.Victory {
		t.Fatal("expected victory")
	}
	if xp := rules.EndEncounter(game); xp != 50 || len(game.Creatures) != 1 || game.Encounter != nil {
		t.Fatalf("xp %d creatures %d", xp, len(game.Creatures))
	}
}

func TestMonsterChoiceAndAdvance(t *testing.T) {
	t.Parallel()
	f, g := fighter(), goblin()
	game := &domain.Game{Creatures: []*domain.Creature{f, g}, Party: []string{"f"}}
	rules.StartEncounter(dice(2, 19), game)
	if game.Encounter.Current() != "g" {
		t.Fatal("goblin should go first")
	}
	opt, ok := rules.MonsterChoice(dice(), game, "g")
	if !ok || opt.TargetID != "f" || opt.ActionID != "scimitar" {
		t.Fatalf("monster choice %+v", opt)
	}
	if _, err := rules.TakeTurn(dice(2), game, "g", opt); err != nil {
		t.Fatal(err)
	}
	rules.Advance(dice(), game)
	if game.Encounter.Current() != "f" {
		t.Fatal("advance to fighter")
	}
	rules.Advance(dice(), game)
	if game.Encounter.Round != 2 {
		t.Fatalf("round %d, want 2", game.Encounter.Round)
	}
}

func TestLevelForXP(t *testing.T) {
	t.Parallel()
	cases := map[int]int{0: 1, 299: 1, 300: 2, 900: 3, 2700: 4, 6499: 4, 6500: 5, 50000: 5}
	for xp, want := range cases {
		if got := rules.LevelForXP(xp); got != want {
			t.Errorf("LevelForXP(%d)=%d want %d", xp, got, want)
		}
	}
}

func TestParseDice(t *testing.T) {
	t.Parallel()
	good := map[string]string{"1d8": "1d8", "2d6+3": "2d6+3", "d20-1": "1d20-1", "4": "4"}
	for in, want := range good {
		d, err := domain.ParseDice(in)
		if err != nil || d.String() != want {
			t.Errorf("ParseDice(%q) = %v, %v", in, d, err)
		}
	}
	for _, bad := range []string{"", "xd6", "1d1", "200d6"} {
		if _, err := domain.ParseDice(bad); err == nil {
			t.Errorf("ParseDice(%q) should fail", bad)
		}
	}
}

func TestComeRound(t *testing.T) {
	t.Parallel()
	down := &domain.Creature{Name: "Pip", HP: 0, MaxHP: 8, Stable: true, Conditions: []domain.Condition{domain.Unconscious}}
	if notes := rules.ComeRound(down); len(notes) != 1 || down.HP != 1 || down.Has(domain.Unconscious) || down.Stable {
		t.Fatalf("a downed hero wakes with 1 HP: %+v %v", down, notes)
	}
	for _, c := range []*domain.Creature{{Name: "Up", HP: 5}, {Name: "Gone", Dead: true}} {
		if notes := rules.ComeRound(c); notes != nil {
			t.Fatalf("%s should not change: %v", c.Name, notes)
		}
	}
}

func TestEncounterBudget(t *testing.T) {
	t.Parallel()
	hero := func(level int) *domain.Creature { return &domain.Creature{Level: level} }
	goblins := func(n int) []*domain.Creature {
		out := make([]*domain.Creature, n)
		for i := range out {
			out[i] = &domain.Creature{XPValue: 50}
		}
		return out
	}
	four := []*domain.Creature{hero(1), hero(1), hero(1), hero(1)}
	if got := rules.HardBudget(append(four, &domain.Creature{Level: 1, Dead: true})); got != 300 {
		t.Fatalf("four level 1 heroes: %d, want 300 (the dead don't count)", got)
	}
	if got := rules.HardBudget([]*domain.Creature{hero(3), hero(9)}); got != 225+675 {
		t.Fatalf("levels 3 and 9: %d", got)
	}
	tests := []struct {
		name  string
		count int
		party int
		want  int
	}{
		{"one goblin", 1, 4, 50},
		{"two goblins", 2, 4, 150},
		{"three goblins", 3, 4, 300},
		{"four goblins: deadly for four level 1 heroes", 4, 4, 400},
		{"small party pays one step more", 1, 2, 75},
		{"big party pays one step less", 3, 6, 225},
		{"a horde", 20, 4, 4000},
		{"a horde against a big party", 8, 6, 800},
		{"eleven", 11, 4, 1650},
		{"no step below x1", 1, 6, 50},
	}
	for _, tc := range tests {
		if got := rules.AdjustedXP(goblins(tc.count), tc.party); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestMaxCR(t *testing.T) {
	t.Parallel()
	for cr, want := range map[string]float64{"1/8": 0.125, "1/4": 0.25, "1/2": 0.5, "2": 2, "0": 0, "x": 0} {
		if got := rules.CRValue(cr); got != want {
			t.Errorf("CR %q = %v, want %v", cr, got, want)
		}
	}
	at := func(levels ...int) []*domain.Creature {
		out := []*domain.Creature{{Level: 9, Dead: true}}
		for _, l := range levels {
			out = append(out, &domain.Creature{Level: l})
		}
		return out
	}
	for _, tc := range []struct {
		party []*domain.Creature
		want  float64
	}{{at(1, 1), 0.5}, {at(2, 1), 1}, {at(3), 2}} {
		if got := rules.MaxCR(tc.party); got != tc.want {
			t.Errorf("max CR %v, want %v", got, tc.want)
		}
	}
}

func TestOptionsSayHowTheyFight(t *testing.T) {
	t.Parallel()
	f, g := fighter(), goblin()
	f.Actions = append(f.Actions,
		domain.Action{ID: "bow", Name: "Bow", Kind: domain.ActAttack, Dice: domain.MustDice("1d6"), RangeFt: 80},
		domain.Action{ID: "far", Name: "Far Bow", Kind: domain.ActAttack, Dice: domain.MustDice("1d6"), RangeFt: 10},
		domain.Action{ID: "touch", Name: "Touch Heal", Kind: domain.ActHeal, Dice: domain.MustDice("1d4"), RangeFt: 5},
		domain.Action{ID: "word", Name: "Word Heal", Kind: domain.ActHeal, Dice: domain.MustDice("1d4"), RangeFt: 60},
	)
	ally := fighter()
	ally.ID, ally.Name, ally.HP = "a", "Ally", 1
	game := &domain.Game{Creatures: []*domain.Creature{f, ally, g}, Party: []string{"f", "a"}}
	rules.StartEncounter(dice(15, 5, 5), game)
	f.Pos, ally.Pos, g.Pos = domain.Position{X: 0, Y: 0}, domain.Position{X: 0, Y: 4}, domain.Position{X: 6, Y: 0}
	got := map[string]string{}
	for _, o := range rules.Options(game, "f") {
		got[o.ActionID] = o.Summary
	}
	want := map[string]string{
		"longsword":      "melee: moves up to the foe",
		"bow":            "ranged 80 ft)",
		"far":            "ranged 10 ft: moves into range",
		"touch":          "moves to reach them",
		"word":           "in reach",
		rules.ApproachID: "no attack this turn",
	}
	for id, part := range want {
		if !strings.Contains(got[id], part) {
			t.Errorf("%s: %q should mention %q", id, got[id], part)
		}
	}
	g.Pos = domain.Position{X: 1, Y: 0}
	for _, o := range rules.Options(game, "f") {
		if o.ActionID == "longsword" && !strings.HasSuffix(o.Summary, "(melee)") {
			t.Errorf("adjacent swing: %q", o.Summary)
		}
	}
}

func TestMonstersFightInTheirOwnStyle(t *testing.T) {
	t.Parallel()
	// The fighter (AC 18, 12 HP) stands next to the monster; the wizard
	// (AC 11, 6 HP) is 25 ft away with another monster already on her.
	arena := func(tactic domain.Tactic) *domain.Game {
		m := goblin()
		m.ID, m.Tactic, m.Pos = "m", tactic, domain.Position{X: 5, Y: 0}
		m.Actions = []domain.Action{
			{ID: "club", Name: "Club", Kind: domain.ActAttack, Dice: domain.MustDice("1d6"), RangeFt: 5},
			{ID: "bow", Name: "Bow", Kind: domain.ActAttack, Dice: domain.MustDice("1d6"), RangeFt: 80},
		}
		f := fighter()
		f.Pos = domain.Position{X: 4, Y: 0}
		w := &domain.Creature{ID: "w", Name: "Wizard", Kind: domain.KindPC, Level: 1, MaxHP: 6, HP: 6, AC: 11, Speed: 30, Pos: domain.Position{X: 0, Y: 4}}
		ally := goblin()
		ally.ID, ally.Pos = "a", domain.Position{X: 0, Y: 5}
		return &domain.Game{Creatures: []*domain.Creature{f, w, m, ally}, Party: []string{"f", "w"}}
	}
	tests := []struct {
		tactic         domain.Tactic
		action, target string
	}{
		{domain.TacticBrute, "club", "f"},
		{domain.TacticSkirmisher, "bow", "w"},
		{domain.TacticPack, "", "w"},
		{domain.TacticCunning, "", "w"},
		{domain.TacticMindless, "", "f"},
		{"", "", "w"},
	}
	for _, tc := range tests {
		opt, ok := rules.MonsterChoice(dice(), arena(tc.tactic), "m")
		if !ok || opt.TargetID != tc.target || (tc.action != "" && opt.ActionID != tc.action) {
			t.Errorf("%q monster chose %s → %s, want %s → %s", tc.tactic, opt.ActionID, opt.TargetID, tc.action, tc.target)
		}
	}
}

func TestDimMonstersSometimesForgetTheirTactics(t *testing.T) {
	t.Parallel()
	setup := func(intel int, kind domain.Kind) *domain.Game {
		m := goblin()
		m.Kind, m.Tactic, m.Abilities = kind, domain.TacticMindless, domain.AbilityScores{domain.INT: intel}
		near, far := fighter(), fighter()
		near.ID, near.Pos = "near", domain.Position{X: 2}
		far.ID, far.Pos = "far", domain.Position{X: 5}
		m.Actions = append(m.Actions, domain.Action{ID: "bow", Name: "Bow", Kind: domain.ActAttack, Dice: domain.MustDice("1d6"), RangeFt: 80})
		return &domain.Game{Creatures: []*domain.Creature{m, near, far}, Party: []string{"near", "far"}}
	}
	tests := []struct {
		name   string
		intel  int
		kind   domain.Kind
		rolls  []int
		target string
	}{
		{"a wolf usually plays its style", 3, domain.KindMonster, []int{41}, "near"},
		{"a wolf sometimes lashes out at random", 3, domain.KindMonster, []int{40, 4}, "far"},
		{"an orc forgets a quarter of the time", 7, domain.KindMonster, []int{25, 4}, "far"},
		{"a goblin rarely forgets", 10, domain.KindMonster, []int{11}, "near"},
		{"a goblin can still slip up", 10, domain.KindMonster, []int{10, 4}, "far"},
		{"clever monsters never forget", 14, domain.KindMonster, []int{1, 4}, "near"},
		{"no Intelligence score: no roll", 0, domain.KindMonster, []int{1, 4}, "near"},
		{"heroes are not erratic", 3, domain.KindPC, []int{1, 4}, "near"},
	}
	for _, tc := range tests {
		g := setup(tc.intel, tc.kind)
		if tc.kind == domain.KindPC {
			g.Creatures[1].Kind, g.Creatures[2].Kind = domain.KindMonster, domain.KindMonster
		}
		opt, _ := rules.MonsterChoice(dice(tc.rolls...), g, "g")
		if opt.TargetID != tc.target {
			t.Errorf("%s: chose %s → %s, want target %s", tc.name, opt.ActionID, opt.TargetID, tc.target)
		}
	}
	if opts := rules.Options(setup(3, domain.KindMonster), "g"); len(opts) < 4 {
		t.Fatalf("expected attack and move options: %+v", opts)
	}
}
