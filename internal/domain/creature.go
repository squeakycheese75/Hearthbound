package domain

import (
	"fmt"
	"slices"
)

// Kind separates player characters from monsters.
type Kind string

// Creature kinds.
const (
	KindPC      Kind = "pc"
	KindMonster Kind = "monster"
)

// Controller says who decides a creature's actions.
type Controller string

// Controllers.
const (
	ControlHuman Controller = "human"
	ControlAI    Controller = "ai"
	ControlDM    Controller = "dm"
)

// Condition is a 5e condition (subset) or a transient combat state.
type Condition string

// Supported conditions.
const (
	Blinded     Condition = "blinded"
	Frightened  Condition = "frightened"
	Poisoned    Condition = "poisoned"
	Prone       Condition = "prone"
	Restrained  Condition = "restrained"
	Unconscious Condition = "unconscious"
	Dodging     Condition = "dodging"
	// Class feature states.
	Raging    Condition = "raging"
	Frenzied  Condition = "frenzied"
	Reckless  Condition = "reckless"
	Hidden    Condition = "hidden"
	Exhausted Condition = "exhausted"
	Turned    Condition = "turned"
)

// Position is a square on the battle grid. One square is 5 feet.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// DistanceFt returns the 5e grid distance in feet (diagonals count as 5ft).
func (p Position) DistanceFt(q Position) int {
	dx := abs(p.X - q.X)
	dy := abs(p.Y - q.Y)
	return max(dx, dy) * 5
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// DeathSaves tracks a dying character's saving throws.
type DeathSaves struct {
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

// Persona shapes how an AI companion plays its character.
type Persona struct {
	Archetype   string `json:"archetype"`
	Personality string `json:"personality"`
	Backstory   string `json:"backstory"`
	Voice       string `json:"voice"`
	Bravery     int    `json:"bravery"`
	Chattiness  int    `json:"chattiness"`
	Loyalty     int    `json:"loyalty"`
}

// Creature is any combatant: a player character or a monster.
type Creature struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId,omitempty"`
	// PlayerID is a friend playing this character in an adventure; empty
	// means the owner plays it. Turns, billing and covering follow the player;
	// the sheet still settles to the owner.
	PlayerID   string        `json:"playerId,omitempty"`
	Name       string        `json:"name"`
	Kind       Kind          `json:"kind"`
	Controller Controller    `json:"controller"`
	Race       string        `json:"race,omitempty"`
	Gender     Gender        `json:"gender,omitempty"`
	Class      string        `json:"class,omitempty"`
	Level      int           `json:"level"`
	XP         int           `json:"xp"`
	Abilities  AbilityScores `json:"abilities"`
	MaxHP      int           `json:"maxHp"`
	HP         int           `json:"hp"`
	AC         int           `json:"ac"`
	Speed      int           `json:"speed"`
	SaveProfs  []Ability     `json:"saveProfs,omitempty"`
	SkillProfs []Skill       `json:"skillProfs,omitempty"`
	Actions    []Action      `json:"actions"`
	SpellSlots map[int]int   `json:"spellSlots,omitempty"`
	SlotsUsed  map[int]int   `json:"slotsUsed,omitempty"`
	Conditions []Condition   `json:"conditions,omitempty"`
	DeathSaves DeathSaves    `json:"deathSaves"`
	Stable     bool          `json:"stable,omitempty"`
	Dead       bool          `json:"dead,omitempty"`
	Pos        Position      `json:"pos"`
	Persona    *Persona      `json:"persona,omitempty"`
	CR         string        `json:"cr,omitempty"`
	Tactic     Tactic        `json:"tactic,omitempty"`
	XPValue    int           `json:"xpValue,omitempty"`
	Portrait   string        `json:"portrait,omitempty"`
	// Type is a monster's creature type, where it matters (undead).
	Type string `json:"type,omitempty"`
	// Features are the character's class features.
	Features []Feature `json:"features,omitempty"`
	// Inspiration is a Bardic Inspiration die (its size) waiting to be used.
	Inspiration int `json:"inspiration,omitempty"`
	// ASI counts Ability Score Improvements earned but not yet chosen.
	ASI int `json:"asi,omitempty"`
	// Inventory is the old free-text inventory, converted to Items on load.
	Inventory []string `json:"inventory,omitempty"`
	// Items are the things the character carries.
	Items []Item `json:"items,omitempty"`
	// BaseAbilities and BaseMaxHP are the character's own values before any
	// equipment bonuses; Recalculate derives Abilities and MaxHP from them.
	BaseAbilities AbilityScores `json:"baseAbilities,omitempty"`
	BaseMaxHP     int           `json:"baseMaxHp,omitempty"`
	// Outfitted is set once the character's class weapons are items and their
	// gear has been equipped.
	Outfitted bool `json:"outfitted,omitempty"`
	Gold      int  `json:"gold"`
}

// ProfBonus returns the creature's proficiency bonus.
func (c *Creature) ProfBonus() int {
	return ProficiencyBonus(c.Level)
}

// Has reports whether the creature has a condition.
func (c *Creature) Has(cond Condition) bool {
	return slices.Contains(c.Conditions, cond)
}

// AddCondition adds cond if not already present.
func (c *Creature) AddCondition(cond Condition) {
	if c.Has(cond) {
		return
	}
	c.Conditions = append(c.Conditions, cond)
}

// RemoveCondition removes cond if present.
func (c *Creature) RemoveCondition(cond Condition) {
	c.Conditions = slices.DeleteFunc(c.Conditions, func(x Condition) bool { return x == cond })
}

// Down reports whether the creature is at 0 HP or dead.
func (c *Creature) Down() bool {
	return c.Dead || c.HP <= 0
}

// CanAct reports whether the creature can take actions. A turned creature
// spends its turns fleeing.
func (c *Creature) CanAct() bool {
	return !c.Down() && !c.Has(Unconscious) && !c.Has(Turned)
}

// SaveProficient reports proficiency in saves of ability a.
func (c *Creature) SaveProficient(a Ability) bool {
	return slices.Contains(c.SaveProfs, a)
}

// SkillProficient reports proficiency in skill s.
func (c *Creature) SkillProficient(s Skill) bool {
	return slices.Contains(c.SkillProfs, s)
}

// Action returns the action with the given ID.
func (c *Creature) Action(id string) (Action, error) {
	for _, a := range c.Actions {
		if a.ID == id {
			return a, nil
		}
	}
	return Action{}, fmt.Errorf("%w: action %q for %s", ErrNotFound, id, c.Name)
}

// SlotsLeft returns remaining spell slots of a level.
func (c *Creature) SlotsLeft(level int) int {
	return c.SpellSlots[level] - c.SlotsUsed[level]
}

// SlotFor is the lowest slot level at or above level with a slot left (0 if none).
func (c *Creature) SlotFor(level int) int {
	for l := level; l <= 9; l++ {
		if c.SlotsLeft(l) > 0 {
			return l
		}
	}
	return 0
}

// UseSlot spends the lowest available slot at or above level (casting a
// spell with a higher slot if its own are spent) and returns its level.
func (c *Creature) UseSlot(level int) (int, error) {
	used := c.SlotFor(level)
	if used == 0 {
		return 0, fmt.Errorf("%w: %s has no level %d slots left", ErrInvalid, c.Name, level)
	}
	if c.SlotsUsed == nil {
		c.SlotsUsed = map[int]int{}
	}
	c.SlotsUsed[used]++
	return used, nil
}

// RestInTown is the full recovery a surviving hero gets between adventures:
// hit points, spell slots and death saves restored, conditions cleared.
func (c *Creature) RestInTown() {
	c.HP = c.MaxHP
	c.SlotsUsed = nil
	c.Conditions = nil
	c.DeathSaves = DeathSaves{}
	c.Stable = false
	c.Inspiration = 0
	c.RechargeFeatures(RechargeLong)
}

// Tactic is how a monster fights; the engine's monster AI follows it.
type Tactic string

// Monster fighting styles.
const (
	// TacticBrute charges the nearest foe and hits hard in melee.
	TacticBrute Tactic = "brute"
	// TacticSkirmisher keeps its distance, shooting the wounded.
	TacticSkirmisher Tactic = "skirmisher"
	// TacticPack gangs up on whoever its allies are already attacking.
	TacticPack Tactic = "pack"
	// TacticCunning goes for soft targets: low armour, low hit points.
	TacticCunning Tactic = "cunning"
	// TacticMindless attacks whatever is closest.
	TacticMindless Tactic = "mindless"
)
