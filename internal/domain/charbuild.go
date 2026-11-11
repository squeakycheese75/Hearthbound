package domain

import (
	"fmt"
	"slices"
	"strings"
)

// StandardArray is the 5e standard ability array, assigned in a class's priority order.
var StandardArray = []int{15, 14, 13, 12, 10, 8}

// WeaponTemplate is a weapon a class starts with.
type WeaponTemplate struct {
	Name       string    `json:"name"`
	Dice       Dice      `json:"dice"`
	Abilities  []Ability `json:"abilities"`
	RangeFt    int       `json:"rangeFt"`
	DamageType string    `json:"damageType"`
}

// SpellTemplate is a spell a class knows, from MinLevel onward.
type SpellTemplate struct {
	Name        string     `json:"name"`
	Kind        ActionKind `json:"kind"`
	Dice        Dice       `json:"dice"`
	AddMod      bool       `json:"addMod"`
	SaveAbility Ability    `json:"saveAbility,omitempty"`
	HalfOnSave  bool       `json:"halfOnSave,omitempty"`
	RangeFt     int        `json:"rangeFt"`
	SlotLevel   int        `json:"slotLevel"`
	DamageType  string     `json:"damageType,omitempty"`
	MinLevel    int        `json:"minLevel"`
	Description string     `json:"description"`
	// Radius makes a save spell an area: everyone within it of the target saves.
	Radius int `json:"radius,omitempty"`
	// Upcast is the extra dice for each slot level above SlotLevel.
	Upcast Dice `json:"upcast,omitzero"`
	// Mass heals every wounded ally in range instead of one.
	Mass bool `json:"mass,omitempty"`
}

// ClassTemplate is the SRD data needed to build a character of a class.
type ClassTemplate struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	// Tactics tells an agent how this class fights.
	Tactics      string              `json:"tactics,omitempty"`
	HitDie       int                 `json:"hitDie"`
	Priority     []Ability           `json:"priority"`
	SaveProfs    []Ability           `json:"saveProfs"`
	Skills       []Skill             `json:"skills"`
	ArmorBase    int                 `json:"armorBase"`
	ArmorDexCap  int                 `json:"armorDexCap"`
	Shield       bool                `json:"shield"`
	UnarmoredCON bool                `json:"unarmoredCon"`
	Weapons      []WeaponTemplate    `json:"weapons"`
	CastAbility  Ability             `json:"castAbility,omitempty"`
	Spells       []SpellTemplate     `json:"spells,omitempty"`
	Slots        map[int]map[int]int `json:"slots,omitempty"`
	Gear         []string            `json:"gear"`
	// ArmourProf lists armour categories (light, medium, heavy) and "shield";
	// WeaponProf lists weapon categories (simple, martial) or catalogue keys.
	ArmourProf []string `json:"armourProf,omitempty"`
	WeaponProf []string `json:"weaponProf,omitempty"`
	// Features are the class's features by level.
	Features []FeatureTemplate `json:"-"`
}

// RaceTemplate is the SRD data for a playable race.
type RaceTemplate struct {
	Name    string          `json:"name"`
	Summary string          `json:"summary"`
	Bonuses map[Ability]int `json:"bonuses"`
	Speed   int             `json:"speed"`
	Skills  []Skill         `json:"skills,omitempty"`
}

// BuildSpec describes a character to create.
type BuildSpec struct {
	ID         string
	Name       string
	Level      int
	Controller Controller
	Persona    *Persona
	Portrait   string
	Gender     Gender
}

// BuildCharacter creates a ready-to-play character from SRD templates.
func BuildCharacter(cls ClassTemplate, race RaceTemplate, spec BuildSpec) (*Creature, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("%w: character name is required", ErrInvalid)
	}
	level := min(max(spec.Level, 1), MaxLevel)
	c := &Creature{
		ID: spec.ID, Name: strings.TrimSpace(spec.Name), Kind: KindPC, Controller: spec.Controller,
		Race: race.Name, Class: cls.Name, Level: level, Abilities: abilitiesFor(cls, race),
		Speed: race.Speed, SaveProfs: cls.SaveProfs, Persona: spec.Persona, Portrait: spec.Portrait,
		Gender:    spec.Gender,
		Inventory: append([]string(nil), cls.Gear...), Gold: 10,
	}
	c.SkillProfs = mergeSkills(cls.Skills, race.Skills)
	c.SyncFeatures(cls)
	c.MaxHP = hitPoints(cls, c)
	c.HP = c.MaxHP
	c.AC = armorClass(cls, c)
	c.Actions = append(weaponActions(cls, c), spellActions(cls, c)...)
	c.SpellSlots = cls.Slots[level]
	return c, nil
}

