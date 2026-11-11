package domain

import (
	"fmt"
	"slices"
	"strings"
)

// Slot is where an item is equipped.
type Slot string

// Equipment slots.
const (
	SlotMain     Slot = "main"     // a melee or thrown weapon
	SlotOff      Slot = "off"      // a shield or a light melee weapon
	SlotRanged   Slot = "ranged"   // a bow, crossbow or thrown weapon
	SlotArmour   Slot = "armour"   // body armour
	SlotTrinket1 Slot = "trinket1" // a ring, amulet or charm
	SlotTrinket2 Slot = "trinket2"
)

// Slots lists every equipment slot.
var Slots = []Slot{SlotMain, SlotOff, SlotRanged, SlotArmour, SlotTrinket1, SlotTrinket2}

// ParseSlot validates a slot ("" means unequip).
func ParseSlot(s string) (Slot, error) {
	if s == "" || slices.Contains(Slots, Slot(s)) {
		return Slot(s), nil
	}
	return "", fmt.Errorf("%w: slot %q", ErrUnknown, s)
}

// ItemLookup finds a catalogue entry by key.
type ItemLookup func(key string) (BaseItem, bool)

// Item finds one of the creature's items by ID.
func (c *Creature) Item(id string) *Item {
	for i := range c.Items {
		if c.Items[i].ID == id {
			return &c.Items[i]
		}
	}
	return nil
}

// Equipped returns the item in a slot, or nil.
func (c *Creature) Equipped(slot Slot) *Item {
	for i := range c.Items {
		if c.Items[i].Slot == slot {
			return &c.Items[i]
		}
	}
	return nil
}

// Proficient reports whether a class can use a weapon, armour or shield well.
// Weapons work without proficiency (no proficiency bonus); armour and shields
// can't be worn without it.
func (cls ClassTemplate) Proficient(b BaseItem) bool {
	switch b.Kind {
	case ItemWeapon:
		return slices.Contains(cls.WeaponProf, b.Category) || slices.Contains(cls.WeaponProf, b.Key)
	case ItemArmour:
		return slices.Contains(cls.ArmourProf, b.Category)
	case ItemShield:
		return slices.Contains(cls.ArmourProf, "shield")
	}
	return true
}

// slotRules say what each slot takes.
var slotRules = map[Slot]func(b BaseItem) bool{
	SlotMain:     func(b BaseItem) bool { return b.Kind == ItemWeapon && !b.Ranged },
	SlotOff:      func(b BaseItem) bool { return b.Kind == ItemShield || (b.Kind == ItemWeapon && b.Light && !b.Ranged) },
	SlotRanged:   func(b BaseItem) bool { return b.Kind == ItemWeapon && (b.Ranged || b.Thrown) },
	SlotArmour:   func(b BaseItem) bool { return b.Kind == ItemArmour },
	SlotTrinket1: func(b BaseItem) bool { return b.Kind == ItemTrinket },
	SlotTrinket2: func(b BaseItem) bool { return b.Kind == ItemTrinket },
}

// fits reports whether a catalogue item can go in a slot.
func fits(b BaseItem, slot Slot) bool {
	rule, ok := slotRules[slot]
	return ok && rule(b)
}

// Equip puts one of the creature's items in a slot (an empty slot takes it
// off). Whatever was in the slot goes back in the pack, and a two-handed
// weapon frees the off hand. Call Recalculate afterwards.
func (c *Creature) Equip(itemID string, slot Slot, cls ClassTemplate, lookup ItemLookup) error {
	it := c.Item(itemID)
	if it == nil {
		return fmt.Errorf("%w: %s isn't carrying that", ErrNotFound, c.Name)
	}
	if slot == "" {
		it.Slot = ""
		return nil
	}
	b, ok := lookup(it.Base)
	if !ok || !fits(b, slot) {
		return fmt.Errorf("%w: %s can't go in the %s slot", ErrInvalid, it.Name, slot)
	}
	if b.Kind != ItemWeapon && !cls.Proficient(b) {
		return fmt.Errorf("%w: %ss aren't trained to use %s", ErrInvalid, cls.Name, it.Name)
	}
	if err := c.clearFor(b, slot, lookup); err != nil {
		return err
	}
	it = c.Item(itemID) // clearFor may have moved things; find it again
	it.Slot = slot
	return nil
}

