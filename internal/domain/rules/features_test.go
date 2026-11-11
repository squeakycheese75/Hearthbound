package rules_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"slices"
	"strings"
	"testing"
)

// arena is a fight with the creatures in turn order, first one to act.
func arena(cs ...*domain.Creature) *domain.Game {
	enc := &domain.Encounter{Round: 1, Initiative: map[string]int{}}
	for _, c := range cs {
		enc.Order = append(enc.Order, c.ID)
	}
	return &domain.Game{Creatures: cs, Mode: domain.ModeCombat, Encounter: enc}
}

// turnOf hands the turn to the creature at position i, as a fresh turn.
func turnOf(g *domain.Game, i int) {
	g.Encounter.Turn, g.Encounter.Spent = i, domain.TurnSpent{}
}

func feat(key domain.FeatureKey, uses int) domain.Feature {
	return domain.Feature{Key: key, Name: string(key), Max: uses, Recharge: domain.RechargeShort}
}

func hero(id string, feats ...domain.Feature) *domain.Creature {
	c := fighter()
	c.ID, c.Name, c.Features = id, strings.ToUpper(id[:1])+id[1:], feats
	c.Actions[0].Weapon, c.Actions[0].Stat = true, domain.STR
	c.Actions[1].Weapon, c.Actions[1].Stat = true, domain.DEX
	return c
}

func has(opts []rules.Option, id string) (rules.Option, bool) {
	i := slices.IndexFunc(opts, func(o rules.Option) bool { return o.ActionID == id })
	if i < 0 {
		return rules.Option{}, false
	}
	return opts[i], true
}

func labels(out rules.Outcome) string {
	var b strings.Builder
	for _, r := range out.Rolls {
		b.WriteString(r.Label + "|")
	}
	return b.String()
}

func TestBonusActionsKeepTheTurnOpenAndEndTurnCloses(t *testing.T) {
	t.Parallel()
	f := hero("fighter", feat(domain.FeatSecondWind, 1), feat(domain.FeatActionSurge, 1))
	f.HP = 4
	gob := goblin()
	g := arena(f, gob)
	opts := rules.Options(g, f.ID)
	wind, ok := has(opts, rules.SecondWindID)
	if !ok || wind.Cost != rules.CostBonus {
		t.Fatalf("second wind on offer: %+v", opts)
	}
	out, err := rules.TakeTurn(dice(6), g, f.ID, wind)
	if err != nil || out.TurnOver || f.HP != 4+6+1 || !g.Encounter.Spent.Bonus {
		t.Fatalf("second wind: %+v %v HP %d", out, err, f.HP)
	}
	if _, ok := has(rules.Options(g, f.ID), rules.SecondWindID); ok {
		t.Fatal("one bonus action a turn")
	}
	surge, _ := has(rules.Options(g, f.ID), rules.ActionSurgeID)
	if out, _ := rules.TakeTurn(dice(), g, f.ID, surge); out.TurnOver || g.Encounter.Spent.ActionsLeft() != 2 {
		t.Fatalf("action surge: %+v", g.Encounter.Spent)
	}
	attack := rules.Option{ActionID: "longsword", TargetID: gob.ID}
	if out, _ := rules.TakeTurn(dice(2, 1), g, f.ID, attack); out.TurnOver {
		t.Fatal("one action still left after surging")
	}
	out, _ = rules.TakeTurn(dice(2, 1), g, f.ID, attack)
	if !out.TurnOver || rules.Options(g, f.ID) != nil {
		t.Fatalf("nothing left: the turn is over: %+v", rules.Options(g, f.ID))
	}

	// A rogue who has attacked still has Cunning Action; End turn closes the turn.
	rogue := hero("rogue", feat(domain.FeatCunningAction, 0))
	g = arena(rogue, goblin())
	out, _ = rules.TakeTurn(dice(2, 1), g, rogue.ID, rules.Option{ActionID: "longsword", TargetID: "g"})
	end, ok := has(rules.Options(g, rogue.ID), rules.EndTurnID)
	if out.TurnOver || !ok {
		t.Fatal("bonus actions left: the turn stays open with an End turn option")
	}
	if out, _ := rules.TakeTurn(dice(), g, rogue.ID, end); !out.TurnOver {
		t.Fatal("end turn")
	}
	rules.Advance(dice(), g)
	if sp := g.Encounter.Spent; sp.Actions != 0 || sp.Bonus || len(sp.Free) != 0 {
		t.Fatalf("a new turn starts fresh: %+v", g.Encounter.Spent)
	}
}

