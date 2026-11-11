// Package srd provides game content derived from the D&D 5e System Reference
// Document 5.1 (CC BY 4.0, Wizards of the Coast), plus original persona presets.
package srd

import d "dnd/internal/domain"

var (
	simpleMelee = []d.Ability{d.STR}
	finesse     = []d.Ability{d.STR, d.DEX}
	ranged      = []d.Ability{d.DEX}
)

func fullCasterSlots() map[int]map[int]int {
	return map[int]map[int]int{1: {1: 2}, 2: {1: 3}, 3: {1: 4, 2: 2}, 4: {1: 4, 2: 3}, 5: {1: 4, 2: 3, 3: 2}}
}

var classes = []d.ClassTemplate{
	{
		Name: "Fighter", Summary: "A master of weapons and armour who holds the front line.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatDefenseStyle, Name: "Fighting Style: Defense", Level: 1, Description: "+1 AC while wearing armour."},
			{Key: d.FeatSecondWind, Name: "Second Wind", Level: 1, Uses: map[int]int{1: 1}, Recharge: d.RechargeShort, Description: "Bonus action: regain 1d10 + your level in hit points."},
			{Key: d.FeatActionSurge, Name: "Action Surge", Level: 2, Uses: map[int]int{2: 1}, Recharge: d.RechargeShort, Description: "Take one additional action on your turn."},
			{Key: d.FeatImprovedCritical, Name: "Improved Critical", Level: 3, Description: "Champion: your weapon attacks score a critical hit on a roll of 19 or 20."},
			{Key: d.FeatExtraAttack, Name: "Extra Attack", Level: 5, Description: "Attack twice when you take the Attack action."},
		},
		Tactics: "Front line. Step between the enemy and your softer allies, engage the biggest threat in melee, and keep foes off the wizard. Use the crossbow only when you cannot reach anyone. Second Wind heals you when hurt; Action Surge gives you a second attack in a crisis.",
		HitDie:  10, Priority: []d.Ability{d.STR, d.CON, d.DEX, d.WIS, d.CHA, d.INT},
		SaveProfs: []d.Ability{d.STR, d.CON}, Skills: []d.Skill{d.Athletics, d.Perception},
		ArmorBase: 16, ArmorDexCap: 0, Shield: true,
		Weapons: []d.WeaponTemplate{
			{Name: "Longsword", Dice: d.MustDice("1d8"), Abilities: simpleMelee, RangeFt: 5, DamageType: "slashing"},
			{Name: "Light Crossbow", Dice: d.MustDice("1d8"), Abilities: ranged, RangeFt: 80, DamageType: "piercing"},
		},
		Gear:       []string{"chain mail", "shield", "explorer's pack", "rations (5)"},
		ArmourProf: []string{"light", "medium", "heavy", "shield"},
		WeaponProf: []string{"simple", "martial"},
	},
	{
		Name: "Barbarian", Summary: "A furious warrior who shrugs off blows that would fell others.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatRage, Name: "Rage", Level: 1, Uses: map[int]int{1: 2, 3: 3}, Recharge: d.RechargeLong, Description: "Bonus action: +2 damage on Strength melee attacks and resistance to bludgeoning, piercing and slashing damage until the fight ends."},
			{Key: d.FeatRecklessAttack, Name: "Reckless Attack", Level: 2, Description: "Attack with advantage on Strength melee attacks this turn; attacks against you have advantage until your next turn."},
			{Key: d.FeatDangerSense, Name: "Danger Sense", Level: 2, Description: "Advantage on Dexterity saving throws against effects you can see."},
			{Key: d.FeatFrenzy, Name: "Frenzy", Level: 3, Description: "Berserker: you can rage in a frenzy, making a melee weapon attack as a bonus action on each later turn. Afterwards you are exhausted until a long rest."},
			{Key: d.FeatExtraAttack, Name: "Extra Attack", Level: 5, Description: "Attack twice when you take the Attack action."},
			{Key: d.FeatFastMovement, Name: "Fast Movement", Level: 5, Description: "Your speed increases by 10 feet."},
		},
		Tactics: "Front line shock trooper. Charge the most dangerous foe and hit hard with the greataxe; you can take the hits others cannot. Throw javelins only when no one is in reach. Rage at the start of a fight; attack recklessly when you can afford to be hit.",
		HitDie:  12, Priority: []d.Ability{d.STR, d.CON, d.DEX, d.WIS, d.CHA, d.INT},
		SaveProfs: []d.Ability{d.STR, d.CON}, Skills: []d.Skill{d.Athletics, d.Survival},
		ArmorBase: 10, ArmorDexCap: -1, UnarmoredCON: true,
		Weapons: []d.WeaponTemplate{
			{Name: "Greataxe", Dice: d.MustDice("1d12"), Abilities: simpleMelee, RangeFt: 5, DamageType: "slashing"},
			{Name: "Javelin", Dice: d.MustDice("1d6"), Abilities: simpleMelee, RangeFt: 30, DamageType: "piercing"},
		},
		Gear:       []string{"explorer's pack", "javelins (4)", "rations (5)"},
		ArmourProf: []string{"light", "medium", "shield"},
		WeaponProf: []string{"simple", "martial"},
	},
	{
		Name: "Rogue", Summary: "A nimble trickster who strikes from the shadows.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatExpertise, Name: "Expertise", Level: 1, Prefer: []d.Skill{d.Stealth, d.Perception, d.SleightOfHand, d.Acrobatics}, Description: "Double your proficiency bonus for two of your skills."},
			{Key: d.FeatSneakAttack, Name: "Sneak Attack", Level: 1, Description: "Once per turn, deal extra damage (1d6, 2d6 at level 3, 3d6 at level 5) with a finesse or ranged weapon when you have advantage, or when an ally is next to your target."},
			{Key: d.FeatCunningAction, Name: "Cunning Action", Level: 2, Description: "Bonus action: Dash, or Hide (a Stealth check; while hidden, your next attack has advantage)."},
			{Key: d.FeatFastHands, Name: "Fast Hands", Level: 3, Description: "Thief: use an object, pick a lock or pick a pocket as a bonus action, and climb with ease."},
			{Key: d.FeatUncannyDodge, Name: "Uncanny Dodge", Level: 5, Description: "Reaction: halve the damage of an attack that hits you."},
		},
		Tactics: "Skirmisher, not a front-liner. Stay at range with the shortbow and pick off wounded or isolated foes; only close to melee to finish a foe that is already engaged by an ally. Sneak Attack lands when you have advantage or an ally is next to your target, so hide after attacking.",
		HitDie:  8, Priority: []d.Ability{d.DEX, d.CON, d.WIS, d.INT, d.CHA, d.STR},
		SaveProfs: []d.Ability{d.DEX, d.INT}, Skills: []d.Skill{d.Stealth, d.SleightOfHand, d.Perception, d.Deception},
		ArmorBase: 11, ArmorDexCap: -1,
		Weapons: []d.WeaponTemplate{
			{Name: "Rapier", Dice: d.MustDice("1d8"), Abilities: finesse, RangeFt: 5, DamageType: "piercing"},
			{Name: "Shortbow", Dice: d.MustDice("1d6"), Abilities: ranged, RangeFt: 80, DamageType: "piercing"},
		},
		Gear:       []string{"leather armour", "thieves' tools", "burglar's pack"},
		ArmourProf: []string{"light"},
		WeaponProf: []string{"simple", "longsword", "rapier", "shortsword"},
	},
	{
		Name: "Ranger", Summary: "A wilderness hunter, deadly with a bow.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatFavoredEnemy, Name: "Favored Enemy & Natural Explorer", Level: 1, Description: "Advantage on Survival checks to track your favoured enemies and Intelligence checks to recall lore about them; you travel and forage well in the wild."},
			{Key: d.FeatArcheryStyle, Name: "Fighting Style: Archery", Level: 2, Description: "+2 to hit with ranged weapon attacks."},
			{Key: d.FeatColossusSlayer, Name: "Colossus Slayer", Level: 3, Description: "Hunter: once per turn, a weapon hit deals an extra 1d8 to a creature that is already wounded."},
			{Key: d.FeatExtraAttack, Name: "Extra Attack", Level: 5, Description: "Attack twice when you take the Attack action."},
		},
		Tactics: "Archer. Keep your distance and shoot with the longbow, focusing the most dangerous target. Fight in melee only if a foe is already on you; heal an ally who is down (from level 2).",
		HitDie:  10, Priority: []d.Ability{d.DEX, d.WIS, d.CON, d.STR, d.INT, d.CHA},
		SaveProfs: []d.Ability{d.STR, d.DEX}, Skills: []d.Skill{d.Survival, d.Nature, d.Stealth},
		ArmorBase: 14, ArmorDexCap: 2,
		Weapons: []d.WeaponTemplate{
			{Name: "Longbow", Dice: d.MustDice("1d8"), Abilities: ranged, RangeFt: 150, DamageType: "piercing"},
			{Name: "Shortsword", Dice: d.MustDice("1d6"), Abilities: finesse, RangeFt: 5, DamageType: "piercing"},
		},
		CastAbility: d.WIS,
		Spells: []d.SpellTemplate{
			{Name: "Cure Wounds", Kind: d.ActHeal, Dice: d.MustDice("1d8"), AddMod: true, RangeFt: 5, SlotLevel: 1, Upcast: d.MustDice("1d8"), MinLevel: 2, Description: "Touch an ally to heal 1d8 + WIS."},
		},
		Slots:      map[int]map[int]int{2: {1: 2}, 3: {1: 3}, 4: {1: 3}, 5: {1: 4, 2: 2}},
		Gear:       []string{"scale mail", "quiver of arrows (20)", "explorer's pack"},
		ArmourProf: []string{"light", "medium", "shield"},
		WeaponProf: []string{"simple", "martial"},
	},
	{
		Name: "Wizard", Summary: "A scholar of the arcane who hurls fire and force.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatArcaneRecovery, Name: "Arcane Recovery", Level: 1, Uses: map[int]int{1: 1}, Recharge: d.RechargeLong, Description: "Once a day, during a short rest, recover spent spell slots (total levels up to half your wizard level, rounded up)."},
			{Key: d.FeatSculptSpells, Name: "Evocation: Sculpt Spells", Level: 2, Description: "Evoker: shape your area spells so chosen allies are untouched by them."},
		},
		Tactics: "Fragile artillery. Never walk into melee. Stay at the back and cast from range: Magic Missile to finish foes, Burning Hands or Shatter for groups, cantrips otherwise. Save spell slots when a cantrip will do.",
		HitDie:  6, Priority: []d.Ability{d.INT, d.CON, d.DEX, d.WIS, d.CHA, d.STR},
		SaveProfs: []d.Ability{d.INT, d.WIS}, Skills: []d.Skill{d.Arcana, d.History, d.Investigation},
		ArmorBase: 10, ArmorDexCap: -1,
		Weapons: []d.WeaponTemplate{
			{Name: "Quarterstaff", Dice: d.MustDice("1d6"), Abilities: simpleMelee, RangeFt: 5, DamageType: "bludgeoning"},
		},
		CastAbility: d.INT,
		Spells: []d.SpellTemplate{
			{Name: "Fire Bolt", Kind: d.ActAttack, Dice: d.MustDice("1d10"), RangeFt: 120, DamageType: "fire", MinLevel: 1, Description: "Cantrip: hurl a mote of fire."},
			{Name: "Ray of Frost", Kind: d.ActAttack, Dice: d.MustDice("1d8"), RangeFt: 60, DamageType: "cold", MinLevel: 1, Description: "Cantrip: a frigid beam."},
			{Name: "Magic Missile", Kind: d.ActAutoHit, Dice: d.MustDice("3d4+3"), RangeFt: 120, SlotLevel: 1, DamageType: "force", MinLevel: 1, Upcast: d.MustDice("1d4+1"), Description: "Three darts of force that never miss (one more per higher slot)."},
			{Name: "Burning Hands", Kind: d.ActSave, Dice: d.MustDice("3d6"), SaveAbility: d.DEX, HalfOnSave: true, RangeFt: 15, SlotLevel: 1, DamageType: "fire", MinLevel: 1, Upcast: d.MustDice("1d6"), Description: "A sheet of flame (DEX save for half)."},
			{Name: "Shatter", Kind: d.ActSave, Dice: d.MustDice("3d8"), SaveAbility: d.CON, HalfOnSave: true, RangeFt: 60, SlotLevel: 2, DamageType: "thunder", MinLevel: 3, Radius: 10, Upcast: d.MustDice("1d8"), Description: "A painful ringing burst: everyone within 10 ft of the target (CON save for half)."},
			{Name: "Fireball", Kind: d.ActSave, Dice: d.MustDice("8d6"), SaveAbility: d.DEX, HalfOnSave: true, RangeFt: 150, SlotLevel: 3, DamageType: "fire", MinLevel: 5, Radius: 20, Upcast: d.MustDice("1d6"), Description: "A bead of flame blossoms into a roaring blast: everyone within 20 ft of the target, friend or foe (DEX save for half)."},
		},
		Slots:      fullCasterSlots(),
		Gear:       []string{"spellbook", "arcane focus", "scholar's pack"},
		WeaponProf: []string{"dagger", "quarterstaff", "light-crossbow"},
	},
	{
		Name: "Cleric", Summary: "A divine champion who heals allies and smites the wicked.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatDiscipleOfLife, Name: "Disciple of Life", Level: 1, Description: "Life Domain: healing spells of 1st level or higher restore an extra 2 + the spell's level."},
			{Key: d.FeatChannelDivinity, Name: "Channel Divinity", Level: 2, Uses: map[int]int{2: 1}, Recharge: d.RechargeShort, Description: "Action: Turn Undead (undead nearby must save or flee) or Preserve Life (share out 5 x your level in healing among badly hurt allies nearby)."},
			{Key: d.FeatDestroyUndead, Name: "Destroy Undead", Level: 5, Description: "Undead of challenge 1/2 or lower that fail against your Turn Undead are destroyed."},
		},
		Tactics: "Support and second line. Keep allies standing: Healing Word at range for anyone at 0 HP comes first, Cure Wounds by touch. Otherwise Sacred Flame or Guiding Bolt from range; you can hold the line with your mace if needed.",
		HitDie:  8, Priority: []d.Ability{d.WIS, d.CON, d.STR, d.CHA, d.DEX, d.INT},
		SaveProfs: []d.Ability{d.WIS, d.CHA}, Skills: []d.Skill{d.Medicine, d.Religion, d.Insight},
		ArmorBase: 14, ArmorDexCap: 2, Shield: true,
		Weapons: []d.WeaponTemplate{
			{Name: "Mace", Dice: d.MustDice("1d6"), Abilities: simpleMelee, RangeFt: 5, DamageType: "bludgeoning"},
		},
		CastAbility: d.WIS,
		Spells: []d.SpellTemplate{
			{Name: "Sacred Flame", Kind: d.ActSave, Dice: d.MustDice("1d8"), SaveAbility: d.DEX, RangeFt: 60, DamageType: "radiant", MinLevel: 1, Description: "Cantrip: radiant flame descends (DEX save)."},
			{Name: "Guiding Bolt", Kind: d.ActAttack, Dice: d.MustDice("4d6"), RangeFt: 120, SlotLevel: 1, DamageType: "radiant", MinLevel: 1, Upcast: d.MustDice("1d6"), Description: "A flash of light streaks toward a foe."},
			{Name: "Cure Wounds", Kind: d.ActHeal, Dice: d.MustDice("1d8"), AddMod: true, RangeFt: 5, SlotLevel: 1, Upcast: d.MustDice("1d8"), MinLevel: 1, Description: "Touch an ally to heal 1d8 + WIS."},
			{Name: "Healing Word", Kind: d.ActHeal, Dice: d.MustDice("1d4"), AddMod: true, RangeFt: 60, SlotLevel: 1, MinLevel: 1, Upcast: d.MustDice("1d4"), Description: "A word of power heals an ally at range."},
			{Name: "Spiritual Weapon", Kind: d.ActAttack, Dice: d.MustDice("1d8"), AddMod: true, RangeFt: 60, SlotLevel: 2, DamageType: "force", MinLevel: 3, Description: "A floating spectral weapon strikes."},
			{Name: "Mass Healing Word", Kind: d.ActHeal, Dice: d.MustDice("1d4"), AddMod: true, RangeFt: 60, SlotLevel: 3, MinLevel: 5, Mass: true, Upcast: d.MustDice("1d4"), Description: "Words of restoration heal every wounded ally within 60 ft."},
		},
		Slots:      fullCasterSlots(),
		Gear:       []string{"scale mail", "shield", "holy symbol", "priest's pack"},
		ArmourProf: []string{"light", "medium", "shield"},
		WeaponProf: []string{"simple"},
	},
	{
		Name: "Bard", Summary: "A silver-tongued performer whose magic inspires and wounds.",
		Features: []d.FeatureTemplate{
			{Key: d.FeatBardicInspiration, Name: "Bardic Inspiration", Level: 1, UsesAbility: d.CHA, Recharge: d.RechargeLong, Description: "Bonus action: give an ally a d6 (d8 from level 5) to add to an attack roll, check or save that would otherwise fail."},
			{Key: d.FeatJackOfAllTrades, Name: "Jack of All Trades", Level: 2, Description: "Add half your proficiency bonus to checks you are not proficient in (including initiative)."},
			{Key: d.FeatSongOfRest, Name: "Song of Rest", Level: 2, Description: "Allies who rest with you regain an extra 1d6 hit points on a short rest."},
			{Key: d.FeatCuttingWords, Name: "Cutting Words", Level: 3, Description: "College of Lore, reaction: spend a Bardic Inspiration to subtract the die from a foe's attack roll against an ally."},
			{Key: d.FeatExpertise, Name: "Expertise", Level: 3, Prefer: []d.Skill{d.Persuasion, d.Deception, d.Performance, d.Insight}, Description: "Double your proficiency bonus for two of your skills."},
			{Key: d.FeatFontOfInspiration, Name: "Font of Inspiration", Level: 5, Description: "Your Bardic Inspiration (now a d8) comes back on a short rest."},
		},
		Tactics: "Support from range. Stay behind the front line, use Vicious Mockery on the most dangerous foe and heal fallen allies. Avoid melee unless cornered.",
		HitDie:  8, Priority: []d.Ability{d.CHA, d.DEX, d.CON, d.WIS, d.INT, d.STR},
		SaveProfs: []d.Ability{d.DEX, d.CHA}, Skills: []d.Skill{d.Performance, d.Persuasion, d.Deception, d.Insight},
		ArmorBase: 11, ArmorDexCap: -1,
		Weapons: []d.WeaponTemplate{
			{Name: "Rapier", Dice: d.MustDice("1d8"), Abilities: finesse, RangeFt: 5, DamageType: "piercing"},
		},
		CastAbility: d.CHA,
		Spells: []d.SpellTemplate{
			{Name: "Vicious Mockery", Kind: d.ActSave, Dice: d.MustDice("1d4"), SaveAbility: d.WIS, RangeFt: 60, DamageType: "psychic", MinLevel: 1, Description: "Cantrip: an insult laced with magic (WIS save)."},
			{Name: "Healing Word", Kind: d.ActHeal, Dice: d.MustDice("1d4"), AddMod: true, RangeFt: 60, SlotLevel: 1, MinLevel: 1, Upcast: d.MustDice("1d4"), Description: "A word of power heals an ally at range."},
			{Name: "Dissonant Whispers", Kind: d.ActSave, Dice: d.MustDice("3d6"), SaveAbility: d.WIS, HalfOnSave: true, RangeFt: 60, SlotLevel: 1, DamageType: "psychic", MinLevel: 1, Upcast: d.MustDice("1d6"), Description: "A maddening melody (WIS save for half)."},
			{Name: "Shatter", Kind: d.ActSave, Dice: d.MustDice("3d8"), SaveAbility: d.CON, HalfOnSave: true, RangeFt: 60, SlotLevel: 2, DamageType: "thunder", MinLevel: 3, Radius: 10, Upcast: d.MustDice("1d8"), Description: "A painful ringing burst: everyone within 10 ft of the target (CON save for half)."},
		},
		Slots:      fullCasterSlots(),
		Gear:       []string{"lute", "leather armour", "entertainer's pack"},
		ArmourProf: []string{"light"},
		WeaponProf: []string{"simple", "longsword", "rapier", "shortsword"},
	},
}

