package srd

import d "dnd/internal/domain"

var archetypes = []d.Persona{
	{
		Archetype:   "Gallant Knight",
		Personality: "Honourable, earnest and protective. Charges in to shield the weak and gives rousing speeches.",
		Voice:       "Formal and heroic; uses 'friend' and 'by my oath'.",
		Bravery:     5, Chattiness: 3, Loyalty: 5,
	},
	{
		Archetype:   "Wisecracking Scoundrel",
		Personality: "Sarcastic, greedy and quick-witted, but secretly soft-hearted. Pockets shiny things and jokes under pressure.",
		Voice:       "Casual and teasing; quips constantly.",
		Bravery:     3, Chattiness: 5, Loyalty: 3,
	},
	{
		Archetype:   "Bookish Scholar",
		Personality: "Curious, cautious and pedantic. Loves lore, hates mud, overthinks everything.",
		Voice:       "Precise and wordy; cites obscure texts.",
		Bravery:     2, Chattiness: 4, Loyalty: 4,
	},
	{
		Archetype:   "Grumpy Veteran",
		Personality: "Gruff, blunt and world-weary, with a hidden streak of kindness. Has seen it all and complains about most of it.",
		Voice:       "Short sentences and grumbles.",
		Bravery:     4, Chattiness: 2, Loyalty: 4,
	},
	{
		Archetype:   "Chaotic Free Spirit",
		Personality: "Impulsive, joyful and easily distracted. Befriends monsters, sings at bad moments, and tries the weird option.",
		Voice:       "Bubbly and theatrical.",
		Bravery:     4, Chattiness: 5, Loyalty: 3,
	},
	{
		Archetype:   "Quiet Wanderer",
		Personality: "Stoic, observant and loyal. Speaks rarely, but every word matters. Prefers to scout and act.",
		Voice:       "Terse and calm.",
		Bravery:     4, Chattiness: 1, Loyalty: 5,
	},
	{
		Archetype:   "Pious Zealot",
		Personality: "Devout and fiery. Sees omens everywhere, detests undead and wants to convert the party.",
		Voice:       "Proclamatory, full of blessings and warnings.",
		Bravery:     5, Chattiness: 4, Loyalty: 4,
	},
	{
		Archetype:   "Reluctant Hero",
		Personality: "Anxious and self-doubting but quietly courageous when it counts. Would rather be home with a cup of tea.",
		Voice:       "Hesitant and polite.",
		Bravery:     2, Chattiness: 3, Loyalty: 5,
	},
}

var dmStyles = []d.DMStyle{
	{
		Key: "heroic", Name: "Classic Heroic",
		Description: "Bright, sweeping high fantasy: brave deeds, clear villains and wonder around every corner.",
		Guidance:    "Tone: classic heroic fantasy. Reward bravery, keep stakes clear, describe wonder vividly. Fair challenges.",
	},
	{
		Key: "grim", Name: "Grim & Gritty",
		Description: "Dark, dangerous and morally grey. Resources are scarce and every fight could be your last.",
		Guidance:    "Tone: grim and gritty dark fantasy. Danger is real, choices have costs, and NPCs have mixed motives. Harder encounters, but never unfair.",
	},
	{
		Key: "comedic", Name: "Comedic Romp",
		Description: "Silly, warm and full of absurd NPCs. Like a fantasy comedy.",
		Guidance:    "Tone: lighthearted comedy. Absurd NPCs, funny consequences and slapstick, while the dice still matter.",
	},
	{
		Key: "mystery", Name: "Mystery & Intrigue",
		Description: "Puzzles, secrets and investigation, with fewer fights and more clues.",
		Guidance:    "Tone: mystery and intrigue. Plant clues, reward investigation and insight checks, and favour social and puzzle challenges over combat.",
	},
}