func TestRageRecklessAndFrenzy(t *testing.T) {
	t.Parallel()
	b := hero("barb", feat(domain.FeatRage, 2), feat(domain.FeatRecklessAttack, 0), feat(domain.FeatFrenzy, 0))
	gob := goblin()
	gob.MaxHP, gob.HP = 50, 50
	g := arena(b, gob)
	if _, ok := has(rules.Options(g, b.ID), rules.FrenzyRageID); !ok {
		t.Fatal("a berserker may rage in a frenzy")
	}
	frenzy, _ := has(rules.Options(g, b.ID), rules.FrenzyRageID)
	reckless, _ := has(rules.Options(g, b.ID), rules.RecklessID)
	for _, o := range []rules.Option{frenzy, reckless} {
		if _, err := rules.TakeTurn(dice(), g, b.ID, o); err != nil {
			t.Fatal(err)
		}
	}
	if !b.Has(domain.Raging) || !b.Has(domain.Frenzied) || b.FeatureLeft(domain.FeatRage) != 1 {
		t.Fatalf("raging: %v", b.Conditions)
	}
	if _, ok := has(rules.Options(g, b.ID), rules.FrenzyPrefix+"longsword"); ok {
		t.Fatal("the frenzied strike starts next turn")
	}
	if a := rules.AttackAdvantage(b, gob, b.Actions[0]); a != rules.Adv {
		t.Fatal("reckless: advantage on strength melee attacks")
	}
	if a := rules.AttackAdvantage(gob, b, gob.Actions[0]); a != rules.Adv {
		t.Fatal("reckless: foes have advantage too")
	}
	out, _ := rules.TakeTurn(dice(15, 15, 4), g, b.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID})
	if !strings.Contains(labels(out), "(rage +2)") || out.Amount != 4+3+2 {
		t.Fatalf("rage damage: %s %d", labels(out), out.Amount)
	}
	// Hit back: resistance halves slashing damage.
	notes := rules.ApplyDamage(b, 0, false)
	if notes != nil {
		t.Fatal("no damage, no note")
	}
	turnOf(g, 1)
	gob.Actions[0].DamageType = "slashing"
	hit, _ := rules.TakeTurn(dice(18, 18, 6), g, gob.ID, rules.Option{ActionID: "scimitar", TargetID: b.ID}) // advantage: two d20s
	if b.HP != 12-4 || !strings.Contains(strings.Join(hit.Notes, " "), "rage shrugs off") {
		t.Fatalf("resisted: HP %d %v", b.HP, hit.Notes)
	}
	turnOf(g, 1)
	rules.Advance(dice(), g) // back to the barbarian
	if b.Has(domain.Reckless) {
		t.Fatal("reckless lasts until the start of your next turn")
	}
	if _, ok := has(rules.Options(g, b.ID), rules.FrenzyPrefix+"longsword"); !ok {
		t.Fatal("a frenzied barbarian attacks as a bonus action on later turns")
	}
	rules.EndEncounter(g)
	if b.Has(domain.Raging) || !b.Has(domain.Exhausted) {
		t.Fatalf("after a frenzy: %v", b.Conditions)
	}
	roll := rules.AbilityCheck(dice(20, 2), b, domain.STR, "", 10, rules.Normal)
	if roll.Kept != 2 {
		t.Fatal("exhaustion: disadvantage on ability checks")
	}
}

