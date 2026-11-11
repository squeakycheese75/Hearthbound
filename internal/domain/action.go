package domain

// ActionKind says how an action is resolved by the rules engine.
type ActionKind string

// Action kinds.
const (
	// ActAttack makes an attack roll against AC (weapons, spell attacks).
	ActAttack ActionKind = "attack"
	// ActSave forces targets to make a saving throw against SaveDC.
	ActSave ActionKind = "save"
	// ActAutoHit deals damage with no roll to hit (e.g. Magic Missile).
	ActAutoHit ActionKind = "auto_hit"
	// ActHeal restores hit points to an ally.
	ActHeal ActionKind = "heal"
	// ActDodge imposes disadvantage on attacks against the user until its next turn.
	ActDodge ActionKind = "dodge"
)

// Action is something a creature can do on its turn: a weapon attack or a spell.
type Action struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Kind        ActionKind `json:"kind"`
	ToHit       int        `json:"toHit,omitempty"`
	SaveDC      int        `json:"saveDc,omitempty"`
	SaveAbility Ability    `json:"saveAbility,omitempty"`
	HalfOnSave  bool       `json:"halfOnSave,omitempty"`
	Dice        Dice       `json:"dice"`
	DamageType  string     `json:"damageType,omitempty"`
	RangeFt     int        `json:"rangeFt"`
	SlotLevel   int        `json:"slotLevel,omitempty"`
	Description string     `json:"description,omitempty"`
	// Weapon marks a weapon attack; Finesse a finesse weapon; Stat the
	// ability it attacks with. Class features (Rage, Sneak Attack) read these.
	Weapon  bool    `json:"weapon,omitempty"`
	Finesse bool    `json:"finesse,omitempty"`
	Stat    Ability `json:"stat,omitempty"`
	// Radius, Upcast and Mass: see SpellTemplate.
	Radius int  `json:"radius,omitempty"`
	Upcast Dice `json:"upcast,omitzero"`
	Mass   bool `json:"mass,omitempty"`
}

// TargetsAllies reports whether the action is aimed at friends rather than foes.
func (a Action) TargetsAllies() bool {
	return a.Kind == ActHeal
}

// NeedsTarget reports whether the action needs a target creature.
func (a Action) NeedsTarget() bool {
	return a.Kind != ActDodge && !a.Mass
}

// Ranged reports whether the action reaches beyond 5 feet.
func (a Action) Ranged() bool {
	return a.RangeFt > 5
}
