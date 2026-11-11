package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ItemKind is what sort of thing an item is.
type ItemKind string

// Item kinds.
const (
	ItemWeapon  ItemKind = "weapon"
	ItemArmour  ItemKind = "armour"
	ItemShield  ItemKind = "shield"
	ItemTrinket ItemKind = "trinket" // rings, amulets, charms: carriers of affixes
	ItemPotion  ItemKind = "potion"
	ItemGear    ItemKind = "gear"
	ItemPack    ItemKind = "pack"
	// ItemKeepsake is a story object with no stats: a map, a key, a letter.
	ItemKeepsake ItemKind = "keepsake"
)

// Rarity is how unusual an item is. Rarer items roll more affixes.
type Rarity string

// Rarities, from most to least common.
const (
	Common    Rarity = "common"
	Uncommon  Rarity = "uncommon"
	Rare      Rarity = "rare"
	VeryRare  Rarity = "very rare"
	Legendary Rarity = "legendary"
)

// Rarities lists rarities from most to least common.
var Rarities = []Rarity{Common, Uncommon, Rare, VeryRare, Legendary}

// AffixStat is what an affix improves.
type AffixStat string

// Affix stats. Ability affixes use the ability's name (STR, DEX...).
const (
	StatToHit  AffixStat = "to_hit"
	StatDamage AffixStat = "damage"
	StatAC     AffixStat = "ac"
	StatMaxHP  AffixStat = "max_hp"
)

// Affix is one rolled property of an item, like "+1 to hit" (Keen).
type Affix struct {
	Key    string    `json:"key"`
	Name   string    `json:"name"`
	Prefix bool      `json:"prefix,omitempty"`
	Stat   AffixStat `json:"stat"`
	Value  int       `json:"value"`
}

// Text describes an affix's effect, e.g. "+1 to hit".
func (a Affix) Text() string {
	switch a.Stat {
	case StatToHit:
		return fmt.Sprintf("%+d to hit", a.Value)
	case StatDamage:
		return fmt.Sprintf("%+d damage", a.Value)
	case StatAC:
		return fmt.Sprintf("%+d AC", a.Value)
	case StatMaxHP:
		return fmt.Sprintf("%+d max HP", a.Value)
	}
	return fmt.Sprintf("%+d %s", a.Value, a.Stat)
}

// BaseItem is a catalogue entry: a type of item before any rolled properties.
type BaseItem struct {
	Key    string   `json:"key"`
	Name   string   `json:"name"`
	Kind   ItemKind `json:"kind"`
	Value  int      `json:"value"`  // gold pieces
	Weight int      `json:"weight"` // pounds
	Desc   string   `json:"desc"`
	// Damage is a weapon's damage, e.g. "1d8 slashing".
	Damage string `json:"damage,omitempty"`
	// AC is armour's base AC, or a shield's bonus.
	AC    int      `json:"ac,omitempty"`
	Props []string `json:"props,omitempty"`
	// Weapon details: dice, damage type, simple or martial, reach or range.
	Dice       Dice   `json:"dice,omitzero"`
	DamageType string `json:"damageType,omitempty"`
	Category   string `json:"category,omitempty"` // simple, martial; light, medium, heavy for armour
	Finesse    bool   `json:"finesse,omitempty"`
	Light      bool   `json:"light,omitempty"`
	TwoHanded  bool   `json:"twoHanded,omitempty"`
	// Ranged weapons fire ammunition; thrown weapons can also be hurled. RangeFt
	// is the normal range for either (melee reach is 5 ft).
	Ranged  bool `json:"ranged,omitempty"`
	Thrown  bool `json:"thrown,omitempty"`
	RangeFt int  `json:"rangeFt,omitempty"`
	// Stack marks items counted rather than kept apart (arrows, rations).
	Stack bool `json:"stack,omitempty"`
	// Aliases are other names the catalogue accepts (for converting old gear).
	Aliases []string `json:"-"`
}

// Item is one specific, owned object.
type Item struct {
	ID      string   `json:"id"`
	Base    string   `json:"base,omitempty"` // catalogue key; empty for keepsakes
	Name    string   `json:"name"`
	Kind    ItemKind `json:"kind"`
	Rarity  Rarity   `json:"rarity"`
	Affixes []Affix  `json:"affixes,omitempty"`
	Value   int      `json:"value"`
	Weight  int      `json:"weight"`
	Qty     int      `json:"qty,omitempty"`
	Desc    string   `json:"desc,omitempty"`
	Damage  string   `json:"damage,omitempty"`
	AC      int      `json:"ac,omitempty"`
	Props   []string `json:"props,omitempty"`
	// History is where the item has been: found, bought, traded.
	History []string `json:"history,omitempty"`
	// Slot is where the item is equipped ("" when it's just carried).
	Slot Slot `json:"slot,omitempty"`
}

// NewItem makes a plain (common) item from a catalogue entry.
func NewItem(id string, b BaseItem, qty int) Item {
	it := Item{
		ID: id, Base: b.Key, Name: b.Name, Kind: b.Kind, Rarity: Common, Value: b.Value, Weight: b.Weight,
		Desc: b.Desc, Damage: b.Damage, AC: b.AC, Props: append([]string(nil), b.Props...),
	}
	if b.Stack {
		it.Qty = max(qty, 1)
	}
	return it
}

// Keepsake makes a story object with no stats or value.
func Keepsake(id, name string) Item {
	return Item{ID: id, Name: name, Kind: ItemKeepsake, Rarity: Common}
}

// Label is how an item reads in prompts: name, quantity and rarity.
func (i Item) Label() string {
	label := i.Name
	if i.Qty > 1 {
		label += fmt.Sprintf(" (%d)", i.Qty)
	}
	if i.Rarity != Common && i.Rarity != "" {
		label += " [" + string(i.Rarity) + "]"
	}
	if len(i.Affixes) > 0 {
		parts := make([]string, 0, len(i.Affixes))
		for _, a := range i.Affixes {
			parts = append(parts, a.Text())
		}
		label += " (" + strings.Join(parts, ", ") + ")"
	}
	return label
}

var qtySuffix = regexp.MustCompile(`^(.*?)\s*\((\d+)\)\s*$`)

// splitQty reads "rations (5)" as ("rations", 5).
func splitQty(s string) (string, int) {
	m := qtySuffix.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return strings.TrimSpace(s), 1
	}
	n, _ := strconv.Atoi(m[2])
	return m[1], n
}

// AdoptLegacyItems converts an old free-text inventory into items: names the
// catalogue knows become catalogue items, the rest keepsakes. IDs are
// derived from the creature's ID so converting is repeatable.
func (c *Creature) AdoptLegacyItems(lookup func(name string) (BaseItem, bool)) {
	if len(c.Inventory) == 0 {
		return
	}
	for n, raw := range c.Inventory {
		name, qty := splitQty(raw)
		id := fmt.Sprintf("itm-%s-%d", c.ID, n)
		if b, ok := lookup(name); ok {
			c.Items = append(c.Items, NewItem(id, b, qty))
			continue
		}
		c.Items = append(c.Items, Keepsake(id, raw))
	}
	c.Inventory = nil
}

// NewItemID returns a fresh ID for an item created in this adventure.
func (g *Game) NewItemID() string {
	g.ItemSeq++
	return fmt.Sprintf("itm-%s-%d", g.ID, g.ItemSeq)
}