func TestSneakAttackHideAndColossusSlayer(t *testing.T) {
	t.Parallel()
	rogue := hero("rogue", feat(domain.FeatSneakAttack, 0), feat(domain.FeatCunningAction, 0))
	rogue.Actions[0].Finesse = true
	rogue.Pos = domain.Position{X: 5}
	gob := goblin()
	gob.MaxHP, gob.HP = 60, 60
	buddy := hero("buddy")
	buddy.Pos = domain.Position{X: 2}
	g := arena(rogue, gob, buddy)
	out, _ := rules.TakeTurn(dice(15, 3, 5), g, rogue.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID})
	if !strings.Contains(labels(out), "Sneak Attack") || out.Amount != 3+3+5 {
		t.Fatalf("an ally beside the target: sneak attack: %s %d", labels(out), out.Amount)
	}
	g.Encounter.Spent.Extra = 1
	out, _ = rules.TakeTurn(dice(15, 3, 5), g, rogue.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID})
	if strings.Contains(labels(out), "Sneak Attack") {
		t.Fatal("once per turn")
	}

	// Alone, the rogue hides first; the hidden attack has advantage and sneaks.
	lone := hero("lone", feat(domain.FeatSneakAttack, 0), feat(domain.FeatCunningAction, 0))
	lone.Pos = domain.Position{X: 5}
	g = arena(lone, goblin())
	hide, _ := has(rules.Options(g, lone.ID), rules.HideID)
	if out, _ := rules.TakeTurn(dice(20), g, lone.ID, hide); !lone.Has(domain.Hidden) || out.TurnOver {
		t.Fatalf("hidden: %+v", out)
	}
	if _, ok := has(rules.Options(g, lone.ID), rules.DashID); ok {
		t.Fatal("one bonus action")
	}
	out, _ = rules.TakeTurn(dice(15, 2, 3, 4), g, lone.ID, rules.Option{ActionID: "crossbow", TargetID: "g"})
	if !strings.Contains(labels(out), "Sneak Attack") || lone.Has(domain.Hidden) {
		t.Fatalf("hidden attack: %s", labels(out))
	}
	failed := hero("clumsy", feat(domain.FeatCunningAction, 0))
	g = arena(failed, goblin())
	hide, _ = has(rules.Options(g, failed.ID), rules.HideID)
	if _, err := rules.TakeTurn(dice(1), g, failed.ID, hide); err != nil || failed.Has(domain.Hidden) {
		t.Fatal("a failed stealth check leaves you seen")
	}

	ranger := hero("ranger", feat(domain.FeatColossusSlayer, 0))
	gob = goblin()
	gob.MaxHP, gob.HP = 60, 59
	g = arena(ranger, gob)
	out, _ = rules.TakeTurn(dice(15, 3, 7), g, ranger.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID})
	if !strings.Contains(labels(out), "Colossus Slayer") || out.Amount != 3+3+7 {
		t.Fatalf("colossus slayer on a wounded foe: %s %d", labels(out), out.Amount)
	}
}

func TestDashMovesFarther(t *testing.T) {
	t.Parallel()
	rogue := hero("rogue", feat(domain.FeatCunningAction, 0))
	gob := goblin()
	gob.Pos = domain.Position{X: 11, Y: 7}
	g := arena(rogue, gob)
	dash, _ := has(rules.Options(g, rogue.ID), rules.DashID)
	_, _ = rules.TakeTurn(dice(), g, rogue.ID, dash)
	_, _ = rules.TakeTurn(dice(), g, rogue.ID, rules.Option{ActionID: rules.ApproachID, TargetID: gob.ID})
	if rogue.Pos.X < 10 {
		t.Fatalf("dashing doubles movement: at %+v", rogue.Pos)
	}
}

