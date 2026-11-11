package play_test

import (
	"dnd/internal/domain"
	"errors"
	"testing"
)

func action(c *domain.Creature, id string) *domain.Action {
	for i := range c.Actions {
		if c.Actions[i].ID == id {
			return &c.Actions[i]
		}
	}
	return nil
}

func gearItem(id, base, name string, kind domain.ItemKind, affixes ...domain.Affix) domain.Item {
	return domain.Item{ID: id, Base: base, Name: name, Kind: kind, Rarity: domain.Rare, Affixes: affixes}
}

func TestOutfittingKeepsTheNumbersAndGearChangesThem(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	before := *g.Creatures[0]
	beforeSword := *action(&before, "longsword")
	h := setup(t, g)
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "nothing", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown items: %v", err)
	}
	hero := h.store.game(t).Creatures[0]
	if !hero.Outfitted || hero.Equipped(domain.SlotMain).Name != "Longsword" || hero.Equipped(domain.SlotRanged).Name != "Light Crossbow" ||
		hero.Equipped(domain.SlotArmour).Name != "Chain Mail" || hero.Equipped(domain.SlotOff).Name != "Shield" {
		t.Fatalf("outfitted from class gear: %+v", hero.Items)
	}
	sword := action(hero, "longsword")
	// The Fighter's Defense style adds 1 AC once they wear armour.
	if hero.AC != before.AC+1 || hero.MaxHP != before.MaxHP || sword.ToHit != beforeSword.ToHit || sword.Dice != beforeSword.Dice || action(hero, "light_crossbow") == nil {
		t.Fatalf("outfitting changes nothing: AC %d/%d HP %d/%d sword %+v/%+v", hero.AC, before.AC, hero.MaxHP, before.MaxHP, sword, beforeSword)
	}

	// Loot: a rare longsword (+1 to hit, +1 STR) and a ring (+1 AC, +3 max HP).
	g = h.store.game(t)
	hero = g.Creatures[0]
	hero.Items = append(hero.Items,
		gearItem("keen", "longsword", "Keen Longsword of the Bear", domain.ItemWeapon,
			domain.Affix{Key: "keen", Stat: domain.StatToHit, Value: 1}, domain.Affix{Key: "bear", Stat: "STR", Value: 1}),
		gearItem("ring", "ring", "Sturdy Ring of Vigour", domain.ItemTrinket,
			domain.Affix{Key: "warding", Stat: domain.StatAC, Value: 1}, domain.Affix{Key: "vigour", Stat: domain.StatMaxHP, Value: 3}),
		gearItem("axe", "greataxe", "Greataxe", domain.ItemWeapon),
		gearItem("rope", "rope", "Rope", domain.ItemGear),
		gearItem("javelin", "javelin", "Javelin", domain.ItemWeapon),
	)
	_ = h.store.Save(ctx, g)
	str := hero.Abilities[domain.STR]
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "keen", domain.SlotMain); err != nil {
		t.Fatal(err)
	}
	hero = h.store.game(t).Creatures[0]
	keen := action(hero, "longsword")
	if hero.Abilities[domain.STR] != str+1 || keen.Name != "Keen Longsword of the Bear" || keen.ToHit < sword.ToHit+1 || hero.Item("itm-pc1-w0").Slot != "" {
		t.Fatalf("the rare sword replaces the old one and its bonuses apply: STR %d hit %d/%d", hero.Abilities[domain.STR], keen.ToHit, sword.ToHit)
	}
	if crossbow := action(hero, "light_crossbow"); crossbow.ToHit != action(&before, "light_crossbow").ToHit {
		t.Fatal("a weapon's to-hit bonus doesn't leak onto other weapons")
	}
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "ring", domain.SlotTrinket1); err != nil {
		t.Fatal(err)
	}
	hero = h.store.game(t).Creatures[0]
	if hero.AC != before.AC+2 || hero.MaxHP != before.MaxHP+3 || hero.HP != before.HP+3 {
		t.Fatalf("worn bonuses: AC %d HP %d/%d", hero.AC, hero.HP, hero.MaxHP)
	}
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "ring", ""); err != nil {
		t.Fatal(err)
	}
	if hero = h.store.game(t).Creatures[0]; hero.AC != before.AC+1 || hero.MaxHP != before.MaxHP || hero.HP != before.HP {
		t.Fatalf("taking it off removes its bonuses: AC %d HP %d/%d", hero.AC, hero.HP, hero.MaxHP)
	}

	if err := h.uc.Equip(ctx, me, "g1", "pc1", "axe", domain.SlotMain); err != nil {
		t.Fatal(err)
	}
	hero = h.store.game(t).Creatures[0]
	if hero.Equipped(domain.SlotOff) != nil || hero.AC != before.AC-1 { // no shield, still Defense
		t.Fatalf("a two-handed axe frees the off hand: AC %d", hero.AC)
	}
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "itm-pc1-1", domain.SlotOff); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no shield with a greataxe: %v", err)
	}
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "rope", domain.SlotMain); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("rope isn't a weapon: %v", err)
	}
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "javelin", domain.SlotRanged); err != nil {
		t.Fatal(err)
	}
	if j := action(h.store.game(t).Creatures[0], "javelin"); j == nil || j.RangeFt != 30 {
		t.Fatalf("a thrown javelin uses its thrown range: %+v", j)
	}
	for _, id := range []string{"axe", "javelin"} {
		_ = h.uc.Equip(ctx, me, "g1", "pc1", id, "")
	}
	if a := action(h.store.game(t).Creatures[0], "unarmed"); a == nil {
		t.Fatal("with no weapons, an unarmed strike")
	}
}

