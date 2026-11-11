package characters

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"slices"
	"strings"
)

// RandomInput asks for a randomly rolled hero.
type RandomInput struct {
	AccountID string `json:"-"`
	// Avoid lists classes already in the party, so the new hero fills a gap.
	Avoid []string `json:"avoid"`
}

// Random rolls a level 1 hero (class, ancestry, personality, name and
// backstory) and adds it to the roster, like Create.
func (u *UseCase) Random(ctx context.Context, in RandomInput) (domain.Character, error) {
	roster, err := u.store.CharactersByOwner(ctx, in.AccountID)
	if err != nil {
		return domain.Character{}, fmt.Errorf("load roster: %w", err)
	}
	race := pick(u.dice, u.muse.Races())
	gender := pick(u.dice, domain.Genders)
	return u.Create(ctx, CreateInput{
		AccountID: in.AccountID,
		Gender:    string(gender),
		Name:      u.name(race.Name, gender, roster),
		Class:     pick(u.dice, freshClasses(u.muse.Classes(), in.Avoid)).Name,
		Race:      race.Name,
		Archetype: pick(u.dice, u.catalog.Archetypes()).Archetype,
		Backstory: pick(u.dice, u.muse.Backstories()),
	})
}

// freshClasses drops the classes to avoid, unless that would leave none.
func freshClasses(all []domain.ClassTemplate, avoid []string) []domain.ClassTemplate {
	fresh := slices.DeleteFunc(slices.Clone(all), func(c domain.ClassTemplate) bool {
		return slices.ContainsFunc(avoid, func(a string) bool { return strings.EqualFold(a, c.Name) })
	})
	if len(fresh) == 0 {
		return all
	}
	return fresh
}

// name picks a name for the race that nobody on the roster has yet.
func (u *UseCase) name(race string, g domain.Gender, roster []domain.Character) string {
	taken := map[string]bool{}
	for _, ch := range roster {
		taken[strings.ToLower(ch.Sheet.Name)] = true
	}
	all := u.muse.Names(race, g)
	free := slices.DeleteFunc(slices.Clone(all), func(n string) bool { return taken[strings.ToLower(n)] })
	if len(free) == 0 {
		return pick(u.dice, all) + " the Younger"
	}
	return pick(u.dice, free)
}

// pick returns a random element; the zero value when there are none.
func pick[T any](dice Roller, items []T) T {
	var zero T
	if len(items) == 0 {
		return zero
	}
	return items[dice.Roll(len(items))-1]
}