func TestBardsInspireAndCut(t *testing.T) {
	t.Parallel()
	bard := hero("bard", feat(domain.FeatBardicInspiration, 3), feat(domain.FeatCuttingWords, 0), feat(domain.FeatJackOfAllTrades, 0))
	ally := hero("ally")
	gob := goblin()
	g := arena(bard, ally, gob)
	inspire, ok := has(rules.Options(g, bard.ID), rules.InspireID)
	if !ok || inspire.TargetID != ally.ID {
		t.Fatalf("inspire the ally: %+v", rules.Options(g, bard.ID))
	}
	_, _ = rules.TakeTurn(dice(), g, bard.ID, inspire)
	if ally.Inspiration != 6 || bard.FeatureLeft(domain.FeatBardicInspiration) != 2 {
		t.Fatal("inspiration given")
	}
	// The ally's attack misses by 2 (8+5 vs AC 15): the die saves it.
	turnOf(g, 1)
	out, _ := rules.TakeTurn(dice(8, 4, 3), g, ally.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID})
	if !out.Hit || ally.Inspiration != 0 || !strings.Contains(labels(out), "inspiration 4") {
		t.Fatalf("inspired hit: %+v", out)
	}
	// The goblin's attack would hit the ally by 1 (14+4 vs 18): cutting words.
	turnOf(g, 2)
	out, _ = rules.TakeTurn(dice(14, 3), g, gob.ID, rules.Option{ActionID: "scimitar", TargetID: ally.ID})
	if out.Hit || !g.Encounter.Reacted[bard.ID] || bard.FeatureLeft(domain.FeatBardicInspiration) != 1 {
		t.Fatalf("cutting words: %+v", out)
	}
	turnOf(g, 2)
	out, _ = rules.TakeTurn(dice(14, 3), g, gob.ID, rules.Option{ActionID: "scimitar", TargetID: ally.ID})
	if !out.Hit {
		t.Fatal("one reaction a round")
	}
	if got := rules.CheckModifier(bard, domain.CHA, domain.Persuasion); got != 0+1 {
		t.Fatalf("jack of all trades: %d", got)
	}
	if got := rules.Initiative(dice(10), bard).Modifier; got != 1+1 {
		t.Fatalf("jack of all trades on initiative: %d", got)
	}
	save := rules.SavingThrow(dice(9, 5), func() *domain.Creature { c := hero("s"); c.Inspiration = 6; return c }(), domain.WIS, 10, rules.Normal)
	if !*save.Success {
		t.Fatal("inspiration rescues a save")
	}
}

func TestExpertiseAndDangerSense(t *testing.T) {
	t.Parallel()
	rogue := hero("rogue", domain.Feature{Key: domain.FeatExpertise, Skills: []domain.Skill{domain.Athletics}})
	if got := rules.CheckModifier(rogue, domain.STR, domain.Athletics); got != 3+4 {
		t.Fatalf("expertise doubles proficiency: %d", got)
	}
	barb := hero("barb", feat(domain.FeatDangerSense, 0))
	if roll := rules.SavingThrow(dice(3, 17), barb, domain.DEX, 10, rules.Normal); roll.Kept != 17 {
		t.Fatal("danger sense: advantage on DEX saves")
	}
}

func TestChannelDivinity(t *testing.T) {
	t.Parallel()
	cleric := hero("cleric", feat(domain.FeatChannelDivinity, 1), feat(domain.FeatDiscipleOfLife, 0))
	cleric.Level = 2
	hurt := hero("hurt")
	hurt.HP = 1
	fine := hero("fine")
	zombie := goblin()
	zombie.ID, zombie.Name, zombie.Type = "z", "Zombie", "undead"
	g := arena(cleric, hurt, fine, zombie)
	preserve, ok := has(rules.Options(g, cleric.ID), rules.PreserveLifeID)
	if !ok {
		t.Fatal("preserve life when an ally is badly hurt")
	}
	if _, ok := has(rules.Options(g, cleric.ID), rules.TurnUndeadID); !ok {
		t.Fatal("turn undead with undead near")
	}
	out, _ := rules.TakeTurn(dice(), g, cleric.ID, preserve)
	if hurt.HP != 6 || fine.HP != 12 || out.Amount != 5 || !out.TurnOver {
		t.Fatalf("preserve life heals up to half: hurt %d fine %d amount %d", hurt.HP, fine.HP, out.Amount)
	}

	cleric.Features[0].Used = 0
	g = arena(cleric, zombie)
	turn, _ := has(rules.Options(g, cleric.ID), rules.TurnUndeadID)
	out, _ = rules.TakeTurn(dice(2), g, cleric.ID, turn)
	if !zombie.Has(domain.Turned) || zombie.CanAct() || !out.Hit {
		t.Fatalf("turned: %+v", out)
	}
	rules.ApplyDamage(zombie, 1, false)
	if zombie.Has(domain.Turned) {
		t.Fatal("damage breaks a turning")
	}
	cleric.Features[0].Used = 0
	resist := goblin()
	resist.Type, resist.Abilities = "undead", domain.AbilityScores{domain.WIS: 20}
	g = arena(cleric, resist)
	turn, _ = has(rules.Options(g, cleric.ID), rules.TurnUndeadID)
	if out, _ := rules.TakeTurn(dice(20), g, cleric.ID, turn); out.Hit || resist.Has(domain.Turned) {
		t.Fatal("a successful save resists")
	}

	heal := domain.Action{ID: "cure", Name: "Cure Wounds", Kind: domain.ActHeal, Dice: domain.MustDice("1d8"), SlotLevel: 1}
	cleric.SpellSlots = map[int]int{1: 1}
	out, _ = rules.Resolve(dice(4), cleric, heal, hurt)
	if out.Amount != 4+3 {
		t.Fatalf("disciple of life: %d", out.Amount)
	}
}

