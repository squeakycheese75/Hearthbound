package rules

import (
	"dnd/internal/domain"
	"fmt"
	"slices"
	"strings"
)

// Random loot: the engine rolls an item's rarity from the party's level, then
// rolls affixes from tables for its kind. Bonuses stay within +1/+2 so they
// fit 5e's bounded accuracy at levels 1-3.

// TurnLootCap is the most rolled items the DM can hand out in one turn.
const TurnLootCap = 2

// LootKinds are the kinds the engine rolls as random loot.
var LootKinds = []domain.ItemKind{domain.ItemWeapon, domain.ItemArmour, domain.ItemShield, domain.ItemTrinket}

// rarityOdds are cumulative d100 thresholds per party level for common,
// uncommon, rare and very rare. Legendary items are set pieces, never rolled.
var rarityOdds = map[int][4]int{
	1: {70, 94, 100, 100},
	2: {60, 88, 99, 100},
	3: {50, 82, 97, 100},
	4: {45, 78, 95, 100},
	5: {40, 74, 93, 100},
}

// RollRarity rolls an item's rarity at a party level. good loot (a boss, a
// hoard) rolls twice and keeps the better result.
func RollRarity(r Roller, level int, good bool) domain.Rarity {
	rarity := rollRarityOnce(r, level)
	if good {
		if other := rollRarityOnce(r, level); rank(other) > rank(rarity) {
			rarity = other
		}
	}
	return rarity
}

func rollRarityOnce(r Roller, level int) domain.Rarity {
	odds, ok := rarityOdds[min(max(level, 1), MaxLevel)]
	if !ok {
		odds = rarityOdds[1]
	}
	roll := r.Roll(100)
	for i, t := range odds {
		if roll <= t {
			return domain.Rarities[i]
		}
	}
	return domain.Common
}

func rank(r domain.Rarity) int { return slices.Index(domain.Rarities, r) }

// affixCount is how many affixes each rarity rolls.
var affixCount = map[domain.Rarity]int{domain.Common: 0, domain.Uncommon: 1, domain.Rare: 2, domain.VeryRare: 3, domain.Legendary: 3}

// rarityValue is what each rarity adds to an item's base value, in gold.
var rarityValue = map[domain.Rarity]int{domain.Common: 0, domain.Uncommon: 60, domain.Rare: 250, domain.VeryRare: 900, domain.Legendary: 3000}

type affixDef struct {
	domain.Affix
	min   domain.Rarity
	kinds []domain.ItemKind
}

var (
	weapons   = []domain.ItemKind{domain.ItemWeapon}
	defences  = []domain.ItemKind{domain.ItemArmour, domain.ItemShield}
	trinkets  = []domain.ItemKind{domain.ItemTrinket}
	worn      = []domain.ItemKind{domain.ItemArmour, domain.ItemShield, domain.ItemTrinket}
	anyLooted = []domain.ItemKind{domain.ItemWeapon, domain.ItemArmour, domain.ItemShield, domain.ItemTrinket}
)

func prefix(key, name string, stat domain.AffixStat, v int, minR domain.Rarity, kinds []domain.ItemKind) affixDef {
	return affixDef{Affix: domain.Affix{Key: key, Name: name, Prefix: true, Stat: stat, Value: v}, min: minR, kinds: kinds}
}

func suffix(key, name string, stat domain.AffixStat, v int, minR domain.Rarity, kinds []domain.ItemKind) affixDef {
	return affixDef{Affix: domain.Affix{Key: key, Name: name, Stat: stat, Value: v}, min: minR, kinds: kinds}
}

