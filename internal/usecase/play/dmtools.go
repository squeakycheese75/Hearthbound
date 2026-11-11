package play

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

func spec(name, desc, schema string) domain.ToolSpec {
	return domain.ToolSpec{Name: name, Description: desc, Schema: json.RawMessage(schema)}
}

func quoted(vals []string) string {
	q := make([]string, len(vals))
	for i, v := range vals {
		q[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(q, ",")
}

func skillNames() []string {
	var out []string
	for _, s := range domain.Skills() {
		out = append(out, string(s))
	}
	return out
}

func offerChoicesSpec() domain.ToolSpec {
	return spec("offer_choices", "Call last whenever the party may act: 3-4 distinct, specific next actions for the hero, each under 10 words and written as an action (e.g. \"Ask Tobin where Elsie was last seen\"). Mix safe, bold and clever options. Only offer what the party can really do: use the abilities, spells (with slots left) and items listed for each member in [State], and never invent spells, items or powers a character doesn't have. This ends your turn.",
		`{"type":"object","properties":{"choices":{"type":"array","minItems":2,"maxItems":4,"items":{"type":"string"}}},"required":["choices"],"additionalProperties":false}`)
}

// npcSpecs are the DM's tools for putting NPCs on the table (show_npc, npc_leaves).
func npcSpecs() []domain.ToolSpec {
	return []domain.ToolSpec{
		spec("show_npc", "Put a non-player character on the table so the players see them: anyone who joins the scene and matters (an innkeeper, a guide, a prisoner, a rival). Call again with the same name to update them (a change of attitude, or they start travelling with the party). Not for monsters the party fights: use start_combat for those.",
			`{"type":"object","properties":{"name":{"type":"string"},"type":{"type":"string","description":"A person type (`+strings.Join(domain.NPCTypes, ", ")+`) or a monster key from the list below for creatures (e.g. kobold for a kobold guide)."},"role":{"type":"string","description":"Who they are to the party, in a few words, e.g. 'kobold prisoner turned guide'."},"attitude":{"type":"string","enum":[`+quoted(domain.Attitudes)+`]},"gender":{"type":"string","enum":["m","f"],"description":"m or f. Sets their portrait and the voice that reads their lines."},"look":{"type":"string","description":"One sentence on how they look, for the illustrator."},"follows":{"type":"boolean","description":"True if they travel with the party to new scenes."}},"required":["name","type","role","attitude","gender"],"additionalProperties":false}`),
		spec("npc_leaves", "Take a non-player character off the table when they leave, die or are left behind.",
			`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`),
	}
}

func dmTools() []domain.ToolSpec {
	abilities := `"enum":["STR","DEX","CON","INT","WIS","CHA"]`
	adv := `"advantage":{"type":"string","enum":["normal","advantage","disadvantage"]}`
	return append([]domain.ToolSpec{
		spec("set_scene", "Change the backdrop when the party arrives somewhere new. An illustrator paints it from title, mood and visual, so describe what the place looks like.",
			`{"type":"object","properties":{"scene_key":{"type":"string","enum":[`+quoted(domain.SceneKeys)+`]},"title":{"type":"string"},"mood":{"type":"string"},"visual":{"type":"string","description":"One or two sentences describing what the place looks like (architecture, light, weather, notable features), for the illustrator. No characters."}},"required":["scene_key","title","mood","visual"],"additionalProperties":false}`),
		spec("ability_check", "Roll a skill or ability check for a character. The engine rolls; never invent the result. Provide skill OR ability.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"skill":{"type":"string","enum":[`+quoted(skillNames())+`]},"ability":{"type":"string",`+abilities+`},"dc":{"type":"integer","minimum":1,"maximum":30},`+adv+`,"reason":{"type":"string"}},"required":["character_id","dc","reason"],"additionalProperties":false}`),
		spec("saving_throw", "Roll a saving throw for a character against a hazard, trap or effect.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"ability":{"type":"string",`+abilities+`},"dc":{"type":"integer","minimum":1,"maximum":30},`+adv+`,"reason":{"type":"string"}},"required":["character_id","ability","dc","reason"],"additionalProperties":false}`),
		spec("deal_damage", "Deal damage outside combat (traps, falls, hazards). The engine rolls the dice.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"dice":{"type":"string","description":"e.g. 2d6 or 1d10+2"},"damage_type":{"type":"string"},"reason":{"type":"string"}},"required":["character_id","dice","damage_type","reason"],"additionalProperties":false}`),
		spec("heal", "Restore hit points (potions, NPC healers). The engine rolls the dice.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"dice":{"type":"string"},"reason":{"type":"string"}},"required":["character_id","dice","reason"],"additionalProperties":false}`),
		spec("set_condition", "Add or remove a condition on a character.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"condition":{"type":"string","enum":["blinded","frightened","poisoned","prone","restrained"]},"active":{"type":"boolean"}},"required":["character_id","condition","active"],"additionalProperties":false}`),
		spec("give_item", "Give a character a specific item and/or gold: ordinary equipment by name (rope, a potion of healing, a shortsword), a story keepsake (a map, a key, a letter; it has no stats), or gold (negative gold pays for purchases). For treasure and magic items use roll_loot instead.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"item":{"type":"string"},"qty":{"type":"integer","minimum":1,"maximum":20},"gold":{"type":"integer"}},"required":["character_id"],"additionalProperties":false}`),
		spec("roll_loot", "Treasure: the engine rolls a random item (weapon, armour, shield or trinket) of random rarity for the party's level and gives it to a character. Use it for chests, hoards and defeated foes' gear; quality good is for bosses and hidden hoards. Narrate the find from the result. At most two per turn.",
			`{"type":"object","properties":{"character_id":{"type":"string"},"kind":{"type":"string","enum":["weapon","armour","shield","trinket"]},"quality":{"type":"string","enum":["normal","good"]}},"required":["character_id"],"additionalProperties":false}`),
		spec("rest", "The party takes a short or long rest. Long rests restore all HP and spell slots.",
			`{"type":"object","properties":{"kind":{"type":"string","enum":["short","long"]}},"required":["kind"],"additionalProperties":false}`),
		spec("award_xp", "Award experience to every surviving party member for milestones, clever play or roleplay.",
			`{"type":"object","properties":{"amount":{"type":"integer","minimum":1,"maximum":2000},"reason":{"type":"string"}},"required":["amount","reason"],"additionalProperties":false}`),
		spec("start_combat", "Begin a fight. Spawns monsters, rolls initiative and hands control to the combat engine. After calling it, narrate one or two tense sentences and stop.",
			`{"type":"object","properties":{"monsters":{"type":"array","minItems":1,"maxItems":8,"items":{"type":"object","properties":{"key":{"type":"string"},"count":{"type":"integer","minimum":1,"maximum":6},"name":{"type":"string","description":"optional flavour name, e.g. 'Grik the Goblin Boss'"}},"required":["key","count"],"additionalProperties":false}}},"required":["monsters"],"additionalProperties":false}`),
		spec("update_quest", "Keep the party's journal: call when they take on a goal (status open), learn something that changes it (a short note), or complete or fail it. Use the same title to update a quest. The players see the journal.",
			`{"type":"object","properties":{"title":{"type":"string","description":"Short, e.g. 'Find Elsie the miller's daughter'."},"status":{"type":"string","enum":["open","done","failed"]},"note":{"type":"string","description":"Optional: one sentence on what the party knows now."}},"required":["title","status"],"additionalProperties":false}`),
		offerChoicesSpec(),
		spec("end_adventure", "End the campaign when the story reaches a satisfying conclusion or the party is lost.",
			`{"type":"object","properties":{"epilogue":{"type":"string"}},"required":["epilogue"],"additionalProperties":false}`),
	}, npcSpecs()...)
}

type dmInput struct {
	CharacterID string   `json:"character_id"`
	SceneKey    string   `json:"scene_key"`
	Title       string   `json:"title"`
	Mood        string   `json:"mood"`
	Visual      string   `json:"visual"`
	Skill       string   `json:"skill"`
	Ability     string   `json:"ability"`
	DC          int      `json:"dc"`
	Advantage   string   `json:"advantage"`
	Reason      string   `json:"reason"`
	Dice        string   `json:"dice"`
	DamageType  string   `json:"damage_type"`
	Condition   string   `json:"condition"`
	Active      bool     `json:"active"`
	Item        string   `json:"item"`
	Gold        int      `json:"gold"`
	Kind        string   `json:"kind"`
	Amount      int      `json:"amount"`
	Epilogue    string   `json:"epilogue"`
	Choices     []string `json:"choices"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Role        string   `json:"role"`
	Attitude    string   `json:"attitude"`
	Gender      string   `json:"gender"`
	Look        string   `json:"look"`
	Follows     *bool    `json:"follows"`
	Qty         int      `json:"qty"`
	Quality     string   `json:"quality"`
	Status      string   `json:"status"`
	Note        string   `json:"note"`
	Monsters    []struct {
		Key   string `json:"key"`
		Count int    `json:"count"`
		Name  string `json:"name"`
	} `json:"monsters"`
}

func (s *session) dmTool(call domain.ToolCall) (string, error) {
	var in dmInput
	if err := json.Unmarshal(call.Input, &in); err != nil {
		return "", fmt.Errorf("%w: bad tool input: %w", domain.ErrInvalid, err)
	}
	handlers := map[string]func(dmInput) (string, error){
		"set_scene": s.setScene, "ability_check": s.abilityCheck, "saving_throw": s.savingThrow,
		"deal_damage": s.dealDamage, "heal": s.heal, "set_condition": s.setCondition,
		"give_item": s.giveItem, "rest": s.rest, "award_xp": s.awardXP,
		"start_combat": s.startCombat, "end_adventure": s.endAdventure, "offer_choices": s.offerChoices,
		"show_npc": s.showNPC, "npc_leaves": s.npcLeaves, "roll_loot": s.rollLoot,
		"update_quest": s.updateQuest,
	}
	h, ok := handlers[call.Name]
	if !ok {
		return "", fmt.Errorf("%w: tool %q", domain.ErrUnknown, call.Name)
	}
	return h(in)
}

func (s *session) setScene(in dmInput) (string, error) {
	if !slices.Contains(domain.SceneKeys, in.SceneKey) {
		return "", fmt.Errorf("%w: scene %q", domain.ErrUnknown, in.SceneKey)
	}
	s.g.Scene = domain.Scene{Key: in.SceneKey, Title: in.Title, Mood: in.Mood, Visual: in.Visual}
	for _, n := range s.g.NPCs {
		n.Here = n.Here && n.Follows // the rest stay behind
	}
	s.u.art.Request(domain.SceneFor(s.g))
	scene := s.g.Scene
	s.emit(domain.Event{Type: domain.EvScene, Scene: &scene, Text: in.Title})
	return "Scene set to " + in.Title + ".", nil
}

func (s *session) abilityCheck(in dmInput) (string, error) {
	c, err := s.g.Creature(in.CharacterID)
	if err != nil {
		return "", err
	}
	var skill domain.Skill
	var ability domain.Ability
	if in.Skill != "" {
		if skill, err = domain.ParseSkill(in.Skill); err != nil {
			return "", err
		}
	}
	if skill == "" {
		if ability, err = domain.ParseAbility(in.Ability); err != nil {
			return "", err
		}
	}
	r := rules.AbilityCheck(s.u.roller, c, ability, skill, in.DC, rules.ParseAdvantage(in.Advantage))
	return s.reportTest(c, r, in.Reason), nil
}

func (s *session) savingThrow(in dmInput) (string, error) {
	c, err := s.g.Creature(in.CharacterID)
	if err != nil {
		return "", err
	}
	ability, err := domain.ParseAbility(in.Ability)
	if err != nil {
		return "", err
	}
	r := rules.SavingThrow(s.u.roller, c, ability, in.DC, rules.ParseAdvantage(in.Advantage))
	return s.reportTest(c, r, in.Reason), nil
}

func (s *session) reportTest(c *domain.Creature, r domain.Roll, reason string) string {
	r.Label = fmt.Sprintf("%s: %s", r.Label, reason)
	s.roll(c.ID, r)
	verdict := "FAILURE"
	if r.Success != nil && *r.Success {
		verdict = "SUCCESS"
	}
	return fmt.Sprintf("%s rolled %d (d20 %v, %+d) vs DC %d: %s.", c.Name, r.Total, r.Rolls, r.Modifier, r.Target, verdict)
}

func (s *session) dealDamage(in dmInput) (string, error) {
	c, d, err := s.targetAndDice(in)
	if err != nil {
		return "", err
	}
	total, rolls := rules.RollDice(s.u.roller, d)
	total = max(total, 0)
	s.roll(c.ID, domain.Roll{Label: fmt.Sprintf("%s (%s)", in.Reason, in.DamageType), Dice: d.String(), Rolls: rolls, Modifier: d.Bonus, Total: total})
	notes := rules.DamageOf(c, total, in.DamageType)
	s.notes(notes)
	return strings.Join(notes, " "), nil
}

func (s *session) heal(in dmInput) (string, error) {
	c, d, err := s.targetAndDice(in)
	if err != nil {
		return "", err
	}
	total, rolls := rules.RollDice(s.u.roller, d)
	s.roll(c.ID, domain.Roll{Label: in.Reason, Dice: d.String(), Rolls: rolls, Modifier: d.Bonus, Total: total})
	notes := rules.Heal(c, total)
	s.notes(notes)
	return fmt.Sprintf("%s is at %d/%d HP.", c.Name, c.HP, c.MaxHP), nil
}

func (s *session) targetAndDice(in dmInput) (*domain.Creature, domain.Dice, error) {
	c, err := s.g.Creature(in.CharacterID)
	if err != nil {
		return nil, domain.Dice{}, err
	}
	d, err := domain.ParseDice(in.Dice)
	if err != nil {
		return nil, domain.Dice{}, err
	}
	if err := rules.CheckHazardDice(d, s.g.PartyLevel()); err != nil {
		return nil, domain.Dice{}, err
	}
	return c, d, nil
}

func (s *session) setCondition(in dmInput) (string, error) {
	c, err := s.g.Creature(in.CharacterID)
	if err != nil {
		return "", err
	}
	cond := domain.Condition(in.Condition)
	if !in.Active {
		c.RemoveCondition(cond)
		s.system(fmt.Sprintf("%s is no longer %s.", c.Name, cond))
		return "Done.", nil
	}
	c.AddCondition(cond)
	s.system(fmt.Sprintf("%s is %s.", c.Name, cond))
	return "Done.", nil
}

func (s *session) giveItem(in dmInput) (string, error) {
	c, err := s.g.Creature(in.CharacterID)
	if err != nil {
		return "", err
	}
	if c.Gold+in.Gold < 0 {
		return "", fmt.Errorf("%w: %s only has %d gold", domain.ErrInvalid, c.Name, c.Gold)
	}
	item := strings.TrimSpace(in.Item)
	if err := s.checkLoot(c, item, in.Gold); err != nil {
		return "", err
	}
	c.Gold += in.Gold
	s.goldGiven += max(in.Gold, 0)
	s.recordGold(c, in.Gold, in.Item)
	var parts []string
	if item != "" {
		it := s.makeItem(item, in.Qty)
		c.Items = append(c.Items, it)
		parts = append(parts, it.Label())
	}
	if in.Gold != 0 {
		parts = append(parts, fmt.Sprintf("%+d gold", in.Gold))
	}
	msg := fmt.Sprintf("%s receives %s.", c.Name, strings.Join(parts, " and "))
	s.cue(giftCue(item), msg)
	return msg, nil
}

func (s *session) rest(in dmInput) (string, error) {
	song := 0
	if in.Kind != "long" {
		song = rules.SongOfRest(s.u.roller, s.g.PartyMembers())
	}
	var notes []string
	for _, c := range s.g.PartyMembers() {
		if c.Dead {
			continue
		}
		if in.Kind == "long" {
			c.SlotsUsed = nil
			c.Conditions = nil
			c.Inspiration = 0
			c.RechargeFeatures(domain.RechargeLong)
			rules.Heal(c, c.MaxHP)
			continue
		}
		c.RechargeFeatures(domain.RechargeShort)
		notes = append(notes, rules.ArcaneRecovery(c)...)
		rules.Heal(c, max(c.MaxHP/4, 1)+song)
	}
	msg := fmt.Sprintf("The party takes a %s rest.", in.Kind)
	if song > 0 {
		msg += fmt.Sprintf(" A song soothes their wounds (+%d HP each).", song)
	}
	s.cue(domain.CueRest, msg)
	s.notes(notes)
	return msg + " " + strings.Join(notes, " ") + " " + partyStatus(s.g), nil
}

func (s *session) updateQuest(in dmInput) (string, error) {
	status := domain.QuestStatus(in.Status)
	added, err := s.g.UpdateQuest(in.Title, status, in.Note)
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(in.Title)
	msg := map[domain.QuestStatus]string{
		domain.QuestOpen:   "Journal updated: " + title + ".",
		domain.QuestDone:   "Quest complete: " + title + ".",
		domain.QuestFailed: "Quest failed: " + title + ".",
	}[status]
	if added && status == domain.QuestOpen {
		msg = "New quest: " + title + "."
	}
	s.system(msg)
	return msg, nil
}

// giftCue is the sound for a gift: an item, or just gold.
func giftCue(item string) domain.Cue {
	if item == "" {
		return domain.CueCoins
	}
	return domain.CueLoot
}

// recordGold notes gold the DM handed out (loot entering the realm) or took
// (an in-story purchase leaving it) for the gold ledger.
func (s *session) recordGold(c *domain.Creature, gold int, note string) {
	if gold == 0 {
		return
	}
	why := domain.GoldLoot
	if gold < 0 {
		why, gold = domain.GoldDMCharge, -gold
	}
	s.g.GoldMoves = append(s.g.GoldMoves, domain.LedgerEntry{
		At: s.u.clock.Now(), Reason: why, Amount: gold, AccountID: c.OwnerID, CharacterID: c.ID, GameID: s.g.ID, Note: note,
	})
}

// makeItem turns a name into an item: catalogue equipment when the
// catalogue knows it, otherwise a story keepsake with no stats.
func (s *session) makeItem(name string, qty int) domain.Item {
	id := s.g.NewItemID()
	var it domain.Item
	if b, ok := s.u.catalog.ItemByName(name); ok {
		it = domain.NewItem(id, b, qty)
	}
	if it.ID == "" {
		it = domain.Keepsake(id, name)
	}
	it.History = []string{s.provenance("Given")}
	return it
}

// provenance says where and when an item changed hands, for its history.
func (s *session) provenance(how string) string {
	return fmt.Sprintf("%s in %s, %s (%s)", how, s.g.Scene.Title, s.g.Title, s.u.clock.Now().Format("2 Jan 2006"))
}

// rollLoot has the engine roll a random item for a character.
func (s *session) rollLoot(in dmInput) (string, error) {
	c, err := s.g.Creature(in.CharacterID)
	if err != nil {
		return "", err
	}
	if s.lootGiven >= rules.TurnLootCap {
		return "", fmt.Errorf("%w: at most %d rolled items per turn", domain.ErrInvalid, rules.TurnLootCap)
	}
	if len(c.Items) >= rules.MaxInventory {
		return "", fmt.Errorf("%w: %s can't carry more than %d items", domain.ErrInvalid, c.Name, rules.MaxInventory)
	}
	base, err := rules.PickBase(s.u.roller, s.u.catalog.Items(), domain.ItemKind(in.Kind))
	if err != nil {
		return "", err
	}
	it := rules.RollItem(s.u.roller, s.g.NewItemID(), base, s.g.PartyLevel(), in.Quality == "good")
	it.History = []string{s.provenance("Found")}
	c.Items = append(c.Items, it)
	s.lootGiven++
	msg := fmt.Sprintf("%s finds %s.", c.Name, it.Label())
	s.cue(domain.CueLoot, msg)
	return msg + " " + it.Desc, nil
}

// checkLoot applies the engine's loot limits: gold per turn by party level,
// item name length, and how much one character can carry.
func (s *session) checkLoot(c *domain.Creature, item string, gold int) error {
	level := s.g.PartyLevel()
	if limit := rules.TurnGoldCap(level) * s.econ.LootPercent / 100; gold > 0 && s.goldGiven+gold > limit {
		return fmt.Errorf("%w: loot is capped at %d gold per turn at level %d (%d given already)", domain.ErrInvalid, limit, level, s.goldGiven)
	}
	if len([]rune(item)) > rules.MaxItemName {
		return fmt.Errorf("%w: keep item names under %d characters", domain.ErrInvalid, rules.MaxItemName)
	}
	if item != "" && len(c.Items) >= rules.MaxInventory {
		return fmt.Errorf("%w: %s can't carry more than %d items", domain.ErrInvalid, c.Name, rules.MaxInventory)
	}
	return nil
}

func (s *session) awardXP(in dmInput) (string, error) {
	level := s.g.PartyLevel()
	if limit := rules.TurnXPCap(level); s.xpGiven+in.Amount > limit {
		return "", fmt.Errorf("%w: awards are capped at %d XP per turn at level %d (%d given already); award less, or save it for a milestone later",
			domain.ErrInvalid, limit, level, s.xpGiven)
	}
	s.xpGiven += in.Amount
	s.cue(domain.CueXP, fmt.Sprintf("Each hero gains %d XP: %s", in.Amount, in.Reason))
	return s.grantXP(in.Amount) + partyStatus(s.g), nil
}

// grantXP gives xp to living party members and levels them up when earned.
// It returns a note for the DM about any level-ups.
func (s *session) grantXP(xp int) string {
	var b strings.Builder
	for _, c := range s.g.PartyMembers() {
		if c.Dead {
			continue
		}
		c.XP += xp
		if lvl := rules.LevelForXP(c.XP); lvl > c.Level {
			b.WriteString(s.levelUp(c, lvl))
		}
	}
	return b.String()
}

// levelUp raises a hero to a level and tells the table what they gained.
// It returns a note for the DM.
func (s *session) levelUp(c *domain.Creature, level int) string {
	cls, err := s.u.catalog.Class(c.Class)
	if err != nil {
		s.system(fmt.Sprintf("%s could not level up: %v", c.Name, err))
		return ""
	}
	before := domain.Snapshot(c)
	c.Recalculate(cls, s.u.itemLookup) // base scores and hit points in place
	c.BaseMaxHP += domain.HPPerLevel(cls, c.BaseAbilities.Mod(domain.CON)) * (level - c.Level)
	improvements := 0
	for _, l := range domain.ASILevels {
		if c.Level < l && l <= level {
			improvements++
		}
	}
	c.ASI += improvements
	c.Level, c.SpellSlots = level, maps.Clone(cls.Slots[level])
	c.Recalculate(cls, s.u.itemLookup) // raises HP with max HP, adds features and spells
	gains := domain.LevelGains(before, c)
	if improvements > 0 {
		gains = append(gains, "Ability Score Improvement: raise one ability by 2, or two by 1 (choose on the character sheet)")
	}
	s.emit(domain.Event{
		Type: domain.EvLevelUp, Actor: c.ID, Cue: domain.CueLevel,
		Text: fmt.Sprintf("%s reaches level %d!", c.Name, level), LevelUp: &domain.LevelUp{Level: level, Gains: gains},
	})
	return fmt.Sprintf("%s reached level %d and gained: %s. Mark the moment in the story.\n", c.Name, level, strings.Join(gains, "; "))
}

func (s *session) startCombat(in dmInput) (string, error) {
	if s.g.Mode == domain.ModeCombat {
		return "", fmt.Errorf("%w: combat is already running", domain.ErrWrongMode)
	}
	var foes []*domain.Creature
	for _, m := range in.Monsters {
		for i := range m.Count {
			c, err := s.u.catalog.Monster(m.Key, s.monsterID(m.Key, foes))
			if err != nil {
				return "", err
			}
			c.Name = monsterName(c.Name, m.Name, i, m.Count)
			foes = append(foes, c)
		}
	}
	party := s.g.PartyMembers()
	if err := tooTough(foes, party); err != nil {
		return "", err
	}
	if cost, budget := rules.AdjustedXP(foes, len(party)), rules.HardBudget(party); cost > budget {
		return "", fmt.Errorf("%w: too deadly for this party (%d adjusted XP; a hard fight is at most %d). Use fewer or weaker monsters", domain.ErrInvalid, cost, budget)
	}
	spawned := make([]string, 0, len(foes))
	for _, c := range foes {
		s.g.Creatures = append(s.g.Creatures, c)
		s.u.art.Request(domain.MonsterFor(s.g, c))
		spawned = append(spawned, c.Name)
	}
	for _, r := range rules.StartEncounter(s.u.roller, s.g) {
		s.roll("", r)
	}
	s.cue(domain.CueCombat, "Combat begins! "+strings.Join(spawned, ", ")+" join the fray.")
	return "Combat started against " + strings.Join(spawned, ", ") + ". Narrate the opening in 1-2 sentences, then stop; the combat engine runs the turns.", nil
}

// tooTough refuses single monsters beyond the party's level, whatever they are called.
func tooTough(foes, party []*domain.Creature) error {
	limit := rules.MaxCR(party)
	for _, c := range foes {
		if rules.CRValue(c.CR) > limit {
			return fmt.Errorf("%w: %s (CR %s) is too dangerous at this level; the toughest monster allowed is CR %g. Save stronger foes for later and send weaker minions", domain.ErrInvalid, c.Name, c.CR, limit)
		}
	}
	return nil
}

func monsterName(base, custom string, i, count int) string {
	name := base
	if strings.TrimSpace(custom) != "" {
		name = strings.TrimSpace(custom)
	}
	if count > 1 {
		return fmt.Sprintf("%s %d", name, i+1)
	}
	return name
}

// monsterID is the next free ID for a monster of this kind, counting the
// pending ones not yet added to the game.
func (s *session) monsterID(key string, pending []*domain.Creature) string {
	taken := func(id string) bool {
		_, err := s.g.Creature(id)
		return err == nil || slices.ContainsFunc(pending, func(c *domain.Creature) bool { return c.ID == id })
	}
	for n := 1; ; n++ {
		if id := fmt.Sprintf("%s-%d", strings.ReplaceAll(strings.ToLower(key), " ", "_"), n); !taken(id) {
			return id
		}
	}
}

// maxChoices bounds the quick-choice buttons.
const maxChoices = 4

func (s *session) offerChoices(in dmInput) (string, error) {
	if !s.spoke {
		return "", fmt.Errorf("%w: the players have seen nothing yet. Write your narration as plain reply text first (they never see your thinking), then call offer_choices", domain.ErrInvalid)
	}
	var out []string
	for _, c := range in.Choices {
		c = strings.TrimSpace(c)
		if c == "" || len(c) > 120 {
			continue
		}
		out = append(out, c)
		if len(out) == maxChoices {
			break
		}
	}
	if len(out) == 0 {
		return "", fmt.Errorf("%w: give 2-4 short choices", domain.ErrInvalid)
	}
	s.g.Choices = out
	s.endTurn = true
	return "Shown to the players.", nil
}

func (s *session) endAdventure(in dmInput) (string, error) {
	s.g.Mode = domain.ModeOver
	s.emit(domain.Event{Type: domain.EvNarration, Actor: domain.DMSeat, Text: in.Epilogue})
	return "The adventure is over.", nil
}