func TestRestFeatures(t *testing.T) {
	t.Parallel()
	wiz := hero("wiz", domain.Feature{Key: domain.FeatArcaneRecovery, Max: 1, Recharge: domain.RechargeLong})
	wiz.Level, wiz.SlotsUsed = 3, map[int]int{1: 1, 2: 1}
	notes := rules.ArcaneRecovery(wiz)
	if len(notes) != 1 || wiz.SlotsUsed[2] != 0 || wiz.SlotsUsed[1] != 1 {
		t.Fatalf("recover the highest slot that fits (2 levels): %v %v", notes, wiz.SlotsUsed)
	}
	if rules.ArcaneRecovery(wiz) != nil {
		t.Fatal("once a day")
	}
	big := hero("big", domain.Feature{Key: domain.FeatArcaneRecovery, Max: 1})
	big.SlotsUsed = map[int]int{3: 1}
	if rules.ArcaneRecovery(big) != nil || big.FeatureLeft(domain.FeatArcaneRecovery) != 1 {
		t.Fatal("nothing small enough to recover keeps the use")
	}
	bard := hero("bard", feat(domain.FeatSongOfRest, 0))
	if rules.SongOfRest(dice(5), []*domain.Creature{hero("x"), bard}) != 5 || rules.SongOfRest(dice(5), []*domain.Creature{hero("x")}) != 0 {
		t.Fatal("song of rest")
	}
	wiz.RechargeFeatures(domain.RechargeShort)
	if wiz.FeatureLeft(domain.FeatArcaneRecovery) != 0 {
		t.Fatal("long-rest features wait for a long rest")
	}
	wiz.RestInTown()
	if wiz.FeatureLeft(domain.FeatArcaneRecovery) != 1 {
		t.Fatal("town restores everything")
	}
}

func TestImprovedCritical(t *testing.T) {
	t.Parallel()
	champ := hero("champ", feat(domain.FeatImprovedCritical, 0))
	out, _ := rules.TakeTurn(dice(19, 1, 1), arena(champ, goblin()), champ.ID, rules.Option{ActionID: "longsword", TargetID: "g"})
	if !out.Rolls[0].Critical {
		t.Fatal("a 19 is a critical hit for a champion")
	}
}

func TestAgentFeaturePlan(t *testing.T) {
	t.Parallel()
	b := hero("barb", feat(domain.FeatRage, 2), feat(domain.FeatRecklessAttack, 0))
	g := arena(b, goblin())
	if o, ok := rules.AgentFeature(g, b.ID, false); !ok || o.ActionID != rules.RageID {
		t.Fatalf("rage first: %+v", o)
	}
	_, _ = rules.TakeTurn(dice(), g, b.ID, rules.Option{ActionID: rules.RageID})
	if o, ok := rules.AgentFeature(g, b.ID, false); !ok || o.ActionID != rules.RecklessID {
		t.Fatalf("then attack recklessly while healthy: %+v", o)
	}
	if _, ok := rules.AgentFeature(g, "nobody", false); ok {
		t.Fatal("unknown creature")
	}
	rogue := hero("rogue", feat(domain.FeatCunningAction, 0))
	g = arena(rogue, goblin())
	if o, ok := rules.AgentFeature(g, rogue.ID, true); !ok || o.ActionID != rules.HideID {
		t.Fatalf("hide after attacking: %+v", o)
	}
}