var races = []d.RaceTemplate{
	{Name: "Human", Summary: "Adaptable and ambitious.", Bonuses: map[d.Ability]int{d.STR: 1, d.DEX: 1, d.CON: 1, d.INT: 1, d.WIS: 1, d.CHA: 1}, Speed: 30},
	{Name: "Elf", Summary: "Graceful, keen-eyed and long-lived.", Bonuses: map[d.Ability]int{d.DEX: 2, d.INT: 1}, Speed: 30, Skills: []d.Skill{d.Perception}},
	{Name: "Dwarf", Summary: "Stout, stubborn and hard to kill.", Bonuses: map[d.Ability]int{d.CON: 2, d.WIS: 1}, Speed: 25},
	{Name: "Halfling", Summary: "Small, brave and uncommonly lucky.", Bonuses: map[d.Ability]int{d.DEX: 2, d.CHA: 1}, Speed: 25},
	{Name: "Half-Orc", Summary: "Fierce and relentless.", Bonuses: map[d.Ability]int{d.STR: 2, d.CON: 1}, Speed: 30, Skills: []d.Skill{d.Intimidation}},
	{Name: "Tiefling", Summary: "Touched by infernal heritage.", Bonuses: map[d.Ability]int{d.CHA: 2, d.INT: 1}, Speed: 30},
	{Name: "Dragonborn", Summary: "Proud descendants of dragons.", Bonuses: map[d.Ability]int{d.STR: 2, d.CHA: 1}, Speed: 30},
	{Name: "Gnome", Summary: "Curious, clever tinkerers.", Bonuses: map[d.Ability]int{d.INT: 2, d.CON: 1}, Speed: 25},
	{Name: "Half-Elf", Summary: "Charming wanderers between two worlds.", Bonuses: map[d.Ability]int{d.CHA: 2, d.DEX: 1, d.CON: 1}, Speed: 30},
}
