package domain

import (
	"fmt"
	"slices"
)

// FeatureKey identifies a class feature. The rules engine gives each its effect.
type FeatureKey string

// Class features (SRD 5.1, levels 1-3).
const (
	// Fighter.
	FeatSecondWind       FeatureKey = "second_wind"
	FeatDefenseStyle     FeatureKey = "defense_style"
	FeatActionSurge      FeatureKey = "action_surge"
	FeatImprovedCritical FeatureKey = "improved_critical"
	// Barbarian.
	FeatRage           FeatureKey = "rage"
	FeatRecklessAttack FeatureKey = "reckless_attack"
	FeatDangerSense    FeatureKey = "danger_sense"
	FeatFrenzy         FeatureKey = "frenzy"
	// Rogue.
	FeatSneakAttack   FeatureKey = "sneak_attack"
	FeatExpertise     FeatureKey = "expertise"
	FeatCunningAction FeatureKey = "cunning_action"
	FeatFastHands     FeatureKey = "fast_hands"
	// Ranger.
	FeatFavoredEnemy   FeatureKey = "favored_enemy"
	FeatArcheryStyle   FeatureKey = "archery_style"
	FeatColossusSlayer FeatureKey = "colossus_slayer"
	// Wizard.
	FeatArcaneRecovery FeatureKey = "arcane_recovery"
	FeatSculptSpells   FeatureKey = "sculpt_spells"
	// Cleric.
	FeatDiscipleOfLife  FeatureKey = "disciple_of_life"
	FeatChannelDivinity FeatureKey = "channel_divinity"
	// Bard.
	FeatBardicInspiration FeatureKey = "bardic_inspiration"
	FeatJackOfAllTrades   FeatureKey = "jack_of_all_trades"
	FeatSongOfRest        FeatureKey = "song_of_rest"
	FeatCuttingWords      FeatureKey = "cutting_words"
)

// Recharge is when a feature's uses come back.
type Recharge string

// Recharges.
const (
	RechargeShort Recharge = "short"
	RechargeLong  Recharge = "long"
)

// FeatureTemplate is the SRD description of a class feature.
type FeatureTemplate struct {
	Key         FeatureKey
	Name        string
	Level       int
	Description string
	// Uses by character level (the highest level at or below theirs counts);
	// UsesAbility instead makes uses equal that ability's modifier (at least 1).
	// No uses means the feature is always on.
	Uses        map[int]int
	UsesAbility Ability
	Recharge    Recharge
	// Prefer lists skills to pick for Expertise, best first.
	Prefer []Skill
}

// Feature is a class feature a character has.
type Feature struct {
	Key         FeatureKey `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Level       int        `json:"level"`
	Max         int        `json:"max,omitempty"`
	Used        int        `json:"used,omitempty"`
	Recharge    Recharge   `json:"recharge,omitempty"`
	// Skills are the skills Expertise doubles proficiency for.
	Skills []Skill `json:"skills,omitempty"`
}

// Left is how many uses remain (0 for always-on features).
func (f Feature) Left() int {
	return max(f.Max-f.Used, 0)
}

// Feature returns the character's feature, or nil.
func (c *Creature) Feature(key FeatureKey) *Feature {
	for i := range c.Features {
		if c.Features[i].Key == key {
			return &c.Features[i]
		}
	}
	return nil
}

// HasFeature reports whether the character has a feature.
func (c *Creature) HasFeature(key FeatureKey) bool {
	return c.Feature(key) != nil
}

// FeatureLeft is how many uses of a limited feature remain.
func (c *Creature) FeatureLeft(key FeatureKey) int {
	if f := c.Feature(key); f != nil {
		return f.Left()
	}
	return 0
}

// SpendFeature uses up one use of a feature.
func (c *Creature) SpendFeature(key FeatureKey) error {
	f := c.Feature(key)
	if f == nil || f.Left() <= 0 {
		return fmt.Errorf("%w: %s has no uses of %s left", ErrInvalid, c.Name, key)
	}
	f.Used++
	return nil
}

// RechargeFeatures restores uses after a rest: a short rest restores
// short-rest features, a long rest restores everything.
func (c *Creature) RechargeFeatures(r Recharge) {
	font := c.HasFeature(FeatFontOfInspiration)
	for i := range c.Features {
		f := &c.Features[i]
		if r == RechargeLong || f.Recharge == RechargeShort || (font && f.Key == FeatBardicInspiration) {
			f.Used = 0
		}
	}
}

// SyncFeatures gives the character the class features of their level,
// keeping how many uses they have spent.
func (c *Creature) SyncFeatures(cls ClassTemplate) {
	var out []Feature
	for _, t := range cls.Features {
		if t.Level > c.Level {
			continue
		}
		f := Feature{Key: t.Key, Name: t.Name, Description: t.Description, Level: t.Level, Recharge: t.Recharge, Max: t.uses(c)}
		if old := c.Feature(t.Key); old != nil {
			f.Used = min(old.Used, f.Max)
		}
		if t.Key == FeatExpertise {
			f.Skills = c.expertiseSkills(t.Prefer)
		}
		out = append(out, f)
	}
	c.Features = out
}

func (t FeatureTemplate) uses(c *Creature) int {
	if t.UsesAbility != "" {
		return max(c.Abilities.Mod(t.UsesAbility), 1)
	}
	n := 0
	for lvl, uses := range t.Uses {
		if lvl <= c.Level && lvl >= 0 {
			n = max(n, uses)
		}
	}
	return n
}

// expertiseSkills picks two proficient skills, preferred ones first.
func (c *Creature) expertiseSkills(prefer []Skill) []Skill {
	var out []Skill
	for _, s := range append(slices.Clone(prefer), c.SkillProfs...) {
		if len(out) == 2 {
			break
		}
		if c.SkillProficient(s) && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// Expert reports whether the character has Expertise in a skill.
func (c *Creature) Expert(s Skill) bool {
	f := c.Feature(FeatExpertise)
	return f != nil && slices.Contains(f.Skills, s)
}

// SneakDice is how many d6 a rogue's Sneak Attack adds.
func (c *Creature) SneakDice() int {
	if !c.HasFeature(FeatSneakAttack) {
		return 0
	}
	return (c.Level + 1) / 2
}

// More class features (levels 4-5).
const (
	FeatExtraAttack       FeatureKey = "extra_attack"
	FeatFastMovement      FeatureKey = "fast_movement"
	FeatUncannyDodge      FeatureKey = "uncanny_dodge"
	FeatFontOfInspiration FeatureKey = "font_of_inspiration"
	FeatDestroyUndead     FeatureKey = "destroy_undead"
)

// InspirationDie is the size of a bard's Bardic Inspiration die.
func (c *Creature) InspirationDie() int {
	if c.Level >= 5 {
		return 8
	}
	return 6
}
