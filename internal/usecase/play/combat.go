package play

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"fmt"
	"strings"
	"time"
)

// maxCombatSteps guards against a fight that never ends.
const maxCombatSteps = 400

// dmTurn sends a message to the DM, lets it narrate and use tools, and runs any combat it starts.
func (s *session) dmTurn(ctx context.Context, message string) error {
	s.g.Choices = nil
	if len(s.g.TableNotes) > 0 {
		message = "Changes at the table: " + strings.Join(s.g.TableNotes, " ") + "\n\n" + message
		s.g.TableNotes = nil
	}
	_, err := s.runAgent(ctx, s.dmRun(message))
	if err != nil {
		return err
	}
	if s.g.Mode == domain.ModeCombat {
		return s.runCombat(ctx)
	}
	s.castCheck(ctx)
	s.suggestChoices(ctx)
	return nil
}

// dmRun is a DM turn that players watch being narrated.
func (s *session) dmRun(message string) seatRun {
	return seatRun{
		seat: domain.DMSeat, role: domain.RoleDM, payer: s.g.LeaderID, actor: domain.DMSeat, data: s.dmPrompt(), tools: dmTools(),
		message: message, final: domain.EvNarration, handle: s.dmTool,
	}
}

// choicesSeat is a throwaway seat used to suggest options when the DM forgot to.
const choicesSeat = "choices"

// suggestChoices makes sure the party always has quick options: if the DM handed
// the turn back without calling offer_choices, a small separate call writes them.
func (s *session) suggestChoices(ctx context.Context) {
	if s.g.Mode != domain.ModeExploration || len(s.g.Choices) > 0 {
		return
	}
	who := "the party"
	if seats := s.g.LiveSeats(); len(seats) > 0 {
		who = seats[0].Name
	}
	msg := fmt.Sprintf("The Dungeon Master just said:\n%s\n\nThe party and what each of them can do:\n%s\nCall offer_choices with 3-4 short, specific things %s could do next, each under 10 words and written as an action. Only suggest what the party can really do with the abilities, spells and items listed; never invent spells, items or powers. Write nothing else.", s.lastNarration(), partyStatus(s.g), who)
	_, err := s.runAgent(ctx, seatRun{
		seat: choicesSeat, role: domain.RoleChoices, payer: s.g.LeaderID, actor: domain.DMSeat, quiet: true,
		tools:   []domain.ToolSpec{offerChoicesSpec()},
		message: msg, final: domain.EvThinking,
		handle: func(call domain.ToolCall) (string, error) {
			if call.Name != "offer_choices" {
				return "", fmt.Errorf("%w: only offer_choices is available", domain.ErrInvalid)
			}
			return s.dmTool(call)
		},
	})
	delete(s.g.Transcripts, choicesSeat) // each suggestion starts fresh; nothing to keep
	if err != nil {
		s.u.log.Warn("could not suggest choices", "game", s.g.ID, "err", err)
	}
}

// runCombat plays out turns until it is the human's turn or the fight ends.
func (s *session) runCombat(ctx context.Context) error {
	for range maxCombatSteps {
		if result := rules.CombatResult(s.g); result != rules.Ongoing {
			return s.finishCombat(ctx, result)
		}
		s.reseat()
		if !s.g.Playing() {
			return s.pause()
		}
		c, err := s.g.Creature(s.g.Encounter.Current())
		if err != nil {
			return err
		}
		if domain.Live(c) && c.CanAct() && s.autoTurn != c.ID {
			s.g.Deadline = s.after(s.g.Pace.Combat)
			s.system(fmt.Sprintf("Round %d: %s's turn.", s.g.Encounter.Round, c.Name))
			return nil
		}
		s.autoTurn = ""
		if err := s.agentTurn(ctx, c); err != nil {
			return err
		}
		if err := s.advanceAndNarrate(ctx); err != nil {
			return err
		}
	}
	return fmt.Errorf("%w: combat ran too long", domain.ErrInvalid)
}

// agentTurn plays a turn nobody is at the table for: a companion's, a
// monster's, or an away player's (covered by their agent, or held back if
// they have not allowed that).
func (s *session) agentTurn(ctx context.Context, c *domain.Creature) error {
	if s.holds(c) {
		s.system(c.Name + " holds back while their player is away.")
		return nil
	}
	s.cover(c)
	return s.npcTurn(ctx, c)
}

// npcTurn runs an AI companion's or monster's turn. Creatures that cannot act just pass.
// Agent-played heroes use their class features by rule of thumb, ask their
// agent only for the main action, and spend any extra action on the best attack.
func (s *session) npcTurn(ctx context.Context, c *domain.Creature) error {
	if c.Kind != domain.KindPC {
		return s.playOut(c, s.bestAction(c))
	}
	err := s.playOut(c, s.featurePick(c, false))
	if opts := rules.ActionOptions(rules.Options(s.g, c.ID)); err == nil && len(opts) > 0 {
		opt, line := s.companionCombatChoice(ctx, c, opts)
		_, err = s.takeTurn(c, opt, line)
	}
	if err == nil {
		err = s.playOut(c, s.bestAction(c))
	}
	if err == nil {
		err = s.playOut(c, s.featurePick(c, true))
	}
	return err
}