func TestFeaturesFromTheClassAndLevelGains(t *testing.T) {
	t.Parallel()
	cls := domain.ClassTemplate{Features: []domain.FeatureTemplate{
		{Key: domain.FeatRage, Name: "Rage", Level: 1, Uses: map[int]int{1: 2, 3: 3}, Recharge: domain.RechargeLong},
		{Key: domain.FeatBardicInspiration, Name: "Inspiration", Level: 1, UsesAbility: domain.CHA},
		{Key: domain.FeatSneakAttack, Name: "Sneak Attack", Level: 1},
		{Key: domain.FeatExpertise, Name: "Expertise", Level: 1, Prefer: []domain.Skill{domain.Stealth, domain.Athletics}},
		{Key: domain.FeatActionSurge, Name: "Action Surge", Level: 2, Uses: map[int]int{2: 1}},
	}}
	c := hero("c")
	c.Features = nil
	c.SyncFeatures(cls)
	if len(c.Features) != 4 || c.Feature(domain.FeatRage).Max != 2 || c.Feature(domain.FeatBardicInspiration).Max != 1 {
		t.Fatalf("level 1 features: %+v", c.Features)
	}
	if !c.Expert(domain.Athletics) || c.Expert(domain.Stealth) {
		t.Fatal("expertise in proficient skills only")
	}
	_ = c.SpendFeature(domain.FeatRage)
	before := domain.Snapshot(c)
	c.Level, c.MaxHP, c.SpellSlots = 3, 20, map[int]int{1: 4, 2: 2}
	before.Slots[1] = 3
	c.Actions = append(c.Actions, domain.Action{ID: "shatter", Name: "Shatter", SlotLevel: 2})
	c.SyncFeatures(cls)
	if f := c.Feature(domain.FeatRage); f.Max != 3 || f.Used != 1 {
		t.Fatalf("uses grow and spent ones stay spent: %+v", f)
	}
	gains := strings.Join(domain.LevelGains(before, c), "\n")
	for _, want := range []string{
		"+8 maximum hit points", "New: Action Surge", "Rage: 3 uses (was 2)", "Sneak Attack: 2d6 (was 1d6)",
		"Level 1 spell slots: 4 (was 3)", "Level 2 spell slots unlocked (2)", "New spell: Shatter",
	} {
		if !strings.Contains(gains, want) {
			t.Errorf("missing %q in:\n%s", want, gains)
		}
	}
	if err := c.SpendFeature(domain.FeatSneakAttack); err == nil {
		t.Fatal("always-on features have no uses to spend")
	}
	if c.FeatureLeft("nothing") != 0 {
		t.Fatal("unknown features have no uses")
	}
}

func TestUpcastingAndAreas(t *testing.T) {
	t.Parallel()
	wiz := hero("wiz", feat(domain.FeatSculptSpells, 0))
	wiz.SpellSlots, wiz.SlotsUsed = map[int]int{1: 1, 3: 1}, map[int]int{1: 1}
	wiz.Actions = append(wiz.Actions,
		domain.Action{ID: "magic_missile", Name: "Magic Missile", Kind: domain.ActAutoHit, Dice: domain.MustDice("3d4+3"), RangeFt: 120, SlotLevel: 1, Upcast: domain.MustDice("1d4+1")},
		domain.Action{ID: "fireball", Name: "Fireball", Kind: domain.ActSave, Dice: domain.MustDice("8d6"), SaveAbility: domain.DEX, SaveDC: 15, HalfOnSave: true, RangeFt: 150, SlotLevel: 3, Radius: 20})
	gob, gob2 := goblin(), goblin()
	gob.MaxHP, gob.HP, gob.Pos = 99, 99, domain.Position{X: 8}
	gob2.ID, gob2.MaxHP, gob2.HP, gob2.Pos = "g2", 99, 99, domain.Position{X: 9}
	ally := hero("ally")
	ally.Pos = domain.Position{X: 7}
	g := arena(wiz, gob, gob2, ally)
	opts := rules.Options(g, wiz.ID)
	mm, _ := has(opts, "magic_missile")
	if !strings.Contains(mm.Summary, "[level 3 slot]") {
		t.Fatalf("level 1 slots spent: magic missile uses the level 3 slot: %s", mm.Summary)
	}
	fire, _ := has(opts, "fireball")
	if !strings.Contains(fire.Summary, "2 foe(s)") || strings.Contains(fire.Summary, "ally") {
		t.Fatalf("an evoker's burst spares allies: %s", fire.Summary)
	}
	out, _ := rules.TakeTurn(dice(1), g, wiz.ID, mm)
	if out.Rolls[0].Dice != "5d4+5" || wiz.SlotsLeft(3) != 0 {
		t.Fatalf("two extra darts at level 3: %s", out.Rolls[0].Dice)
	}

	wiz.SlotsUsed = nil
	turnOf(g, 0)
	out, _ = rules.TakeTurn(dice(6, 6, 6, 6, 6, 6, 6, 6, 1, 20), g, wiz.ID, rules.Option{ActionID: "fireball", TargetID: gob.ID})
	if gob.HP != 99-10-48 || gob2.HP != 99-24 || ally.HP != 12 || !strings.Contains(strings.Join(out.Notes, " "), "spares Ally") {
		t.Fatalf("fireball: %d %d ally %d %v", gob.HP, gob2.HP, ally.HP, out.Notes)
	}
	bard := hero("bard")
	bard.SpellSlots = map[int]int{2: 1}
	bard.Actions = []domain.Action{{ID: "shatter", Name: "Shatter", Kind: domain.ActSave, Dice: domain.MustDice("3d8"), SaveAbility: domain.CON, SaveDC: 30, RangeFt: 60, SlotLevel: 2, Radius: 10}}
	ally.Pos = domain.Position{X: 8, Y: 1}
	g = arena(bard, gob, ally)
	shatter, _ := has(rules.Options(g, bard.ID), "shatter")
	if !strings.Contains(shatter.Summary, "1 ally") {
		t.Fatalf("other casters are warned about allies in the blast: %s", shatter.Summary)
	}
}