func TestEquipRules(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Creatures[1].Items = []domain.Item{gearItem("mail", "chain-mail", "Chain Mail", domain.ItemArmour)}
	g.Creatures[1].Inventory = nil
	h := setup(t, g)
	if err := h.uc.Equip(ctx, me, "g1", "pc2", "mail", domain.SlotArmour); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("clerics aren't trained for heavy armour: %v", err)
	}
	if err := h.uc.Equip(ctx, friend, "g1", "pc1", "x", domain.SlotMain); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("strangers: %v", err)
	}
	g = h.store.game(t)
	g.Creatures[1].OwnerID = friend
	_ = h.store.Save(ctx, g)
	if err := h.uc.Equip(ctx, friend, "g1", "pc1", "itm-pc1-w0", ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("only your own characters: %v", err)
	}
	g = h.store.game(t)
	g.Mode = domain.ModeCombat
	_ = h.store.Save(ctx, g)
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "itm-pc1-w0", ""); !errors.Is(err, domain.ErrWrongMode) {
		t.Fatalf("not mid-fight: %v", err)
	}
	g.Mode = domain.ModeExploration
	_ = h.store.Save(ctx, g)
	h.cat.failClass = true
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "itm-pc1-w0", ""); err == nil {
		t.Fatal("a missing class template fails cleanly")
	}
}

func TestLevelUpKeepsGearBonuses(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	hero := g.Creatures[0]
	hero.XP = 250
	hero.Items = append(hero.Items, gearItem("ring", "ring", "Ring of Vigour", domain.ItemTrinket, domain.Affix{Key: "vigour", Stat: domain.StatMaxHP, Value: 3}))
	h := setup(t, g)
	if err := h.uc.Equip(ctx, me, "g1", "pc1", "ring", domain.SlotTrinket1); err != nil {
		t.Fatal(err)
	}
	base := h.store.game(t).Creatures[0].BaseMaxHP
	h.agent.on(domain.DMSeat, call("award_xp", `{"amount":50,"reason":"x"}`), say("ok"))
	if err := h.uc.Act(ctx, me, "g1", "go"); err != nil {
		t.Fatal(err)
	}
	hero = h.store.game(t).Creatures[0]
	if hero.Level != 2 || hero.BaseMaxHP <= base || hero.MaxHP != hero.BaseMaxHP+3 || hero.HP != hero.MaxHP {
		t.Fatalf("levels up from the base and keeps the ring: L%d base %d max %d hp %d", hero.Level, hero.BaseMaxHP, hero.MaxHP, hero.HP)
	}
}