// affixes are every affix the engine can roll, with the rarity it needs.
var affixes = []affixDef{
	prefix("keen", "Keen", domain.StatToHit, 1, domain.Uncommon, weapons),
	prefix("honed", "Honed", domain.StatToHit, 2, domain.Rare, weapons),
	prefix("brutal", "Brutal", domain.StatDamage, 1, domain.Uncommon, weapons),
	prefix("savage", "Savage", domain.StatDamage, 2, domain.Rare, weapons),
	prefix("sturdy", "Sturdy", domain.StatAC, 1, domain.Uncommon, defences),
	prefix("bulwark", "Bulwark", domain.StatAC, 2, domain.VeryRare, defences),
	prefix("lucky", "Lucky", domain.StatToHit, 1, domain.Uncommon, trinkets),
	prefix("warding", "Warding", domain.StatAC, 1, domain.Rare, trinkets),
	suffix("bear", "of the Bear", "STR", 1, domain.Uncommon, anyLooted),
	suffix("fox", "of the Fox", "DEX", 1, domain.Uncommon, anyLooted),
	suffix("ox", "of the Ox", "CON", 1, domain.Uncommon, worn),
	suffix("owl", "of the Owl", "WIS", 1, domain.Uncommon, trinkets),
	suffix("sage", "of the Sage", "INT", 1, domain.Uncommon, trinkets),
	suffix("silver", "of the Silver Tongue", "CHA", 1, domain.Uncommon, trinkets),
	suffix("vigour", "of Vigour", domain.StatMaxHP, 3, domain.Uncommon, worn),
	suffix("vitality", "of Vitality", domain.StatMaxHP, 6, domain.Rare, worn),
	suffix("hawk", "of the Hawk", domain.StatToHit, 1, domain.Rare, weapons),
}

// PickBase picks a random catalogue entry of a kind (any loot kind when kind
// is empty).
func PickBase(r Roller, bases []domain.BaseItem, kind domain.ItemKind) (domain.BaseItem, error) {
	var pool []domain.BaseItem
	for _, b := range bases {
		if (kind == "" && slices.Contains(LootKinds, b.Kind)) || b.Kind == kind {
			pool = append(pool, b)
		}
	}
	if len(pool) == 0 {
		return domain.BaseItem{}, fmt.Errorf("%w: no items of kind %q to roll", domain.ErrInvalid, kind)
	}
	return pool[r.Roll(len(pool))-1], nil
}

// RollItem makes a random item from a base at a party level: rarity, then
// affixes (each stat at most once), then its name and value.
func RollItem(r Roller, id string, base domain.BaseItem, level int, good bool) domain.Item {
	it := domain.NewItem(id, base, 1)
	if !slices.Contains(LootKinds, base.Kind) {
		return it
	}
	it.Rarity = RollRarity(r, level, good)
	for range affixCount[it.Rarity] {
		pool := affixPool(base.Kind, it.Rarity, it.Affixes)
		if len(pool) == 0 {
			break
		}
		it.Affixes = append(it.Affixes, pool[r.Roll(len(pool))-1].Affix)
	}
	it.Name = itemName(base.Name, it.Affixes)
	it.Value = base.Value + rarityValue[it.Rarity]
	return it
}

// affixPool lists affixes this item may still roll: right kind, rarity high
// enough, and a stat it doesn't have yet.
func affixPool(kind domain.ItemKind, rarity domain.Rarity, have []domain.Affix) []affixDef {
	var pool []affixDef
	for _, a := range affixes {
		if !slices.Contains(a.kinds, kind) || rank(a.min) > rank(rarity) {
			continue
		}
		if slices.ContainsFunc(have, func(h domain.Affix) bool { return h.Stat == a.Stat }) {
			continue
		}
		pool = append(pool, a)
	}
	return pool
}

// itemName puts the first prefix before the base name and the first suffix
// after it: "Keen Longsword of the Bear".
func itemName(base string, affs []domain.Affix) string {
	var pre, suf string
	for _, a := range affs {
		if a.Prefix && pre == "" {
			pre = a.Name
		}
		if !a.Prefix && suf == "" {
			suf = a.Name
		}
	}
	return strings.TrimSpace(strings.Join([]string{pre, base, suf}, " "))
}