// clearFor empties a slot for an incoming item and resolves two-handed
// conflicts between the main and off hand.
func (c *Creature) clearFor(b BaseItem, slot Slot, lookup ItemLookup) error {
	if old := c.Equipped(slot); old != nil {
		old.Slot = ""
	}
	if slot == SlotMain && b.TwoHanded {
		if off := c.Equipped(SlotOff); off != nil {
			off.Slot = ""
		}
	}
	if slot == SlotOff {
		if main := c.Equipped(SlotMain); main != nil {
			if mb, ok := lookup(main.Base); ok && mb.TwoHanded {
				return fmt.Errorf("%w: %s needs both hands", ErrInvalid, main.Name)
			}
		}
	}
	return nil
}

// Outfit gives a character who has never been equipped their class's weapons
// as items (if they don't carry them yet) and equips the best of what they
// carry. It runs once per character.
func (c *Creature) Outfit(cls ClassTemplate, byName func(name string) (BaseItem, bool), lookup ItemLookup) {
	if c.Outfitted || c.Kind != KindPC {
		return
	}
	c.Outfitted = true
	c.addClassWeapons(cls, byName)
	order := c.outfitOrder(cls, byName)
	for _, slot := range []Slot{SlotArmour, SlotMain, SlotRanged, SlotOff, SlotTrinket1, SlotTrinket2} {
		if c.Equipped(slot) != nil {
			continue
		}
		for _, i := range order {
			if c.Items[i].Slot == "" && c.Equip(c.Items[i].ID, slot, cls, lookup) == nil {
				break
			}
		}
	}
}

// outfitOrder lists the character's items with the class's own weapons first,
// in the class's order, so a barbarian takes up the greataxe, not a javelin.
func (c *Creature) outfitOrder(cls ClassTemplate, byName func(name string) (BaseItem, bool)) []int {
	rank := map[string]int{}
	for n, w := range cls.Weapons {
		if b, ok := byName(w.Name); ok {
			rank[b.Key] = n + 1
		}
	}
	order := make([]int, len(c.Items))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		ra, rb := rank[c.Items[a].Base], rank[c.Items[b].Base]
		switch {
		case ra == rb:
			return 0
		case ra == 0:
			return 1
		case rb == 0:
			return -1
		}
		return ra - rb
	})
	return order
}

// addClassWeapons gives a character their class's weapons as items, unless
// they already carry one of that kind.
func (c *Creature) addClassWeapons(cls ClassTemplate, byName func(name string) (BaseItem, bool)) {
	for n, w := range cls.Weapons {
		b, ok := byName(w.Name)
		if !ok || slices.ContainsFunc(c.Items, func(it Item) bool { return it.Base == b.Key }) {
			continue
		}
		c.Items = append(c.Items, NewItem(fmt.Sprintf("itm-%s-w%d", c.ID, n), b, 1))
	}
}

// Recalculate derives the character's numbers from their base values, class
// and equipment: abilities, max HP, AC, and weapon attacks (plus spells).
// It is safe to call repeatedly.
func (c *Creature) Recalculate(cls ClassTemplate, lookup ItemLookup) {
	if c.BaseAbilities == nil {
		c.BaseAbilities = AbilityScores{}
		for a, v := range c.Abilities {
			c.BaseAbilities[a] = v
		}
	}
	if c.BaseMaxHP == 0 {
		c.BaseMaxHP = c.MaxHP
	}
	c.SyncFeatures(cls)
	worn := c.wornBonuses()
	c.Abilities = AbilityScores{}
	for _, a := range Abilities {
		if v, ok := c.BaseAbilities[a]; ok || worn[AffixStat(a)] != 0 {
			c.Abilities[a] = min(v+worn[AffixStat(a)], 20)
		}
	}
	maxHP := c.BaseMaxHP + worn[StatMaxHP]
	if delta := maxHP - c.MaxHP; delta > 0 && c.HP > 0 {
		c.HP += delta
	}
	c.MaxHP = maxHP
	c.HP = min(c.HP, c.MaxHP)
	c.AC = c.armourClass(cls, lookup) + worn[StatAC]
	c.Actions = append(c.weaponActions(cls, lookup, worn), spellActions(cls, c)...)
	c.fightingStyles()
}

// fightingStyles applies Defense (+1 AC in armour) and Archery (+2 to hit
// with ranged weapons).
func (c *Creature) fightingStyles() {
	if c.HasFeature(FeatDefenseStyle) && c.Equipped(SlotArmour) != nil {
		c.AC++
	}
	if !c.HasFeature(FeatArcheryStyle) {
		return
	}
	for i, a := range c.Actions {
		if a.Weapon && a.Ranged() && a.Stat == DEX {
			c.Actions[i].ToHit += 2
			c.Actions[i].Description = fmt.Sprintf("%+d to hit, %s %s", c.Actions[i].ToHit, a.Dice, a.DamageType)
		}
	}
}

