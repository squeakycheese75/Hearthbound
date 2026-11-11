package domain

import "fmt"

// Gender is a character's or NPC's gender, chosen when they are made. It
// shapes their portrait and the voice that reads their lines.
type Gender string

// Genders.
const (
	Male   Gender = "m"
	Female Gender = "f"
)

// Genders lists every gender.
var Genders = []Gender{Male, Female}

// ParseGender reads "m" or "f".
func ParseGender(s string) (Gender, error) {
	switch Gender(s) {
	case Male, Female:
		return Gender(s), nil
	}
	return "", fmt.Errorf("%w: gender must be m or f", ErrInvalid)
}

// Word is the gender as an adjective for art and voice prompts ("" if unset).
func (g Gender) Word() string {
	switch g {
	case Male:
		return "male"
	case Female:
		return "female"
	}
	return ""
}

// genderSuffix is added to stock art keys for gendered variants.
func (g Gender) suffix() string {
	if g == "" {
		return ""
	}
	return "-" + string(g)
}
