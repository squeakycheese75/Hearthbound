package srd

import (
	d "dnd/internal/domain"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// items is the equipment catalogue (SRD 5.1 weapons, armour and adventuring
// gear, plus plain trinkets that carry rolled affixes). Values are in gold;
// anything cheaper than 1 gp is rounded up to 1.
var items = []d.BaseItem{
	// Simple weapons.
	weapon("club", "Club", 1, 2, "1d4 bludgeoning", "A stout length of hardwood.", "light"),
	weapon("dagger", "Dagger", 2, 1, "1d4 piercing", "A short, double-edged blade.", "finesse", "light", "thrown (20/60)"),
	weapon("handaxe", "Handaxe", 5, 2, "1d6 slashing", "A small axe balanced for throwing.", "light", "thrown (20/60)"),
	stack(weapon("javelin", "Javelin", 1, 2, "1d6 piercing", "A light throwing spear.", "thrown (30/120)"), "javelins"),
	weapon("mace", "Mace", 5, 4, "1d6 bludgeoning", "A flanged iron head on a short haft."),
	weapon("quarterstaff", "Quarterstaff", 1, 4, "1d6 bludgeoning", "A long staff of seasoned wood.", "versatile (1d8)"),
	weapon("spear", "Spear", 1, 3, "1d6 piercing", "A long shaft tipped with a leaf-shaped head.", "thrown (20/60)", "versatile (1d8)"),
	weapon("light-crossbow", "Light Crossbow", 25, 5, "1d8 piercing", "A compact crossbow with a hand crank.", "ammunition (80/320)", "loading", "two-handed"),
	weapon("shortbow", "Shortbow", 25, 2, "1d6 piercing", "A small bow for quick shots.", "ammunition (80/320)", "two-handed"),
	// Martial weapons.
	weapon("battleaxe", "Battleaxe", 10, 4, "1d8 slashing", "A broad, bearded axe.", "versatile (1d10)"),
	weapon("greataxe", "Greataxe", 30, 7, "1d12 slashing", "A huge double-bitted axe.", "heavy", "two-handed"),
	weapon("greatsword", "Greatsword", 50, 6, "2d6 slashing", "A blade nearly as tall as its bearer.", "heavy", "two-handed"),
	weapon("longsword", "Longsword", 15, 3, "1d8 slashing", "A straight, double-edged sword.", "versatile (1d10)"),
	weapon("rapier", "Rapier", 25, 2, "1d8 piercing", "A slender thrusting blade.", "finesse"),
	weapon("scimitar", "Scimitar", 25, 3, "1d6 slashing", "A curved, single-edged blade.", "finesse", "light"),
	weapon("shortsword", "Shortsword", 10, 2, "1d6 piercing", "A short stabbing sword.", "finesse", "light"),
	weapon("warhammer", "Warhammer", 15, 2, "1d8 bludgeoning", "A heavy hammer on a long haft.", "versatile (1d10)"),
	weapon("longbow", "Longbow", 50, 2, "1d8 piercing", "A tall bow of yew.", "ammunition (150/600)", "heavy", "two-handed"),
	// Armour.
	armour("leather", "Leather Armour", 10, 10, 11, "Stiffened leather; AC 11 + DEX.", "light", "leather armour"),
	armour("studded-leather", "Studded Leather", 45, 13, 12, "Leather reinforced with rivets; AC 12 + DEX.", "light"),
	armour("hide", "Hide Armour", 10, 12, 12, "Thick furs and pelts; AC 12 + DEX (max 2).", "medium"),
	armour("chain-shirt", "Chain Shirt", 50, 20, 13, "Interlocking rings worn under clothes; AC 13 + DEX (max 2).", "medium"),
	armour("scale-mail", "Scale Mail", 50, 45, 14, "Overlapping metal scales; AC 14 + DEX (max 2).", "medium", "stealth disadvantage", "scale mail"),
	armour("breastplate", "Breastplate", 400, 20, 14, "A fitted metal chest piece; AC 14 + DEX (max 2).", "medium"),
	armour("chain-mail", "Chain Mail", 75, 55, 16, "Full mail with padding; AC 16.", "heavy", "stealth disadvantage", "chain mail"),
	{Key: "shield", Name: "Shield", Kind: d.ItemShield, Value: 10, Weight: 6, AC: 2, Desc: "A wooden or metal shield; +2 AC.", Aliases: []string{"shield"}},
	// Trinkets: plain until they roll affixes.
	{Key: "ring", Name: "Ring", Kind: d.ItemTrinket, Value: 10, Desc: "A plain band of metal."},
	{Key: "amulet", Name: "Amulet", Kind: d.ItemTrinket, Value: 15, Desc: "A pendant on a fine chain."},
	{Key: "charm", Name: "Charm", Kind: d.ItemTrinket, Value: 5, Desc: "A small carved token on a cord."},
	// Potions.
	{Key: "potion-healing", Name: "Potion of Healing", Kind: d.ItemPotion, Value: 50, Weight: 1, Stack: true, Desc: "A red, shimmering liquid. Drink to regain 2d4 + 2 HP.", Aliases: []string{"healing potion", "potion of healing"}},
	// Adventuring gear.
	gear("rations", "Rations", 1, 2, "A day's dried food.", true, "rations"),
	gear("rope", "Rope (50 ft)", 1, 10, "Hempen rope.", false, "rope", "hempen rope"),
	gear("torch", "Torch", 1, 1, "Burns for an hour, shedding bright light.", true, "torch", "torches"),
	gear("arrows", "Arrows", 1, 1, "Ammunition for bows.", true, "quiver of arrows", "arrows"),
	gear("thieves-tools", "Thieves' Tools", 25, 1, "Picks, a file and a small mirror.", false, "thieves' tools"),
	gear("holy-symbol", "Holy Symbol", 5, 1, "An emblem of a god, used to channel divine magic.", false, "holy symbol"),
	gear("spellbook", "Spellbook", 50, 3, "A wizard's book of spells.", false, "spellbook"),
	gear("arcane-focus", "Arcane Focus", 10, 1, "A crystal or wand that channels arcane magic.", false, "arcane focus"),
	gear("lute", "Lute", 35, 2, "A stringed instrument.", false, "lute"),
	// Packs.
	pack("explorers-pack", "Explorer's Pack", 10, 59, "Backpack, bedroll, mess kit, tinderbox, torches, rations, waterskin and rope.", "explorer's pack"),
	pack("burglars-pack", "Burglar's Pack", 16, 44, "Backpack, ball bearings, string, bell, candles, crowbar, lantern and more.", "burglar's pack"),
	pack("priests-pack", "Priest's Pack", 19, 24, "Backpack, blanket, candles, tinderbox, alms box, incense and vestments.", "priest's pack"),
	pack("scholars-pack", "Scholar's Pack", 40, 10, "Backpack, book of lore, ink, pen, parchment, sand and a little knife.", "scholar's pack"),
	pack("entertainers-pack", "Entertainer's Pack", 40, 38, "Backpack, bedroll, costumes, candles, rations, waterskin and a disguise kit.", "entertainer's pack"),
}

// simpleWeapons are the SRD's simple weapons; every other weapon is martial.
var simpleWeapons = []string{"club", "dagger", "handaxe", "javelin", "mace", "quarterstaff", "spear", "light-crossbow", "shortbow"}

var rangeProp = regexp.MustCompile(`\((\d+)/\d+\)`)

// weapon builds a weapon from its SRD line: damage like "1d8 slashing" and
// properties like "finesse" or "ammunition (80/320)".
func weapon(key, name string, value, weight int, damage, desc string, props ...string) d.BaseItem {
	dice, kind, _ := strings.Cut(damage, " ")
	b := d.BaseItem{
		Key: key, Name: name, Kind: d.ItemWeapon, Value: value, Weight: weight, Damage: damage, Desc: desc, Props: props,
		Dice: d.MustDice(dice), DamageType: kind, Category: "martial", RangeFt: 5,
	}
	if slices.Contains(simpleWeapons, key) {
		b.Category = "simple"
	}
	for _, p := range props {
		switch {
		case p == "finesse":
			b.Finesse = true
		case p == "light":
			b.Light = true
		case p == "two-handed":
			b.TwoHanded = true
		case strings.HasPrefix(p, "ammunition"):
			b.Ranged, b.RangeFt = true, propRange(p)
		case strings.HasPrefix(p, "thrown"):
			b.Thrown = true
		}
	}
	return b
}

func propRange(p string) int {
	m := rangeProp.FindStringSubmatch(p)
	if m == nil {
		return 5
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func stack(b d.BaseItem, aliases ...string) d.BaseItem {
	b.Stack, b.Aliases = true, aliases
	return b
}

// armour builds armour; a trailing prop that isn't a real property is an alias.
func armour(key, name string, value, weight, ac int, desc string, rest ...string) d.BaseItem {
	b := d.BaseItem{Key: key, Name: name, Kind: d.ItemArmour, Value: value, Weight: weight, AC: ac, Desc: desc}
	for _, r := range rest {
		if r == "light" || r == "medium" || r == "heavy" || r == "stealth disadvantage" {
			b.Props = append(b.Props, r)
			if r != "stealth disadvantage" {
				b.Category = r
			}
			continue
		}
		b.Aliases = append(b.Aliases, r)
	}
	return b
}

func gear(key, name string, value, weight int, desc string, stacks bool, aliases ...string) d.BaseItem {
	return d.BaseItem{Key: key, Name: name, Kind: d.ItemGear, Value: value, Weight: weight, Desc: desc, Stack: stacks, Aliases: aliases}
}

func pack(key, name string, value, weight int, desc string, aliases ...string) d.BaseItem {
	return d.BaseItem{Key: key, Name: name, Kind: d.ItemPack, Value: value, Weight: weight, Desc: desc, Aliases: aliases}
}

// Items lists the equipment catalogue.
func (*Catalog) Items() []d.BaseItem { return items }

// Item finds a catalogue entry by key.
func (*Catalog) Item(key string) (d.BaseItem, error) {
	for _, b := range items {
		if b.Key == key {
			return b, nil
		}
	}
	return d.BaseItem{}, fmt.Errorf("%w: item %q", d.ErrUnknown, key)
}

// ItemByName finds a catalogue entry by its key, name or one of its aliases,
// ignoring case.
func (*Catalog) ItemByName(name string) (d.BaseItem, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, b := range items {
		if b.Key == n || strings.ToLower(b.Name) == n || slices.Contains(b.Aliases, n) {
			return b, true
		}
	}
	return d.BaseItem{}, false
}
