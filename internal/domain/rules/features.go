package rules

import (
	"dnd/internal/domain"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Option costs: what an option uses up from the turn.
const (
	CostAction = ""
	CostBonus  = "bonus"
	CostFree   = "free"
)

// Option IDs for class features and ending a turn.
const (
	EndTurnID      = "end_turn"
	SecondWindID   = "second_wind"
	ActionSurgeID  = "action_surge"
	RecklessID     = "reckless_attack"
	RageID         = "rage"
	FrenzyRageID   = "rage_frenzy"
	FrenzyPrefix   = "frenzy:"
	HideID         = "hide"
	DashID         = "dash"
	InspireID      = "bardic_inspiration"
	PreserveLifeID = "preserve_life"
	TurnUndeadID   = "turn_undead"
)

// spentFor is the turn record for the actor: the encounter's when it is
// their turn, otherwise a blank one.
func spentFor(g *domain.Game, actor *domain.Creature) *domain.TurnSpent {
	if g.Encounter != nil && g.Encounter.Current() == actor.ID {
		return &g.Encounter.Spent
	}
	return &domain.TurnSpent{}
}

// featureOptions are the class-feature options open to the actor right now.
func featureOptions(g *domain.Game, a *domain.Creature, sp *domain.TurnSpent) []Option {
	var out []Option
	if sp.ActionsLeft() > 0 {
		out = append(out, beforeAction(g, a, sp)...)
	}
	if !sp.Bonus {
		out = append(out, bonusOptions(g, a, sp)...)
	}
	return out
}

// beforeAction are features used with, or before, the action.
func beforeAction(g *domain.Game, a *domain.Creature, sp *domain.TurnSpent) []Option {
	var out []Option
	if a.FeatureLeft(domain.FeatActionSurge) > 0 && !slices.Contains(sp.Free, domain.FeatActionSurge) {
		out = append(out, Option{ActionID: ActionSurgeID, ActionName: "Action Surge", Cost: CostFree, Summary: "Action Surge: take one more action this turn"})
	}
	if a.HasFeature(domain.FeatRecklessAttack) && !a.Has(domain.Reckless) && sp.Actions == 0 && strengthMelee(a) {
		out = append(out, Option{ActionID: RecklessID, ActionName: "Reckless Attack", Cost: CostFree, Summary: "Reckless Attack: advantage on your melee attacks, but foes get advantage against you"})
	}
	return append(out, channelOptions(g, a, sp)...)
}

// channelOptions are a cleric's Channel Divinity actions.
func channelOptions(g *domain.Game, a *domain.Creature, sp *domain.TurnSpent) []Option {
	if a.FeatureLeft(domain.FeatChannelDivinity) == 0 || midAttack(sp) {
		return nil
	}
	var out []Option
	if len(preserveTargets(g, a)) > 0 {
		out = append(out, Option{ActionID: PreserveLifeID, ActionName: "Preserve Life", Summary: fmt.Sprintf("Channel Divinity: Preserve Life (heal up to %d among badly hurt allies)", 5*a.Level)})
	}
	if len(undeadNear(g, a)) > 0 {
		out = append(out, Option{ActionID: TurnUndeadID, ActionName: "Turn Undead", Summary: "Channel Divinity: Turn Undead (undead within 30 ft must save or flee)"})
	}
	return out
}

// bonusOptions are the bonus actions open to the actor.
func bonusOptions(g *domain.Game, a *domain.Creature, sp *domain.TurnSpent) []Option {
	var out []Option
	if a.FeatureLeft(domain.FeatSecondWind) > 0 && a.HP < a.MaxHP {
		out = append(out, Option{ActionID: SecondWindID, ActionName: "Second Wind", Cost: CostBonus, Summary: fmt.Sprintf("Second Wind: regain 1d10+%d HP", a.Level)})
	}
	out = append(out, rageOptions(g, a, sp)...)
	if a.HasFeature(domain.FeatCunningAction) {
		if !a.Has(domain.Hidden) {
			out = append(out, Option{ActionID: HideID, ActionName: "Hide", Cost: CostBonus, Summary: "Cunning Action: Hide (Stealth check; your next attack has advantage)"})
		}
		out = append(out, Option{ActionID: DashID, ActionName: "Dash", Cost: CostBonus, Summary: "Cunning Action: Dash (double your movement this turn)"})
	}
	return append(out, inspireOptions(g, a)...)
}

func rageOptions(g *domain.Game, a *domain.Creature, sp *domain.TurnSpent) []Option {
	switch {
	case a.Has(domain.Raging) && a.Has(domain.Frenzied) && !sp.Raged:
		return frenzyStrikes(g, a)
	case a.Has(domain.Raging) || a.FeatureLeft(domain.FeatRage) == 0:
		return nil
	}
	out := []Option{{ActionID: RageID, ActionName: "Rage", Cost: CostBonus, Summary: "Rage: +2 melee damage, resist physical damage"}}
	if a.HasFeature(domain.FeatFrenzy) {
		out = append(out, Option{ActionID: FrenzyRageID, ActionName: "Frenzied Rage", Cost: CostBonus, Summary: "Frenzied Rage: rage, and make a bonus melee attack on each later turn (exhausted afterwards)"})
	}
	return out
}

// frenzyStrikes are bonus melee attacks for a frenzied barbarian.
func frenzyStrikes(g *domain.Game, a *domain.Creature) []Option {
	var out []Option
	for _, act := range a.Actions {
		if !act.Weapon || act.Ranged() {
			continue
		}
		for _, o := range optionsFor(g, a, act) {
			o.ActionID, o.ActionName, o.Cost = FrenzyPrefix+o.ActionID, "Frenzied "+o.ActionName, CostBonus
			o.Summary = "Frenzy: " + o.Summary
			out = append(out, o)
		}
	}
	return out
}

func inspireOptions(g *domain.Game, a *domain.Creature) []Option {
	if a.FeatureLeft(domain.FeatBardicInspiration) == 0 {
		return nil
	}
	var out []Option
	for _, t := range g.Creatures {
		if t == a || domain.Hostile(a, t) || t.Down() || t.Inspiration > 0 || a.Pos.DistanceFt(t.Pos) > 60 {
			continue
		}
		out = append(out, Option{
			ActionID: InspireID, ActionName: "Bardic Inspiration", TargetID: t.ID, TargetName: t.Name, Cost: CostBonus,
			Summary: fmt.Sprintf("Bardic Inspiration → %s (a d%d for a roll that would fail)", t.Name, a.InspirationDie()),
		})
	}
	return out
}

// strengthMelee reports whether the creature has a Strength melee weapon attack.
func strengthMelee(a *domain.Creature) bool {
	return slices.ContainsFunc(a.Actions, func(act domain.Action) bool {
		return act.Weapon && !act.Ranged() && act.Stat == domain.STR
	})
}

// preserveTargets are allies within 30 ft below half their hit points.
func preserveTargets(g *domain.Game, a *domain.Creature) []*domain.Creature {
	var out []*domain.Creature
	for _, t := range g.Creatures {
		if !t.Dead && !domain.Hostile(a, t) && t.HP < t.MaxHP/2 && a.Pos.DistanceFt(t.Pos) <= 30 {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].HP < out[j].HP })
	return out
}

