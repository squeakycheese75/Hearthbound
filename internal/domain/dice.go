package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Dice is a dice expression such as 2d6+3.
type Dice struct {
	Count int `json:"count"`
	Sides int `json:"sides"`
	Bonus int `json:"bonus"`
}

var diceRe = regexp.MustCompile(`^(\d*)d(\d+)\s*([+-]\s*\d+)?$`)

// ParseDice parses "1d8", "2d6+3", "d20-1" or a flat number like "4".
func ParseDice(s string) (Dice, error) {
	t := strings.ToLower(strings.ReplaceAll(s, " ", ""))
	if n, err := strconv.Atoi(t); err == nil {
		return Dice{Bonus: n}, nil
	}
	m := diceRe.FindStringSubmatch(t)
	if m == nil {
		return Dice{}, fmt.Errorf("%w: dice %q", ErrInvalid, s)
	}
	count := 1
	if m[1] != "" {
		count, _ = strconv.Atoi(m[1])
	}
	sides, _ := strconv.Atoi(m[2])
	bonus := 0
	if m[3] != "" {
		bonus, _ = strconv.Atoi(m[3])
	}
	if count < 1 || count > 100 || sides < 2 || sides > 100 {
		return Dice{}, fmt.Errorf("%w: dice %q out of range", ErrInvalid, s)
	}
	return Dice{Count: count, Sides: sides, Bonus: bonus}, nil
}

// MustDice parses s and panics on error. Only for static data.
func MustDice(s string) Dice {
	d, err := ParseDice(s)
	if err != nil {
		panic(err)
	}
	return d
}

// String renders the expression, e.g. "2d6+3".
func (d Dice) String() string {
	if d.Count == 0 {
		return strconv.Itoa(d.Bonus)
	}
	base := fmt.Sprintf("%dd%d", d.Count, d.Sides)
	switch {
	case d.Bonus > 0:
		return fmt.Sprintf("%s+%d", base, d.Bonus)
	case d.Bonus < 0:
		return fmt.Sprintf("%s%d", base, d.Bonus)
	}
	return base
}

// Average returns the rounded-down average result.
func (d Dice) Average() int {
	return d.Count*(d.Sides+1)/2 + d.Bonus
}