func TestMassHealingWord(t *testing.T) {
	t.Parallel()
	cleric := hero("cleric")
	cleric.SpellSlots = map[int]int{3: 1}
	cleric.Actions = []domain.Action{{ID: "mass", Name: "Mass Healing Word", Kind: domain.ActHeal, Dice: domain.MustDice("1d4+3"), RangeFt: 60, SlotLevel: 3, Mass: true}}
	a, b := hero("a"), hero("b")
	g := arena(cleric, a, b, goblin())
	if _, ok := has(rules.Options(g, cleric.ID), "mass"); ok {
		t.Fatal("nobody hurt, nothing to heal")
	}
	a.HP, b.HP = 2, 3
	mass, ok := has(rules.Options(g, cleric.ID), "mass")
	if !ok || mass.TargetID != "" {
		t.Fatal("one option heals everyone")
	}
	out, _ := rules.TakeTurn(dice(4, 4), g, cleric.ID, mass)
	if a.HP != 9 || b.HP != 10 || out.Amount != 14 {
		t.Fatalf("both healed: %d %d", a.HP, b.HP)
	}
	solo := hero("solo")
	solo.HP, solo.SpellSlots = 1, map[int]int{3: 1}
	if out, _ := rules.Resolve(dice(1), solo, cleric.Actions[0], solo); out.Amount != 4 {
		t.Fatal("outside a fight, only the caster")
	}
}