// undeadNear are hostile undead within 30 ft that are still standing and not turned.
func undeadNear(g *domain.Game, a *domain.Creature) []*domain.Creature {
	var out []*domain.Creature
	for _, t := range g.Creatures {
		if t.Type == "undead" && domain.Hostile(a, t) && !t.Down() && !t.Has(domain.Turned) && a.Pos.DistanceFt(t.Pos) <= 30 {
			out = append(out, t)
		}
	}
	return out
}

// featureActs carries out each feature option.
var featureActs = map[string]func(Roller, *domain.Game, *domain.Creature, Option) (Outcome, error){
	SecondWindID:   secondWind,
	ActionSurgeID:  actionSurge,
	RecklessID:     reckless,
	RageID:         rage,
	FrenzyRageID:   rage,
	HideID:         hide,
	DashID:         dash,
	InspireID:      inspire,
	PreserveLifeID: preserveLife,
	TurnUndeadID:   turnUndead,
	PotionID:       usePotion,
	FleeID:         flee,
	DisengageID:    disengage,
}

func secondWind(r Roller, _ *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	if err := a.SpendFeature(domain.FeatSecondWind); err != nil {
		return Outcome{}, err
	}
	d := domain.Dice{Count: 1, Sides: 10, Bonus: a.Level}
	total, rolls := RollDice(r, d)
	roll := domain.Roll{Label: a.Name + " catches a Second Wind", Dice: d.String(), Rolls: rolls, Modifier: d.Bonus, Total: total}
	return Outcome{Rolls: []domain.Roll{roll}, Hit: true, Amount: total, Notes: Heal(a, total)}, nil
}