// wornBonuses totals the affixes that apply to everything: those on armour,
// shields and trinkets, plus ability, AC and HP affixes on weapons. A
// weapon's to-hit and damage affixes apply only to its own attacks.
func (c *Creature) wornBonuses() map[AffixStat]int {
	out := map[AffixStat]int{}
	for _, it := range c.Items {
		if it.Slot == "" {
			continue
		}
		for _, a := range it.Affixes {
			if it.Kind == ItemWeapon && (a.Stat == StatToHit || a.Stat == StatDamage) {
				continue
			}
			out[a.Stat] += a.Value
		}
	}
	return out
}

// armourClass is the equipped armour's AC (or unarmoured: 10 + DEX, plus CON
// for classes with unarmoured defence), plus a shield.
func (c *Creature) armourClass(cls ClassTemplate, lookup ItemLookup) int {
	dex := c.Abilities.Mod(DEX)
	ac := 10 + dex
	if cls.UnarmoredCON {
		ac += c.Abilities.Mod(CON)
	}
	if it := c.Equipped(SlotArmour); it != nil {
		if b, ok := lookup(it.Base); ok {
			ac = b.AC + armourDex(b.Category, dex)
		}
	}
	if it := c.Equipped(SlotOff); it != nil && it.Kind == ItemShield {
		if b, ok := lookup(it.Base); ok {
			ac += b.AC
		}
	}
	return ac
}

func armourDex(category string, dex int) int {
	switch category {
	case "heavy":
		return 0
	case "medium":
		return min(dex, 2)
	}
	return dex
}

// weaponActions builds attacks from the main-hand and ranged weapons, or an
// unarmed strike when neither is equipped.
func (c *Creature) weaponActions(cls ClassTemplate, lookup ItemLookup, worn map[AffixStat]int) []Action {
	var out []Action
	for _, slot := range []Slot{SlotMain, SlotRanged} {
		it := c.Equipped(slot)
		if it == nil {
			continue
		}
		if b, ok := lookup(it.Base); ok {
			out = append(out, c.attack(cls, *it, b, slot, worn))
		}
	}
	if len(out) == 0 {
		mod := c.Abilities.Mod(STR)
		out = append(out, Action{
			ID: "unarmed", Name: "Unarmed Strike", Kind: ActAttack, ToHit: mod + c.ProfBonus() + worn[StatToHit],
			Dice: Dice{Bonus: max(1+mod, 1)}, DamageType: "bludgeoning", RangeFt: 5, Description: "A punch or kick.",
			Weapon: true, Stat: STR,
		})
	}
	return out
}

// attack is one weapon's attack: STR for melee, DEX for ranged, the better
// for finesse; proficiency if trained; the weapon's own affixes on top.
func (c *Creature) attack(cls ClassTemplate, it Item, b BaseItem, slot Slot, worn map[AffixStat]int) Action {
	mod, stat := c.attackStat(b)
	toHit, dmg := mod+worn[StatToHit], mod
	if cls.Proficient(b) {
		toHit += c.ProfBonus()
	}
	for _, a := range it.Affixes {
		switch a.Stat {
		case StatToHit:
			toHit += a.Value
		case StatDamage:
			dmg += a.Value
		}
	}
	d := b.Dice
	d.Bonus += dmg
	rangeFt := 5
	if slot == SlotRanged {
		rangeFt = b.RangeFt
		if b.Thrown && !b.Ranged {
			rangeFt = thrownRange(b)
		}
	}
	return Action{
		ID: slug(b.Name), Name: it.Name, Kind: ActAttack, ToHit: toHit, Dice: d, DamageType: b.DamageType, RangeFt: rangeFt,
		Description: fmt.Sprintf("%+d to hit, %s %s", toHit, d, b.DamageType),
		Weapon:      true, Finesse: b.Finesse, Stat: stat,
	}
}

// attackStat is the ability a weapon attacks with: DEX for ranged weapons,
// the better of STR and DEX for finesse ones, STR otherwise.
func (c *Creature) attackStat(b BaseItem) (int, Ability) {
	str, dex := c.Abilities.Mod(STR), c.Abilities.Mod(DEX)
	if b.Ranged || (b.Finesse && dex > str) {
		return dex, DEX
	}
	return str, STR
}

// thrownRange reads the short range from a "thrown (20/60)" property.
func thrownRange(b BaseItem) int {
	for _, p := range b.Props {
		var short, long int
		if _, err := fmt.Sscanf(strings.TrimPrefix(p, "thrown "), "(%d/%d)", &short, &long); err == nil {
			return short
		}
	}
	return 20
}