func TestLevelFiveFeatures(t *testing.T) {
	t.Parallel()
	f := hero("fighter", feat(domain.FeatExtraAttack, 0))
	gob := goblin()
	gob.MaxHP, gob.HP = 99, 99
	g := arena(f, gob)
	out, _ := rules.TakeTurn(dice(2), g, f.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID})
	opts := rules.Options(g, f.ID)
	if out.TurnOver || len(opts) == 0 {
		t.Fatal("the second attack is still to come")
	}
	if _, ok := has(opts, rules.ApproachID); ok {
		t.Fatal("only weapon attacks between the two")
	}
	if _, ok := has(opts, rules.EndTurnID); !ok {
		t.Fatal("you may forgo the second attack")
	}
	if out, _ := rules.TakeTurn(dice(2), g, f.ID, rules.Option{ActionID: "longsword", TargetID: gob.ID}); !out.TurnOver {
		t.Fatal("two attacks spend the action")
	}

	rogue := hero("rogue", feat(domain.FeatUncannyDodge, 0))
	gob = goblin()
	g = arena(gob, rogue)
	out, _ = rules.TakeTurn(dice(19, 6), g, gob.ID, rules.Option{ActionID: "scimitar", TargetID: rogue.ID})
	if rogue.HP != 12-4 || !g.Encounter.Reacted[rogue.ID] {
		t.Fatalf("uncanny dodge halves 8 to 4: HP %d %v", rogue.HP, out.Notes)
	}

	barb := hero("barb", feat(domain.FeatFastMovement, 0))
	if rules.SpeedOf(barb) != 40 || rules.SpeedOf(hero("x")) != 30 {
		t.Fatal("fast movement")
	}

	cleric := hero("cleric", feat(domain.FeatChannelDivinity, 1), feat(domain.FeatDestroyUndead, 0))
	zombie, ghoul := goblin(), goblin()
	zombie.ID, zombie.Type, zombie.CR = "z", "undead", "1/4"
	ghoul.ID, ghoul.Type, ghoul.CR, ghoul.Pos = "gh", "undead", "1", domain.Position{X: 2}
	g = arena(cleric, zombie, ghoul)
	turn, _ := has(rules.Options(g, cleric.ID), rules.TurnUndeadID)
	_, _ = rules.TakeTurn(dice(1, 1), g, cleric.ID, turn)
	if !zombie.Dead || ghoul.Dead || !ghoul.Has(domain.Turned) {
		t.Fatal("weak undead are destroyed, stronger ones turned")
	}

	bard := hero("bard", feat(domain.FeatBardicInspiration, 2), feat(domain.FeatFontOfInspiration, 0))
	bard.Level = 5
	_ = bard.SpendFeature(domain.FeatBardicInspiration)
	bard.Features[0].Recharge = domain.RechargeLong
	bard.RechargeFeatures(domain.RechargeShort)
	if bard.FeatureLeft(domain.FeatBardicInspiration) != 2 || bard.InspirationDie() != 8 {
		t.Fatal("font of inspiration: back on a short rest, and a d8")
	}
}

func TestAbilityScoreImprovements(t *testing.T) {
	t.Parallel()
	c := hero("c")
	c.Level, c.BaseMaxHP = 4, 30
	if err := c.Improve(map[domain.Ability]int{domain.STR: 2}); err == nil {
		t.Fatal("none earned")
	}
	c.ASI = 1
	for _, bad := range []map[domain.Ability]int{
		{domain.STR: 1},
		{domain.STR: 3},
		{domain.STR: 1, domain.DEX: 1, domain.CON: 1},
		{"LUCK": 2},
		{domain.STR: 0, domain.DEX: 2, domain.CON: 1},
	} {
		if err := c.Improve(bad); err == nil {
			t.Errorf("%v should be refused", bad)
		}
	}
	c.Abilities[domain.WIS] = 19
	if err := c.Improve(map[domain.Ability]int{domain.WIS: 2}); err == nil {
		t.Fatal("no score above 20")
	}
	c.ASI = 2
	if err := c.Improve(map[domain.Ability]int{domain.CON: 1, domain.WIS: 1}); err != nil || c.BaseMaxHP != 30 {
		t.Fatalf("CON 14→15 keeps the modifier, so no HP: %v %d", err, c.BaseMaxHP)
	}
	if err := c.Improve(map[domain.Ability]int{domain.CON: 1, domain.STR: 1}); err != nil {
		t.Fatal(err)
	}
	if c.BaseAbilities[domain.CON] != 16 || c.BaseAbilities[domain.WIS] != 20 || c.BaseMaxHP != 30+4 || c.ASI != 0 {
		t.Fatalf("CON 16 raises the modifier: +1 HP a level: %+v %d", c.BaseAbilities, c.BaseMaxHP)
	}
	if domain.ImprovementText(map[domain.Ability]int{domain.CON: 1, domain.STR: 1}) != "STR +1, CON +1" {
		t.Fatal("improvement text")
	}
	for cr, want := range map[string]float64{"1/4": 0.25, "1/2": 0.5, "2": 2, "huge": 99} {
		if got := domain.ChallengeValue(cr); got != want {
			t.Errorf("CR %s = %v", cr, got)
		}
	}
	if domain.HPPerLevel(domain.ClassTemplate{HitDie: 10}, 2) != 8 || domain.HPPerLevel(domain.ClassTemplate{HitDie: 6}, -5) != 1 {
		t.Fatal("hp per level")
	}
}