func actionSurge(_ Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	if err := a.SpendFeature(domain.FeatActionSurge); err != nil {
		return Outcome{}, err
	}
	sp := spentFor(g, a)
	sp.Extra++
	sp.Free = append(sp.Free, domain.FeatActionSurge)
	return Outcome{Notes: []string{a.Name + " surges with renewed vigour: one more action this turn!"}}, nil
}

func reckless(_ Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	a.AddCondition(domain.Reckless)
	sp := spentFor(g, a)
	sp.Free = append(sp.Free, domain.FeatRecklessAttack)
	return Outcome{Notes: []string{a.Name + " attacks recklessly, heedless of defence."}}, nil
}

func rage(_ Roller, g *domain.Game, a *domain.Creature, o Option) (Outcome, error) {
	if err := a.SpendFeature(domain.FeatRage); err != nil {
		return Outcome{}, err
	}
	a.AddCondition(domain.Raging)
	note := a.Name + " flies into a rage!"
	if o.ActionID == FrenzyRageID {
		a.AddCondition(domain.Frenzied)
		note = a.Name + " flies into a frenzied rage!"
	}
	spentFor(g, a).Raged = true
	return Outcome{Notes: []string{note}}, nil
}

func hide(r Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	dc := 0
	for _, t := range g.Creatures {
		if domain.Hostile(a, t) && t.CanAct() {
			dc = max(dc, PassivePerception(t))
		}
	}
	roll := AbilityCheck(r, a, domain.DEX, domain.Stealth, max(dc, 1), Normal)
	roll.Label = a.Name + " tries to hide"
	if *roll.Success {
		a.AddCondition(domain.Hidden)
		return Outcome{Rolls: []domain.Roll{roll}, Hit: true, Notes: []string{a.Name + " slips out of sight."}}, nil
	}
	return Outcome{Rolls: []domain.Roll{roll}, Notes: []string{a.Name + " fails to find cover."}}, nil
}

// PassivePerception is 10 + Wisdom (Perception).
func PassivePerception(c *domain.Creature) int {
	return 10 + CheckModifier(c, domain.WIS, domain.Perception)
}

func dash(_ Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	spentFor(g, a).Dash += SpeedOf(a)
	return Outcome{Notes: []string{a.Name + " darts about, covering double the ground."}}, nil
}

func inspire(_ Roller, g *domain.Game, a *domain.Creature, o Option) (Outcome, error) {
	t, err := g.Creature(o.TargetID)
	if err != nil {
		return Outcome{}, err
	}
	if err := a.SpendFeature(domain.FeatBardicInspiration); err != nil {
		return Outcome{}, err
	}
	t.Inspiration = a.InspirationDie()
	return Outcome{Notes: []string{fmt.Sprintf("%s inspires %s (d%d).", a.Name, t.Name, t.Inspiration)}}, nil
}

func preserveLife(_ Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	if err := a.SpendFeature(domain.FeatChannelDivinity); err != nil {
		return Outcome{}, err
	}
	budget := 5 * a.Level
	out := Outcome{Hit: true, Notes: []string{a.Name + " channels divine power to preserve life."}}
	for _, t := range preserveTargets(g, a) {
		amount := min(budget, t.MaxHP/2-t.HP)
		if amount <= 0 {
			continue
		}
		budget -= amount
		out.Amount += amount
		out.Notes = append(out.Notes, Heal(t, amount)...)
	}
	return out, nil
}

