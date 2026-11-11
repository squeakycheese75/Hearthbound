package srd

import d "dnd/internal/domain"

type monsterStat struct {
	name    string
	cr      string
	xp      int
	ac, hp  int
	speed   int
	scores  [6]int
	attacks []d.Action
}

func melee(name, dice, dmgType string, toHit int) d.Action {
	return d.Action{ID: slugify(name), Name: name, Kind: d.ActAttack, ToHit: toHit, Dice: d.MustDice(dice), DamageType: dmgType, RangeFt: 5}
}

func shoot(name, dice, dmgType string, toHit, rangeFt int) d.Action {
	a := melee(name, dice, dmgType, toHit)
	a.RangeFt = rangeFt
	return a
}

// Stat blocks from the SRD 5.1, simplified: no riders, and Multiattack is two
// attacks (see multiattack).
var monsters = map[string]monsterStat{
	"kobold":         {"Kobold", "1/8", 25, 12, 5, 30, [6]int{7, 15, 9, 8, 7, 8}, []d.Action{melee("Dagger", "1d4+2", "piercing", 4), shoot("Sling", "1d4+2", "bludgeoning", 4, 30)}},
	"giant_rat":      {"Giant Rat", "1/8", 25, 12, 7, 30, [6]int{7, 15, 11, 2, 10, 4}, []d.Action{melee("Bite", "1d4+2", "piercing", 4)}},
	"bandit":         {"Bandit", "1/8", 25, 12, 11, 30, [6]int{11, 12, 12, 10, 10, 10}, []d.Action{melee("Scimitar", "1d6+1", "slashing", 3), shoot("Light Crossbow", "1d8+1", "piercing", 3, 80)}},
	"cultist":        {"Cultist", "1/8", 25, 12, 9, 30, [6]int{11, 12, 10, 10, 11, 10}, []d.Action{melee("Scimitar", "1d6+1", "slashing", 3)}},
	"goblin":         {"Goblin", "1/4", 50, 15, 7, 30, [6]int{8, 14, 10, 10, 8, 8}, []d.Action{melee("Scimitar", "1d6+2", "slashing", 4), shoot("Shortbow", "1d6+2", "piercing", 4, 80)}},
	"skeleton":       {"Skeleton", "1/4", 50, 13, 13, 30, [6]int{10, 14, 15, 6, 8, 5}, []d.Action{melee("Shortsword", "1d6+2", "piercing", 4), shoot("Shortbow", "1d6+2", "piercing", 4, 80)}},
	"zombie":         {"Zombie", "1/4", 50, 8, 22, 20, [6]int{13, 6, 16, 3, 6, 5}, []d.Action{melee("Slam", "1d6+1", "bludgeoning", 3)}},
	"wolf":           {"Wolf", "1/4", 50, 13, 11, 40, [6]int{12, 15, 12, 3, 12, 6}, []d.Action{melee("Bite", "2d4+2", "piercing", 4)}},
	"orc":            {"Orc", "1/2", 100, 13, 15, 30, [6]int{16, 12, 16, 7, 11, 10}, []d.Action{melee("Greataxe", "1d12+3", "slashing", 5), shoot("Javelin", "1d6+3", "piercing", 5, 30)}},
	"hobgoblin":      {"Hobgoblin", "1/2", 100, 18, 11, 30, [6]int{13, 12, 12, 10, 10, 9}, []d.Action{melee("Longsword", "1d8+1", "slashing", 3), shoot("Longbow", "1d8+1", "piercing", 3, 150)}},
	"gnoll":          {"Gnoll", "1/2", 100, 15, 22, 30, [6]int{14, 12, 11, 6, 10, 7}, []d.Action{melee("Spear", "1d6+2", "piercing", 4), shoot("Longbow", "1d8+1", "piercing", 3, 150)}},
	"ghoul":          {"Ghoul", "1", 200, 12, 22, 30, [6]int{13, 15, 10, 7, 10, 6}, []d.Action{melee("Claws", "2d4+2", "slashing", 4), melee("Bite", "2d6+2", "piercing", 2)}},
	"bugbear":        {"Bugbear", "1", 200, 16, 27, 30, [6]int{15, 14, 13, 8, 11, 9}, []d.Action{melee("Morningstar", "2d8+2", "piercing", 4), shoot("Javelin", "1d6+2", "piercing", 4, 30)}},
	"giant_spider":   {"Giant Spider", "1", 200, 14, 26, 30, [6]int{14, 16, 12, 2, 11, 4}, []d.Action{melee("Bite", "1d8+3", "piercing", 5)}},
	"dire_wolf":      {"Dire Wolf", "1", 200, 14, 37, 50, [6]int{17, 15, 15, 3, 12, 7}, []d.Action{melee("Bite", "2d6+3", "piercing", 5)}},
	"bandit_captain": {"Bandit Captain", "2", 450, 15, 65, 30, [6]int{15, 16, 14, 14, 11, 14}, []d.Action{melee("Scimitar", "2d6+3", "slashing", 5), shoot("Dagger", "1d4+3", "piercing", 5, 20)}},
	"ogre":           {"Ogre", "2", 450, 11, 59, 40, [6]int{19, 8, 16, 5, 7, 7}, []d.Action{melee("Greatclub", "2d8+4", "bludgeoning", 6), shoot("Javelin", "2d6+4", "piercing", 6, 30)}},
	"owlbear":        {"Owlbear", "3", 700, 13, 59, 40, [6]int{20, 12, 17, 3, 12, 7}, []d.Action{melee("Claws", "2d8+5", "slashing", 7), melee("Beak", "1d10+5", "piercing", 7)}},
}

