package srd

import (
	"fmt"
	"slices"
	"strings"

	d "dnd/internal/domain"
)

// Catalog serves classes, races, monsters, persona archetypes and DM styles.
type Catalog struct{}

// New returns the built-in catalog.
func New() *Catalog { return &Catalog{} }

// Classes lists every playable class.
func (*Catalog) Classes() []d.ClassTemplate { return classes }

// Races lists every playable race.
func (*Catalog) Races() []d.RaceTemplate { return races }

// Class finds a class by name (case-insensitive).
func (*Catalog) Class(name string) (d.ClassTemplate, error) {
	for _, c := range classes {
		if strings.EqualFold(c.Name, name) {
			return c, nil
		}
	}
	return d.ClassTemplate{}, fmt.Errorf("%w: class %q", d.ErrUnknown, name)
}

// Race finds a race by name (case-insensitive).
func (*Catalog) Race(name string) (d.RaceTemplate, error) {
	for _, r := range races {
		if strings.EqualFold(r.Name, name) {
			return r, nil
		}
	}
	return d.RaceTemplate{}, fmt.Errorf("%w: race %q", d.ErrUnknown, name)
}

// MonsterKeys lists the monster keys the DM may spawn, sorted.
func (*Catalog) MonsterKeys() []string {
	keys := make([]string, 0, len(monsters))
	for k := range monsters {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// MonsterSummary describes each monster for the DM's prompt: key, CR, HP and AC.
func (c *Catalog) MonsterSummary() string {
	var b strings.Builder
	for _, k := range c.MonsterKeys() {
		m := monsters[k]
		fmt.Fprintf(&b, "- %s: %s (CR %s, %d XP, AC %d, HP %d, fights as a %s)\n", k, m.name, m.cr, m.xp, m.ac, m.hp, monsterTactics[k])
	}
	return b.String()
}

// Monster creates a fresh monster instance with the given ID.
func (*Catalog) Monster(key, id string) (*d.Creature, error) {
	m, ok := monsters[slugify(key)]
	if !ok {
		return nil, fmt.Errorf("%w: monster %q", d.ErrUnknown, key)
	}
	return m.creature(slugify(key), id), nil
}

// Archetypes lists persona presets for AI companions.
func (*Catalog) Archetypes() []d.Persona { return archetypes }

// DMStyles lists the Dungeon Master tone presets.
func (*Catalog) DMStyles() []d.DMStyle { return dmStyles }

// DMStyle finds a style by key, defaulting to the first.
func (*Catalog) DMStyle(key string) d.DMStyle {
	for _, s := range dmStyles {
		if s.Key == key {
			return s
		}
	}
	return dmStyles[0]
}

func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
}
