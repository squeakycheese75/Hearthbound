package rules

import (
	"cmp"
	"dnd/internal/domain"
	"fmt"
	"slices"
	"strings"
)

// Grid size for encounters.
const (
	GridW = 12
	GridH = 8
)

// ApproachID is the pseudo-action for moving toward a target without acting.
const ApproachID = "approach"

// Option is one legal thing a combatant can do on its turn.
type Option struct {
	ActionID   string `json:"actionId"`
	ActionName string `json:"actionName"`
	TargetID   string `json:"targetId,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	Summary    string `json:"summary"`
	// Cost is what the option uses: the action (""), the bonus action, or nothing.
	Cost string `json:"cost,omitempty"`
}

// StartEncounter places combatants, rolls initiative and sets up the turn order.
func StartEncounter(r Roller, g *domain.Game) []domain.Roll {
	place(g)
	enc := &domain.Encounter{Round: 1, Initiative: map[string]int{}, GridW: GridW, GridH: GridH}
	var rolls []domain.Roll
	for _, c := range g.Creatures {
		if c.Dead {
			continue
		}
		roll := Initiative(r, c)
		rolls = append(rolls, roll)
		enc.Initiative[c.ID] = roll.Total
		enc.Order = append(enc.Order, c.ID)
	}
	dex := func(id string) int {
		c, _ := g.Creature(id)
		return c.Abilities.Score(domain.DEX)
	}
	slices.SortStableFunc(enc.Order, func(a, b string) int {
		return cmp.Or(cmp.Compare(enc.Initiative[b], enc.Initiative[a]), cmp.Compare(dex(b), dex(a)))
	})
	g.Encounter = enc
	g.Mode = domain.ModeCombat
	return rolls
}

func place(g *domain.Game) {
	pcRow, monRow := 0, 0
	for _, c := range g.Creatures {
		if c.Dead {
			continue
		}
		if c.Kind == domain.KindPC {
			c.Pos = domain.Position{X: 2, Y: (1 + pcRow*2) % GridH}
			pcRow++
			continue
		}
		c.Pos = domain.Position{X: 7 + monRow/(GridH/2), Y: (monRow * 2) % GridH}
		monRow++
	}
}

// Options lists every legal action for the creature whose turn it is.
func Options(g *domain.Game, actorID string) []Option {
	actor, err := g.Creature(actorID)
	if err != nil || !takesTurn(actor) {
		return nil
	}
	sp := spentFor(g, actor)
	if actor.Has(domain.Turned) {
		return fleeOptions(sp)
	}
	var out []Option
	switch {
	case midAttack(sp):
		out = weaponOptions(g, actor)
	case sp.ActionsLeft() > 0:
		out = mainOptions(g, actor)
	}
	out = append(out, featureOptions(g, actor, sp)...)
	if (sp.ActionsLeft() <= 0 || midAttack(sp)) && len(out) > 0 {
		out = append(out, Option{ActionID: EndTurnID, ActionName: "End turn", Cost: CostFree, Summary: "End your turn"})
	}
	return out
}

// mainOptions are the actor's actions: attacks, spells and moving up.
func mainOptions(g *domain.Game, actor *domain.Creature) []Option {
	var out []Option
	for _, act := range actor.Actions {
		out = append(out, optionsFor(g, actor, act)...)
	}
	for _, t := range g.Creatures {
		if domain.Hostile(actor, t) && !t.Down() {
			out = append(out, Option{ActionID: ApproachID, ActionName: "Move", TargetID: t.ID, TargetName: t.Name, Summary: "Move toward " + t.Name + " (no attack this turn)"})
		}
	}
	out = append(out, disengageOptions(g, actor)...)
	return append(out, potionOptions(g, actor)...)
}

// midAttack reports whether the second attack of an Extra Attack is still to come.
func midAttack(sp *domain.TurnSpent) bool {
	return sp.Attacks%2 == 1
}

// weaponOptions are the actor's weapon attacks, for the second attack of an action.
func weaponOptions(g *domain.Game, actor *domain.Creature) []Option {
	var out []Option
	for _, act := range actor.Actions {
		if weaponAttack(actor, act) {
			out = append(out, optionsFor(g, actor, act)...)
		}
	}
	return out
}

// ActionOptions keeps the options that use the action (attacks, spells, moving).
func ActionOptions(opts []Option) []Option {
	var out []Option
	for _, o := range opts {
		if o.Cost == CostAction {
			out = append(out, o)
		}
	}
	return out
}

func optionsFor(g *domain.Game, actor *domain.Creature, act domain.Action) []Option {
	if act.SlotLevel > 0 && actor.SlotFor(act.SlotLevel) == 0 {
		return nil
	}
	if act.Mass && len(woundedAllies(&turnCtx{g: g}, actor, act.RangeFt)) == 0 {
		return nil
	}
	if !act.NeedsTarget() {
		return []Option{{ActionID: act.ID, ActionName: act.Name, Summary: act.Name + slotNote(actor, act)}}
	}
	var out []Option
	for _, t := range g.Creatures {
		if !validTarget(actor, t, act) {
			continue
		}
		pos, ok := Approach(g, actor, t, act)
		if !ok {
			continue
		}
		out = append(out, Option{
			ActionID: act.ID, ActionName: act.Name, TargetID: t.ID, TargetName: t.Name,
			Summary: fmt.Sprintf("%s → %s (%s)%s%s", act.Name, t.Name, how(act, pos != actor.Pos), areaNote(g, actor, act, t), slotNote(actor, act)),
		})
	}
	return out
}

// areaNote says who an area spell would catch, so nobody fireballs a friend by accident.
func areaNote(g *domain.Game, actor *domain.Creature, act domain.Action, t *domain.Creature) string {
	if act.Radius == 0 {
		return ""
	}
	foes, friends := 0, 0
	for _, c := range InArea(g, t.Pos, act.Radius) {
		if domain.Hostile(actor, c) {
			foes++
			continue
		}
		friends++
	}
	note := fmt.Sprintf(" — %d-ft burst: %d foe(s)", act.Radius, foes)
	if friends > 0 && !actor.HasFeature(domain.FeatSculptSpells) {
		note += fmt.Sprintf(" and %d ally(ies)", friends)
	}
	return note
}

// slotNote says when a spell would use a higher slot than its own.
func slotNote(actor *domain.Creature, act domain.Action) string {
	if act.SlotLevel == 0 {
		return ""
	}
	if used := actor.SlotFor(act.SlotLevel); used > act.SlotLevel {
		return fmt.Sprintf(" [level %d slot]", used)
	}
	return ""
}

func validTarget(actor, t *domain.Creature, act domain.Action) bool {
	if t.Dead {
		return false
	}
	if act.TargetsAllies() {
		return !domain.Hostile(actor, t) && t.HP < t.MaxHP
	}
	return domain.Hostile(actor, t) && !t.Down()
}

// how says whether an option is melee or ranged and whether the actor must
// move first, so agents can fight the way their class should.
func how(act domain.Action, moves bool) string {
	switch {
	case act.TargetsAllies() && moves:
		return "moves to reach them"
	case act.TargetsAllies():
		return "in reach"
	case act.RangeFt <= 5 && moves:
		return "melee: moves up to the foe"
	case act.RangeFt <= 5:
		return "melee"
	case moves:
		return fmt.Sprintf("ranged %d ft: moves into range", act.RangeFt)
	}
	return fmt.Sprintf("ranged %d ft", act.RangeFt)
}

// Approach finds where actor should stand to use act on target this turn.
// It returns the actor's current square if already in range, and false when unreachable.
func Approach(g *domain.Game, actor, target *domain.Creature, act domain.Action) (domain.Position, bool) {
	if InRange(actor, target, act) {
		return actor.Pos, true
	}
	reach := max(act.RangeFt, 5)
	return bestSquare(g, actor, func(p domain.Position) (int, bool) {
		return actor.Pos.DistanceFt(p), p.DistanceFt(target.Pos) <= reach
	})
}

// MoveToward moves actor as far toward target as its speed allows.
func MoveToward(g *domain.Game, actor, target *domain.Creature) domain.Position {
	p, _ := bestSquare(g, actor, func(p domain.Position) (int, bool) {
		return p.DistanceFt(target.Pos), true
	})
	return p
}

// bestSquare scans reachable unoccupied squares and returns the lowest-scoring acceptable one.
func bestSquare(g *domain.Game, actor *domain.Creature, score func(domain.Position) (int, bool)) (domain.Position, bool) {
	steps := (SpeedOf(actor) + spentFor(g, actor).Dash) / 5
	best, bestScore, found := actor.Pos, 1<<30, false
	for x := max(0, actor.Pos.X-steps); x <= min(GridW-1, actor.Pos.X+steps); x++ {
		for y := max(0, actor.Pos.Y-steps); y <= min(GridH-1, actor.Pos.Y+steps); y++ {
			p := domain.Position{X: x, Y: y}
			if occupied(g, actor, p) {
				continue
			}
			s, ok := score(p)
			if ok && s < bestScore {
				best, bestScore, found = p, s, true
			}
		}
	}
	return best, found
}

func occupied(g *domain.Game, self *domain.Creature, p domain.Position) bool {
	for _, c := range g.Creatures {
		if c != self && !c.Dead && c.Pos == p {
			return true
		}
	}
	return false
}

// TakeTurn carries out an option for the current combatant: move if needed,
// then act. Outcome.TurnOver says whether their turn has ended; a bonus action
// or a free feature leaves it open.
func TakeTurn(r Roller, g *domain.Game, actorID string, opt Option) (Outcome, error) {
	actor, err := g.Creature(actorID)
	if err != nil {
		return Outcome{}, err
	}
	if g.Encounter.Current() != actorID {
		return Outcome{}, fmt.Errorf("%w: %s", domain.ErrNotYourTurn, actor.Name)
	}
	// Match on what the option does, not its description, which can change as creatures move.
	i := slices.IndexFunc(Options(g, actorID), func(o Option) bool { return o.ActionID == opt.ActionID && o.TargetID == opt.TargetID })
	if i < 0 {
		return Outcome{}, fmt.Errorf("%w: option %q on %q is not available", domain.ErrInvalid, opt.ActionID, opt.TargetID)
	}
	opt = Options(g, actorID)[i]
	out, err := perform(r, g, actor, opt)
	if err != nil {
		return Outcome{}, err
	}
	spend(&g.Encounter.Spent, actor, opt)
	out.TurnOver = opt.ActionID == EndTurnID || !hasMore(g, actor)
	return out, nil
}

// perform carries out one option.
func perform(r Roller, g *domain.Game, actor *domain.Creature, opt Option) (Outcome, error) {
	if f, ok := featureActs[opt.ActionID]; ok {
		return f(r, g, actor, opt)
	}
	switch opt.ActionID {
	case EndTurnID:
		return Outcome{}, nil
	case ApproachID:
		target, _ := g.Creature(opt.TargetID)
		out, err := moveTo(r, g, actor, MoveToward(g, actor, target))
		out.Notes = append(out.Notes, fmt.Sprintf("%s moves toward %s.", actor.Name, target.Name))
		return out, err
	}
	act, _ := actor.Action(strings.TrimPrefix(opt.ActionID, FrenzyPrefix))
	if !act.NeedsTarget() {
		return resolve(r, &turnCtx{g: g}, actor, act, actor)
	}
	target, _ := g.Creature(opt.TargetID)
	pos, _ := Approach(g, actor, target, act)
	moved, err := moveTo(r, g, actor, pos)
	if err != nil || actor.Down() {
		return moved, err
	}
	out, err := resolve(r, &turnCtx{g: g}, actor, act, target)
	return merge(moved, out), err
}

// spend records what an option used up. With Extra Attack, the Attack
// action is spent only after its second attack.
func spend(sp *domain.TurnSpent, actor *domain.Creature, opt Option) {
	switch opt.Cost {
	case CostBonus:
		sp.Bonus = true
	case CostAction:
		if extraAttack(actor, opt) {
			sp.Attacks++
			if midAttack(sp) {
				return
			}
		}
		sp.Actions++
	}
}

func extraAttack(actor *domain.Creature, opt Option) bool {
	act, err := actor.Action(opt.ActionID)
	return err == nil && weaponAttack(actor, act) && actor.HasFeature(domain.FeatExtraAttack)
}

// weaponAttack reports whether an action is a weapon attack: a hero's weapon,
// or any of a monster's attacks (a Multiattack monster makes two).
func weaponAttack(actor *domain.Creature, act domain.Action) bool {
	return act.Weapon || (actor.Kind == domain.KindMonster && act.Kind == domain.ActAttack)
}

// hasMore reports whether the actor can still do something this turn.
func hasMore(g *domain.Game, actor *domain.Creature) bool {
	return slices.ContainsFunc(Options(g, actor.ID), func(o Option) bool { return o.ActionID != EndTurnID })
}

// Advance moves to the next living combatant and starts its turn. A dying PC
// rolls its death save; the roll (if any) and notes are returned.
func Advance(r Roller, g *domain.Game) (*domain.Roll, []string) {
	enc := g.Encounter
	for range len(enc.Order) {
		enc.Turn++
		if enc.Turn%len(enc.Order) == 0 {
			enc.Round++
		}
		enc.Spent = domain.TurnSpent{}
		c, err := g.Creature(enc.Current())
		if err != nil || c.Dead {
			continue
		}
		delete(enc.Reacted, c.ID) // reactions come back at the start of your turn
		c.RemoveCondition(domain.Dodging)
		c.RemoveCondition(domain.Reckless)
		if Dying(c) {
			roll, notes := DeathSave(r, c)
			return &roll, notes
		}
		if takesTurn(c) {
			return nil, nil
		}
	}
	return nil, nil
}

// takesTurn reports whether a creature gets a turn: it can act, or it is
// turned and must spend its turn fleeing.
func takesTurn(c *domain.Creature) bool {
	return c.CanAct() || (c.Has(domain.Turned) && !c.Down() && !c.Has(domain.Unconscious))
}

// endFightStates clears what only lasts a fight. A frenzy leaves the
// barbarian exhausted until a long rest.
func endFightStates(c *domain.Creature) {
	if c.Has(domain.Frenzied) {
		c.AddCondition(domain.Exhausted)
	}
	for _, cond := range []domain.Condition{domain.Dodging, domain.Reckless, domain.Hidden, domain.Turned, domain.Raging, domain.Frenzied} {
		c.RemoveCondition(cond)
	}
}

// Result is how a combat ended.
type Result string

// Combat results.
const (
	Ongoing Result = ""
	Victory Result = "victory"
	Defeat  Result = "defeat"
)

// CombatResult reports whether either side has been defeated.
func CombatResult(g *domain.Game) Result {
	monstersUp, partyUp := false, false
	for _, c := range g.Creatures {
		if c.Down() {
			continue
		}
		if c.Kind == domain.KindMonster {
			monstersUp = true
			continue
		}
		partyUp = true
	}
	switch {
	case !partyUp:
		return Defeat
	case !monstersUp:
		return Victory
	}
	return Ongoing
}

// EndEncounter clears combat state, removes dead monsters and returns XP earned per PC.
func EndEncounter(g *domain.Game) int {
	total := 0
	g.Creatures = slices.DeleteFunc(g.Creatures, func(c *domain.Creature) bool {
		if c.Kind == domain.KindMonster && c.Dead {
			total += c.XPValue
			return true
		}
		return false
	})
	for _, c := range g.Creatures {
		endFightStates(c)
	}
	g.Encounter = nil
	g.Mode = domain.ModeExploration
	alive := 0
	for _, c := range g.PartyMembers() {
		if !c.Dead {
			alive++
		}
	}
	if alive == 0 {
		return 0
	}
	return total / alive
}

// MonsterChoice picks a sensible option for a DM-controlled creature: the most
// damaging attack against the most wounded reachable foe, otherwise close in.
func MonsterChoice(r Roller, g *domain.Game, actorID string) (Option, bool) {
	opts := ActionOptions(Options(g, actorID))
	if len(opts) == 0 {
		return Option{}, false
	}
	actor, _ := g.Creature(actorID)
	if chance := erraticChance(actor); chance > 0 && r.Roll(100) <= chance {
		attacks := attacksIn(opts)
		return attacks[r.Roll(len(attacks))-1], true
	}
	best, bestScore := opts[0], -1<<30
	for _, o := range opts {
		s := scoreOption(g, actor, o)
		if s > bestScore {
			best, bestScore = o, s
		}
	}
	return best, true
}

// erraticChance is the percent chance a monster forgets its tactics this turn
// and lashes out at whatever is handy: the dimmer it is, the likelier. Monsters
// without an Intelligence score, and player characters, always play properly.
func erraticChance(c *domain.Creature) int {
	intel := c.Abilities[domain.INT]
	switch {
	case c.Kind != domain.KindMonster || intel == 0 || intel >= 11:
		return 0
	case intel <= 4:
		return 40
	case intel <= 7:
		return 25
	}
	return 10
}

// attacksIn keeps the options that act rather than just move, if there are any.
func attacksIn(opts []Option) []Option {
	var out []Option
	for _, o := range opts {
		if o.ActionID != ApproachID {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return opts
	}
	return out
}

func scoreOption(g *domain.Game, actor *domain.Creature, o Option) int {
	if o.ActionID == DisengageID || o.ActionID == PotionID {
		return -2000 // the engine's tactics press the attack; agents choose these themselves
	}
	target, err := g.Creature(o.TargetID)
	if err != nil {
		return -1000
	}
	dist := actor.Pos.DistanceFt(target.Pos)
	if o.ActionID == ApproachID {
		return -500 - dist
	}
	act, _ := actor.Action(o.ActionID)
	damage := act.Dice.Average() * 10
	melee := act.RangeFt <= 5
	switch actor.Tactic {
	case domain.TacticBrute:
		return damage - dist + bonus(melee, 50)
	case domain.TacticSkirmisher:
		return damage - target.HP*2 + bonus(!melee, 60)
	case domain.TacticPack:
		return damage - target.HP + 40*alliesOn(g, actor, target)
	case domain.TacticCunning:
		return damage - target.AC*8 - target.HP*2
	case domain.TacticMindless:
		return damage - dist*5
	}
	return damage - target.HP
}

// bonus is n when the condition holds.
func bonus(cond bool, n int) int {
	if cond {
		return n
	}
	return 0
}

// alliesOn counts the actor's allies already next to the target.
func alliesOn(g *domain.Game, actor, target *domain.Creature) int {
	n := 0
	for _, c := range g.Creatures {
		if c != actor && !c.Down() && !domain.Hostile(actor, c) && c.Pos.DistanceFt(target.Pos) <= 5 {
			n++
		}
	}
	return n
}
