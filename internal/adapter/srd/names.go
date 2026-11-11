package srd

import (
	"dnd/internal/domain"
	"strings"
)

// namesByRace holds name ideas for randomly rolled heroes, by gender.
var namesByRace = map[string]map[domain.Gender][]string{
	"human": {
		domain.Male:   {"Aldric", "Cassian", "Dorin", "Garrick", "Marek", "Corwin", "Edric", "Osric"},
		domain.Female: {"Brenna", "Elowen", "Isolde", "Tamsin", "Wren", "Rowena", "Maren", "Linnea"},
	},
	"elf": {
		domain.Male:   {"Aelar", "Erevan", "Ilphas", "Thamior", "Varis", "Faelar", "Sariel", "Galinndan"},
		domain.Female: {"Caelynn", "Lia", "Naivara", "Ielenia", "Mirethil", "Adrie", "Birel", "Keyleth"},
	},
	"dwarf": {
		domain.Male:   {"Bruenor", "Eberk", "Harbek", "Kildrak", "Rurik", "Orsik", "Baern", "Thorin"},
		domain.Female: {"Dagna", "Gunnloda", "Mardred", "Torbera", "Vistra", "Hlin", "Amber", "Eldeth"},
	},
	"halfling": {
		domain.Male:   {"Bramble", "Eldon", "Merric", "Tobin", "Wellby", "Milo", "Cade", "Finnan"},
		domain.Female: {"Cora", "Lavinia", "Nedda", "Rosie", "Verna", "Seraphina", "Pip", "Kithri"},
	},
	"half-orc": {
		domain.Male:   {"Dench", "Feng", "Gell", "Holg", "Krusk", "Ront", "Thokk", "Imsh"},
		domain.Female: {"Baggi", "Emen", "Ovak", "Sutha", "Vola", "Engong", "Myev", "Kansif"},
	},
	"tiefling": {
		domain.Male:   {"Akmenos", "Damakos", "Mordai", "Skamos", "Leucis", "Ekemon", "Torment", "Riven"},
		domain.Female: {"Kallista", "Lerissa", "Nemeia", "Orianna", "Hope", "Ember", "Bryseis", "Makaria"},
	},
	"dragonborn": {
		domain.Male:   {"Arjhan", "Balasar", "Donaar", "Medrash", "Rhogar", "Torinn", "Kriv", "Ghesh"},
		domain.Female: {"Kava", "Nala", "Perra", "Sora", "Biri", "Daar", "Harann", "Thava"},
	},
	"gnome": {
		domain.Male:   {"Alston", "Boddynock", "Fonkin", "Orryn", "Roywyn", "Zook", "Dimble", "Warryn"},
		domain.Female: {"Bimpnottin", "Carlin", "Ellyjobell", "Nissa", "Tana", "Wrenn", "Lilli", "Orla"},
	},
	"half-elf": {
		domain.Male:   {"Arannis", "Davin", "Kieran", "Sylas", "Vaelen", "Ashby", "Merrin", "Theren"},
		domain.Female: {"Celeste", "Eira", "Liriel", "Talia", "Nyx", "Sienna", "Aria", "Lyra"},
	},
}

var fallbackNames = map[domain.Gender][]string{
	domain.Male:   {"Ash", "Quill", "Thorn", "Vale"},
	domain.Female: {"Bryn", "Sable", "Fern", "Ivy"},
}

// backstories are short hooks for randomly rolled heroes.
var backstories = []string{
	"Left a quiet village after a stranger paid in coins that no one could identify.",
	"Owes a dangerous favour to a thieves' guild and is running out of time to repay it.",
	"Searching for a sibling who vanished on the old north road.",
	"A former soldier haunted by an order they followed and now regret.",
	"Raised in a temple orphanage and still writes letters to the kindly abbess.",
	"Carries a map inherited from a grandparent, half of it torn away.",
	"Was cursed by a hedge witch and is looking for someone who can lift it.",
	"Lost a fortune at cards and took up adventuring to win it back.",
	"Once guarded a merchant caravan that was ambushed; the only survivor.",
	"Dreams every night of a door beneath the sea and wants to find it.",
	"Exiled from a noble house after a scandal they insist wasn't their fault.",
	"An apprentice whose master disappeared, leaving only a locked journal.",
	"Swore to protect travellers after a monster destroyed their home.",
	"Hunting the bandit who stole a family heirloom, a silver ring.",
	"Restless and curious, with a habit of collecting strange stones.",
	"Wants to be famous and keeps a list of songs that should be written about them.",
}

// Names suggests names for a hero of the given race and gender.
func (*Catalog) Names(race string, g domain.Gender) []string {
	if names, ok := namesByRace[strings.ToLower(race)]; ok {
		return names[g]
	}
	return fallbackNames[g]
}

// Backstories suggests short backstory hooks.
func (*Catalog) Backstories() []string { return backstories }