func turnUndead(r Roller, g *domain.Game, a *domain.Creature, _ Option) (Outcome, error) {
	if err := a.SpendFeature(domain.FeatChannelDivinity); err != nil {
		return Outcome{}, err
	}
	dc := 8 + a.ProfBonus() + a.Abilities.Mod(domain.WIS)
	out := Outcome{Notes: []string{a.Name + " presents a holy symbol: the dead must flee!"}}
	for _, t := range undeadNear(g, a) {
		save := SavingThrow(r, t, domain.WIS, dc, Normal)
		out.Rolls = append(out.Rolls, save)
		if *save.Success {
			out.Notes = append(out.Notes, t.Name+" resists.")
			continue
		}
		out.Hit = true
		if a.HasFeature(domain.FeatDestroyUndead) && domain.ChallengeValue(t.CR) <= 0.5 {
			t.HP, t.Dead = 0, true
			out.Notes = append(out.Notes, t.Name+" crumbles to dust, destroyed!")
			continue
		}
		t.AddCondition(domain.Turned)
		out.Notes = append(out.Notes, t.Name+" is turned and flees.")
	}
	return out, nil
}

// ArcaneRecovery restores a wizard's spent slots during a short rest, once a
// day: slot levels adding up to half their level, rounded up, highest first.
func ArcaneRecovery(c *domain.Creature) []string {
	if c.FeatureLeft(domain.FeatArcaneRecovery) == 0 || len(c.SlotsUsed) == 0 {
		return nil
	}
	budget := (c.Level + 1) / 2
	recovered := 0
	for lvl := 5; lvl >= 1; lvl-- {
		for c.SlotsUsed[lvl] > 0 && lvl <= budget {
			c.SlotsUsed[lvl]--
			budget -= lvl
			recovered++
		}
	}
	if recovered == 0 {
		return nil
	}
	_ = c.SpendFeature(domain.FeatArcaneRecovery)
	return []string{fmt.Sprintf("%s's Arcane Recovery restores %d spell slot(s).", c.Name, recovered)}
}

// SongOfRest is the extra healing a bard's song gives each hero on a short
// rest: 1d6 when a living bard with the feature is in the party, else 0.
func SongOfRest(r Roller, party []*domain.Creature) int {
	for _, c := range party {
		if !c.Dead && c.HasFeature(domain.FeatSongOfRest) {
			total, _ := RollDice(r, domain.Dice{Count: 1, Sides: 6})
			return total
		}
	}
	return 0
}

// AgentFeature is the next class feature an agent-played hero sensibly uses:
// before its action (after=false) or after it. Monsters have none.
func AgentFeature(g *domain.Game, actorID string, after bool) (Option, bool) {
	a, err := g.Creature(actorID)
	if err != nil || !a.CanAct() {
		return Option{}, false
	}
	opts := Options(g, actorID)
	for _, id := range agentPlan(a, after) {
		for _, o := range opts {
			if o.ActionID == id || (id == FrenzyPrefix && strings.HasPrefix(o.ActionID, FrenzyPrefix)) {
				return o, true
			}
		}
	}
	return Option{}, false
}

// agentPlan lists the features worth using, in order, for an agent.
func agentPlan(a *domain.Creature, after bool) []string {
	if after {
		return []string{FrenzyPrefix, HideID, InspireID}
	}
	plan := []string{RageID, ActionSurgeID, PreserveLifeID}
	if a.HP*2 <= a.MaxHP {
		plan = append(plan, SecondWindID)
	}
	if a.HP*2 > a.MaxHP {
		plan = append(plan, RecklessID)
	}
	return append(plan, TurnUndeadID, InspireID)
}

// SpeedOf is a creature's walking speed, with a barbarian's Fast Movement.
func SpeedOf(c *domain.Creature) int {
	if c.HasFeature(domain.FeatFastMovement) {
		return c.Speed + 10
	}
	return c.Speed
}