// monsterTactics is how each monster fights, after its SRD traits
// (Pack Tactics, Nimble Escape, Aggressive, Surprise Attack...).
var monsterTactics = map[string]d.Tactic{
	"kobold": d.TacticPack, "giant_rat": d.TacticPack, "wolf": d.TacticPack, "dire_wolf": d.TacticPack, "hobgoblin": d.TacticPack,
	"goblin": d.TacticSkirmisher,
	"bandit": d.TacticCunning, "bandit_captain": d.TacticCunning, "bugbear": d.TacticCunning, "ghoul": d.TacticCunning, "giant_spider": d.TacticCunning,
	"orc": d.TacticBrute, "gnoll": d.TacticBrute, "ogre": d.TacticBrute, "owlbear": d.TacticBrute, "cultist": d.TacticBrute,
	"skeleton": d.TacticMindless, "zombie": d.TacticMindless,
}

// multiattack are the monsters that make two attacks with their action.
var multiattack = map[string]bool{"owlbear": true, "bandit_captain": true}

// undead are the monsters a cleric can turn.
var undead = map[string]bool{"skeleton": true, "zombie": true, "ghoul": true}

func (m monsterStat) creature(key, id string) *d.Creature {
	scores := d.AbilityScores{}
	for i, a := range d.Abilities {
		scores[a] = m.scores[i]
	}
	return &d.Creature{
		ID: id, Name: m.name, Kind: d.KindMonster, Controller: d.ControlDM, Level: 1,
		Abilities: scores, MaxHP: m.hp, HP: m.hp, AC: m.ac, Speed: m.speed,
		Actions: append([]d.Action(nil), m.attacks...), CR: m.cr, XPValue: m.xp, Portrait: key, Tactic: monsterTactics[key],
		Type: map[bool]string{true: "undead"}[undead[key]], Features: multiattackFeature(key),
	}
}

func multiattackFeature(key string) []d.Feature {
	if !multiattack[key] {
		return nil
	}
	return []d.Feature{{Key: d.FeatExtraAttack, Name: "Multiattack", Description: "Makes two attacks with its action."}}
}
