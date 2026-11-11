package rules_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"errors"
	"testing"
)

var sword = domain.BaseItem{Key: "longsword", Name: "Longsword", Kind: domain.ItemWeapon, Value: 15, Damage: "1d8 slashing"}

func TestRarityOddsClimbWithLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level int
		roll  int
		want  domain.Rarity
	}{
		{1, 70, domain.Common},
		{1, 71, domain.Uncommon},
		{1, 95, domain.Rare},
		{1, 100, domain.Rare},
		{2, 100, domain.VeryRare},
		{3, 98, domain.VeryRare},
		{3, 50, domain.Common},
		{0, 71, domain.Uncommon},
		{9, 98, domain.VeryRare},
	}
	for _, c := range cases {
		if got := rules.RollRarity(dice(c.roll), c.level, false); got != c.want {
			t.Errorf("level %d roll %d: %s, want %s", c.level, c.roll, got, c.want)
		}
	}
	if got := rules.RollRarity(dice(10, 96), 1, true); got != domain.Rare {
		t.Fatalf("good loot keeps the better of two rolls: %s", got)
	}
	if got := rules.RollRarity(dice(96, 10), 1, true); got != domain.Rare {
		t.Fatalf("good loot never gets worse: %s", got)
	}
}

func TestRollItem(t *testing.T) {
	t.Parallel()
	plain := rules.RollItem(dice(5), "i1", sword, 1, false)
	if plain.Rarity != domain.Common || len(plain.Affixes) != 0 || plain.Name != "Longsword" || plain.Value != 15 || plain.ID != "i1" {
		t.Fatalf("a common roll is the plain item: %+v", plain)
	}
	rare := rules.RollItem(dice(96, 1, 1), "i2", sword, 1, false) // rare: two affixes, first of each pool
	if rare.Rarity != domain.Rare || len(rare.Affixes) != 2 || rare.Affixes[0].Stat == rare.Affixes[1].Stat {
		t.Fatalf("a rare roll has two affixes on different stats: %+v", rare)
	}
	if rare.Name != "Keen Longsword" || rare.Value != 15+250 || rare.Affixes[0].Text() != "+1 to hit" {
		t.Fatalf("name and value: %q %d %v", rare.Name, rare.Value, rare.Affixes)
	}
	ring := domain.BaseItem{Key: "ring", Name: "Ring", Kind: domain.ItemTrinket, Value: 10}
	vr := rules.RollItem(dice(100, 99, 99, 99), "i3", ring, 3, false)
	if vr.Rarity != domain.VeryRare || len(vr.Affixes) != 3 {
		t.Fatalf("very rare trinkets roll three affixes: %+v", vr)
	}
	potion := domain.BaseItem{Key: "potion", Name: "Potion", Kind: domain.ItemPotion, Value: 50}
	if p := rules.RollItem(dice(100), "i4", potion, 3, true); p.Rarity != domain.Common || len(p.Affixes) != 0 {
		t.Fatalf("non-loot kinds stay plain: %+v", p)
	}
	shield := domain.BaseItem{Key: "shield", Name: "Shield", Kind: domain.ItemShield, AC: 2}
	if s := rules.RollItem(dice(100, 1, 1, 1), "i5", shield, 2, false); len(s.Affixes) != 3 || s.Name != "Sturdy Shield of the Bear" {
		t.Fatalf("a very rare shield: %+v", s)
	}
	tiny := domain.BaseItem{Key: "charm", Name: "Charm", Kind: domain.ItemTrinket}
	if c := rules.RollItem(dice(100, 99, 99, 99, 99), "i6", tiny, 2, false); len(c.Affixes) != 3 {
		t.Fatalf("affix pools never repeat a stat: %+v", c)
	}
}

func TestPickBase(t *testing.T) {
	t.Parallel()
	bases := []domain.BaseItem{sword, {Key: "rope", Kind: domain.ItemGear}, {Key: "ring", Kind: domain.ItemTrinket}}
	if b, _ := rules.PickBase(dice(2), bases, ""); b.Key != "ring" {
		t.Fatalf("any loot kind, never gear: %s", b.Key)
	}
	if b, _ := rules.PickBase(dice(1), bases, domain.ItemTrinket); b.Key != "ring" {
		t.Fatalf("by kind: %s", b.Key)
	}
	if _, err := rules.PickBase(dice(1), bases, domain.ItemArmour); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no armour to pick: %v", err)
	}
}