func abilitiesFor(cls ClassTemplate, race RaceTemplate) AbilityScores {
	scores := AbilityScores{}
	for i, a := range cls.Priority {
		if i < len(StandardArray) {
			scores[a] = StandardArray[i]
		}
	}
	for a, b := range race.Bonuses {
		scores[a] = scores.Score(a) + b
	}
	return scores
}

func mergeSkills(a, b []Skill) []Skill {
	out := append([]Skill(nil), a...)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func hitPoints(cls ClassTemplate, c *Creature) int {
	con := c.Abilities.Mod(CON)
	hp := cls.HitDie + con
	perLevel := cls.HitDie/2 + 1 + con
	hp += (c.Level - 1) * max(perLevel, 1)
	return max(hp, 1)
}

func armorClass(cls ClassTemplate, c *Creature) int {
	dex := c.Abilities.Mod(DEX)
	if cls.ArmorDexCap >= 0 {
		dex = min(dex, cls.ArmorDexCap)
	}
	ac := cls.ArmorBase + dex
	if cls.UnarmoredCON {
		ac += c.Abilities.Mod(CON)
	}
	if cls.Shield {
		ac += 2
	}
	return ac
}

// bestStat is the best of several abilities and its modifier.
func bestStat(c *Creature, options []Ability) (int, Ability) {
	best, stat := -10, STR
	for _, a := range options {
		if m := c.Abilities.Mod(a); m > best {
			best, stat = m, a
		}
	}
	return best, stat
}

func weaponActions(cls ClassTemplate, c *Creature) []Action {
	out := make([]Action, 0, len(cls.Weapons))
	for _, w := range cls.Weapons {
		mod, stat := bestStat(c, w.Abilities)
		d := w.Dice
		d.Bonus += mod
		out = append(out, Action{
			ID: slug(w.Name), Name: w.Name, Kind: ActAttack, ToHit: mod + c.ProfBonus(),
			Dice: d, DamageType: w.DamageType, RangeFt: w.RangeFt,
			Description: fmt.Sprintf("%+d to hit, %s %s", mod+c.ProfBonus(), d, w.DamageType),
			Weapon:      true, Finesse: len(w.Abilities) > 1, Stat: stat,
		})
	}
	return out
}

func spellActions(cls ClassTemplate, c *Creature) []Action {
	if cls.CastAbility == "" {
		return nil
	}
	mod := c.Abilities.Mod(cls.CastAbility)
	var out []Action
	for _, s := range cls.Spells {
		if c.Level < s.MinLevel {
			continue
		}
		d := s.Dice
		if s.AddMod {
			d.Bonus += mod
		}
		if s.SlotLevel == 0 && s.Kind != ActHeal && c.Level >= 5 {
			d.Count *= 2 // cantrips grow stronger at level 5
		}
		a := Action{
			ID: slug(s.Name), Name: s.Name, Kind: s.Kind, Dice: d, DamageType: s.DamageType,
			RangeFt: s.RangeFt, SlotLevel: s.SlotLevel, Description: s.Description,
			Radius: s.Radius, Upcast: s.Upcast, Mass: s.Mass,
		}
		switch s.Kind {
		case ActAttack:
			a.ToHit = mod + c.ProfBonus()
		case ActSave:
			a.SaveDC = 8 + mod + c.ProfBonus()
			a.SaveAbility = s.SaveAbility
			a.HalfOnSave = s.HalfOnSave
		}
		out = append(out, a)
	}
	return out
}

func slug(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
}

// DMStyle is a preset tone for the Dungeon Master agent.
type DMStyle struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Guidance    string `json:"guidance"`
}