// maxTurnSteps bounds how many options one creature takes in a turn.
const maxTurnSteps = 8

// playOut takes the options pick offers, one after another, until it offers
// none (or something fails).
func (s *session) playOut(c *domain.Creature, pick func() (rules.Option, bool)) error {
	var err error
	opt, ok := pick()
	for n := 0; ok && err == nil && n < maxTurnSteps; n++ {
		_, err = s.takeTurn(c, opt, "")
		opt, ok = pick()
	}
	return err
}

// bestAction picks the engine's best action while the creature has one left.
func (s *session) bestAction(c *domain.Creature) func() (rules.Option, bool) {
	return func() (rules.Option, bool) { return rules.MonsterChoice(s.u.roller, s.g, c.ID) }
}

// featurePick picks the class feature an agent-played hero would use next.
func (s *session) featurePick(c *domain.Creature, after bool) func() (rules.Option, bool) {
	return func() (rules.Option, bool) { return rules.AgentFeature(s.g, c.ID, after) }
}

// takeTurn resolves one combat option and reports it. It returns whether the
// creature's turn is over.
func (s *session) takeTurn(c *domain.Creature, opt rules.Option, line string) (bool, error) {
	if strings.TrimSpace(line) != "" {
		s.emit(domain.Event{Type: domain.EvSpeech, Actor: c.ID, Text: line})
	}
	out, err := rules.TakeTurn(s.u.roller, s.g, c.ID, opt)
	s.u.log.Debug("combat action", "game", s.g.ID, "round", s.g.Encounter.Round, "actor", c.ID, "controller", c.Controller,
		"action", opt.ActionID, "target", opt.TargetID, "hit", out.Hit, "amount", out.Amount, "err", err)
	if err != nil {
		return false, err
	}
	for _, r := range out.Rolls {
		s.roll(c.ID, r)
	}
	s.notes(out.Notes)
	s.g.RoundNotes = append(s.g.RoundNotes, out.Notes...)
	return out.TurnOver, nil
}

// advance moves to the next turn and reports any death save. It returns true when a new round began.
func (s *session) advance() bool {
	round := s.g.Encounter.Round
	roll, notes := rules.Advance(s.u.roller, s.g)
	if roll != nil {
		s.roll(s.g.Encounter.Current(), *roll)
	}
	s.notes(notes)
	s.g.RoundNotes = append(s.g.RoundNotes, notes...)
	return s.g.Encounter.Round != round
}

func (s *session) advanceAndNarrate(ctx context.Context) error {
	if !s.advance() {
		return nil
	}
	return s.narrateRound(ctx)
}

// narrateRound asks the DM to describe the round that just finished.
func (s *session) narrateRound(ctx context.Context) error {
	msg := fmt.Sprintf("Round %d of combat resolved by the engine:\n- %s\n\nBattlefield now:\n%s\nNarrate this round in 2-4 vivid sentences. Do not call tools and do not ask the party what they do.",
		s.g.Encounter.Round-1, strings.Join(s.g.RoundNotes, "\n- "), battlefield(s.g))
	s.g.RoundNotes = nil
	_, err := s.runAgent(ctx, s.dmRun(msg))
	return err
}

// finishCombat awards XP and hands the story back to the DM.
func (s *session) finishCombat(ctx context.Context, result rules.Result) error {
	s.g.Deadline = time.Time{}
	s.u.log.Debug("combat over", "game", s.g.ID, "result", result, "rounds", s.g.Encounter.Round)
	notes := strings.Join(s.g.RoundNotes, "\n- ")
	s.g.RoundNotes = nil
	xp := rules.EndEncounter(s.g)
	msg := fmt.Sprintf("Combat is over: %s.\nFinal moments:\n- %s\n\n", result, notes)
	if result == rules.Defeat {
		s.g.Mode = domain.ModeOver
		s.cue(domain.CueDefeat, "The party has fallen...")
		msg += "The whole party is down. Narrate their fate and call end_adventure with an epilogue."
		return s.dmTurn(ctx, msg+"\n"+partyStatus(s.g))
	}
	s.cue(domain.CueVictory, fmt.Sprintf("Victory! Each surviving hero earns %d XP.", xp))
	msg += s.grantXP(xp)
	msg += s.comeRound()
	msg += "Narrate the aftermath (loot with give_item if fitting) and invite the party to act."
	return s.dmTurn(ctx, msg+"\n"+partyStatus(s.g))
}

// comeRound wakes downed heroes once there is no fighting, so nobody is left
// unable to act (and the table never waits on them). It returns a note for the DM.
func (s *session) comeRound() string {
	var notes []string
	for _, c := range s.g.PartyMembers() {
		notes = append(notes, rules.ComeRound(c)...)
	}
	s.notes(notes)
	if len(notes) == 0 {
		return ""
	}
	return strings.Join(notes, " ") + "\n"
}
