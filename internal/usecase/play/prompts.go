package play

import (
	"dnd/internal/domain"
	"fmt"
	"strings"
)

// dmPromptData fills the dm role's template. The DM's system prompt only
// changes when the party does, so it stays cached across the campaign.
type dmPromptData struct {
	Title    string
	Style    string
	Party    []partyEntry
	Monsters string
	Scenes   string
	// Tutorial is a new player's guided first adventure.
	Tutorial bool
}

type partyEntry struct {
	ID, Name, Race, Class, About string
}

func (s *session) dmPrompt() dmPromptData {
	d := dmPromptData{
		Title: s.g.Title, Style: s.u.catalog.DMStyle(s.g.DMStyle).Guidance,
		Monsters: s.u.catalog.MonsterSummary(), Scenes: strings.Join(domain.SceneKeys, ", "),
		Tutorial: s.g.Tutorial,
	}
	for _, c := range s.g.PartyMembers() {
		d.Party = append(d.Party, partyEntry{ID: c.ID, Name: c.Name, Race: c.Race, Class: c.Class, About: personaLine(c)})
	}
	return d
}

func personaLine(c *domain.Creature) string {
	if c.Controller == domain.ControlHuman {
		return "Played by the human."
	}
	if c.Persona == nil {
		return "AI companion."
	}
	if c.Persona.Backstory == "" {
		return "AI companion: " + c.Persona.Archetype + "."
	}
	return fmt.Sprintf("AI companion: %s. Backstory: %s", c.Persona.Archetype, domain.PlayerText(c.Persona.Backstory))
}

// characterPromptData fills the character role's template.
type characterPromptData struct {
	Name, Race, Class   string
	Level               int
	Companions, Players string
	Persona             domain.Persona
	Backstory           string
	Tactics, Actions    string
}

func characterPrompt(g *domain.Game, c *domain.Creature, tactics string) characterPromptData {
	p := c.Persona
	if p == nil {
		p = &domain.Persona{Archetype: "Loyal Adventurer", Personality: "Brave and helpful.", Voice: "Plain-spoken."}
	}
	var others []string
	for _, m := range g.PartyMembers() {
		if m.ID != c.ID {
			others = append(others, fmt.Sprintf("%s (%s %s)", m.Name, m.Race, m.Class))
		}
	}
	backstory := p.Backstory
	if backstory == "" {
		backstory = "Make up a short backstory that fits your class and archetype, and stay consistent with it."
	}
	return characterPromptData{
		Name: c.Name, Race: c.Race, Class: c.Class, Level: c.Level,
		Companions: strings.Join(others, ", "), Players: playersLine(g, c),
		Persona: *p, Backstory: backstory, Tactics: tactics, Actions: actionNames(c),
	}
}

// playersLine tells an agent which companions are played live by people.
func playersLine(g *domain.Game, self *domain.Creature) string {
	var names []string
	for _, m := range g.PartyMembers() {
		if m.ID != self.ID && domain.Live(m) {
			names = append(names, m.Name)
		}
	}
	if len(names) == 0 {
		return "Every companion is played by an agent like you; keep the party moving and make it fun."
	}
	return strings.Join(names, ", ") + " played by real people; treat them as friends and fellow heroes, and let them lead."
}

// canDo lists what a character can actually do (weapons, cantrips, spells
// with slots left) and carries, so suggested actions stay achievable.
// featureText lists class features with uses left, e.g. "Rage (1/2, long rest)".
func featureText(c *domain.Creature) string {
	parts := make([]string, 0, len(c.Features))
	for _, f := range c.Features {
		if f.Max == 0 {
			parts = append(parts, f.Name)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%d/%d, %s rest)", f.Name, f.Left(), f.Max, f.Recharge))
	}
	return strings.Join(parts, ", ")
}

func canDo(c *domain.Creature) string {
	var can []string
	for _, a := range c.Actions {
		switch {
		case a.Kind == domain.ActDodge:
		case a.SlotLevel > 0:
			can = append(can, fmt.Sprintf("%s (level %d spell)", a.Name, a.SlotLevel))
		case a.Kind != domain.ActAttack:
			can = append(can, a.Name+" (cantrip)")
		default:
			can = append(can, a.Name)
		}
	}
	out := "Can: " + strings.Join(can, ", ")
	if slots := slotsLeft(c); slots != "" {
		out += "; spell slots left: " + slots
	}
	if feats := featureText(c); feats != "" {
		out += ". Features: " + feats
	}
	if len(c.Items) > 0 {
		labels := make([]string, 0, len(c.Items))
		for _, it := range c.Items {
			labels = append(labels, it.Label())
		}
		out += ". Carries: " + strings.Join(labels, ", ")
	}
	return out
}

func slotsLeft(c *domain.Creature) string {
	var parts []string
	for lvl := 1; lvl <= 9; lvl++ {
		if max := c.SpellSlots[lvl]; max > 0 {
			parts = append(parts, fmt.Sprintf("L%d %d/%d", lvl, max-c.SlotsUsed[lvl], max))
		}
	}
	return strings.Join(parts, ", ")
}

func actionNames(c *domain.Creature) string {
	names := make([]string, 0, len(c.Actions))
	for _, a := range c.Actions {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

// choicesReminder ends every party-facing request: recent instructions are
// followed far more reliably than ones buried at the top of a long history.
const choicesReminder = "Write your narration as reply text (players never see your thinking). Call show_npc for anyone new in the scene, then finish by calling offer_choices with 3-4 options."

func openingMessage(g *domain.Game) string {
	return "Begin the adventure. Set the opening scene (call set_scene), introduce a hook that draws the party together, and give each companion a moment to be noticed. " +
		choicesReminder + "\n\n" + partyStatus(g)
}

func partyMessage(g *domain.Game, lines []string) string {
	return "The party acts:\n" + strings.Join(lines, "\n") + "\n\nResolve these actions, calling tools for anything uncertain, then narrate. " +
		choicesReminder + "\n\n" + partyStatus(g)
}

// partyStatus is the per-turn state digest appended to DM messages.
func partyStatus(g *domain.Game) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[State] Scene: %s (%s). Mode: %s.\n", g.Scene.Title, g.Scene.Key, g.Mode)
	for _, c := range g.PartyMembers() {
		fmt.Fprintf(&b, "- %s %s: L%d %s, HP %d/%d, AC %d, XP %d%s. %s\n", c.ID, c.Name, c.Level, c.Class, c.HP, c.MaxHP, c.AC, c.XP, conditionText(c), canDo(c))
	}
	b.WriteString(npcStatus(g))
	b.WriteString(questStatus(g))
	return b.String()
}

// questStatus lists the party's journal for the DM.
func questStatus(g *domain.Game) string {
	if len(g.Quests) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Journal:\n")
	for _, q := range g.Quests {
		fmt.Fprintf(&b, "- [%s] %s", q.Status, q.Title)
		if q.Note != "" {
			fmt.Fprintf(&b, ": %s", q.Note)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func conditionText(c *domain.Creature) string {
	var parts []string
	for _, cond := range c.Conditions {
		parts = append(parts, string(cond))
	}
	if c.Dead {
		parts = append(parts, "DEAD")
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// battlefield describes combat positions for the DM or a companion.
func battlefield(g *domain.Game) string {
	var b strings.Builder
	for _, c := range g.Creatures {
		if c.Dead {
			continue
		}
		fmt.Fprintf(&b, "- %s (%s) at (%d,%d), HP %d/%d%s\n", c.Name, c.ID, c.Pos.X, c.Pos.Y, c.HP, c.MaxHP, conditionText(c))
	}
	return b.String()
}
