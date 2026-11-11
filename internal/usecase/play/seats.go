package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"time"
)

// owedSeat is the account's live character that still has to act this round.
func (s *session) owedSeat(accountID string) (*domain.Creature, error) {
	mine := false
	for _, c := range s.g.LiveSeats() {
		if domain.PlayerOf(c) != accountID {
			continue
		}
		mine = true
		if s.g.Owes(c) {
			return c, nil
		}
	}
	if mine {
		return nil, fmt.Errorf("%w: you have acted this round; waiting for the others", domain.ErrNotYourTurn)
	}
	return nil, fmt.Errorf("%w: your character is played by its agent; take control to act", domain.ErrNotYourTurn)
}

// unsubmitted names live seats that have not acted this round.
func (s *session) unsubmitted() []string {
	var out []string
	for _, c := range s.g.LiveSeats() {
		if s.g.Owes(c) {
			out = append(out, c.Name)
		}
	}
	return out
}

// resolveRound turns this round's submitted actions plus agent actions (for
// agent seats and absent players) into one message for the DM.
func (s *session) resolveRound(ctx context.Context) error {
	pending := s.g.Pending
	s.g.Pending, s.g.Deadline = nil, time.Time{}
	var lines []string
	said := ""
	for _, c := range s.g.PartyMembers() {
		if !c.CanAct() {
			continue
		}
		text, ok := pending[c.ID]
		label := "played by its owner"
		if !ok && s.holds(c) {
			s.system(c.Name + " holds back while their player is away.")
			lines = append(lines, fmt.Sprintf("- %s (%s, owner away): holds back and does nothing this round.", c.Name, c.ID))
			continue
		}
		if !ok {
			var err error
			if text, err = s.agentReacts(ctx, c, said); err != nil {
				s.system(fmt.Sprintf("%s hesitates (%v).", c.Name, err))
				continue
			}
			label = agentLabel(c)
			s.cover(c)
		}
		said += fmt.Sprintf("\n%s: %s", c.Name, domain.PlayerText(text))
		lines = append(lines, fmt.Sprintf("- %s (%s, %s): %s", c.Name, c.ID, label, domain.PlayerText(text)))
	}
	return s.dmTurn(ctx, partyMessage(s.g, lines))
}

func agentLabel(c *domain.Creature) string {
	if domain.Live(c) {
		return "played by its agent while its owner is away"
	}
	return "played by its agent"
}

// holds reports whether a live character's owner is away and has not let
// its agent stand in for them.
func (s *session) holds(c *domain.Creature) bool {
	return domain.Live(c) && s.g.Hold[c.ID]
}

// cover marks a live character as played by its agent while its owner is
// away, telling the table the first time.
func (s *session) cover(c *domain.Creature) {
	if !domain.Live(c) || s.g.Covering[c.ID] {
		return
	}
	if s.g.Covering == nil {
		s.g.Covering = map[string]bool{}
	}
	s.g.Covering[c.ID] = true
	s.system(c.Name + "'s agent covers for their player while they are away.")
}

// back clears the agent cover once a character's owner acts again.
func (s *session) back(c *domain.Creature) {
	delete(s.g.Covering, c.ID)
}

// agentReacts has a character's agent say and do something this round.
func (s *session) agentReacts(ctx context.Context, c *domain.Creature, said string) (string, error) {
	if said == "" {
		said = "(no one has acted yet)"
	}
	msg := fmt.Sprintf("The DM last described:\n%s\n\nYour companions this round:%s\n\nWhat do you say and do?", s.lastNarration(), said)
	return s.runAgent(ctx, s.companionRun(c, msg, noTools))
}

// pausedNote tells the table why nothing is happening.
const pausedNote = "The party's leader isn't playing a character, so the story pauses: their AI runs the Dungeon Master. It carries on when they play one of their characters."

// proceed carries the game on after seats change hands. With nobody left
// playing, the story pauses instead of running on agents alone.
func (s *session) proceed(ctx context.Context) error {
	if !s.g.Playing() {
		return s.pause()
	}
	switch s.g.Mode {
	case domain.ModeExploration:
		if len(s.unsubmitted()) > 0 {
			return nil
		}
		return s.resolveRound(ctx)
	case domain.ModeCombat:
		if c, err := s.g.Creature(s.g.Encounter.Current()); err == nil && domain.Live(c) && c.CanAct() && !s.g.Deadline.IsZero() {
			return nil // a player is already on the clock
		}
		return s.runCombat(ctx)
	}
	return nil
}

// pause stops the clock until a player takes control of a character.
func (s *session) pause() error {
	s.g.Deadline = time.Time{}
	s.system(pausedNote)
	return nil
}

// handOver gives a character to its agent.
func (s *session) handOver(c *domain.Creature) {
	c.Controller = domain.ControlAI
	delete(s.g.Pending, c.ID)
	s.back(c)
	s.system(c.Name + "'s agent takes over.")
}

// takeControl makes c the account's live character. A player plays one
// character at a time, so their other characters go to their agents.
func (s *session) takeControl(accountID string, c *domain.Creature) error {
	if c.Dead {
		return fmt.Errorf("%w: %s has fallen; choose another character", domain.ErrInvalid, c.Name)
	}
	for _, other := range s.g.PartyMembers() {
		if other != c && domain.PlayerOf(other) == accountID && domain.Live(other) {
			s.handOver(other)
		}
	}
	c.Controller = domain.ControlHuman
	s.back(c)
	s.system(c.Name + "'s player takes control.")
	return nil
}

// reseat moves a player whose live character has died onto another of their
// living characters in the party, so they stay at the table.
func (s *session) reseat() {
	for _, c := range s.g.PartyMembers() {
		if !domain.Live(c) || !c.Dead {
			continue
		}
		if next := s.spare(domain.PlayerOf(c)); next != nil {
			c.Controller, next.Controller = domain.ControlAI, domain.ControlHuman
			s.cue(domain.CueFall, fmt.Sprintf("%s has fallen. Their player now plays %s.", c.Name, next.Name))
		}
	}
}

// retireFallen takes the dead out of the party at the next quiet moment
// (outside combat): their agent is detached and the store sends them home as
// fallen. Someone living always remains; a whole fallen party ends the story.
func (s *session) retireFallen() {
	if s.g.Mode != domain.ModeExploration || !anyAlive(s.g) {
		return
	}
	for _, c := range s.g.PartyMembers() {
		if !c.Dead {
			continue
		}
		s.g.Leave(c)
		s.cue(domain.CueFall, c.Name+" has fallen and is laid to rest.")
		s.g.TableNotes = append(s.g.TableNotes, fmt.Sprintf("%s (%s) has died and is no longer with the party.", c.Name, c.ID))
	}
}

func anyAlive(g *domain.Game) bool {
	for _, c := range g.PartyMembers() {
		if !c.Dead {
			return true
		}
	}
	return false
}

// spare is a living agent-played character in the party that the account plays.
func (s *session) spare(accountID string) *domain.Creature {
	for _, c := range s.g.PartyMembers() {
		if domain.PlayerOf(c) == accountID && !domain.Live(c) && !c.Dead {
			return c
		}
	}
	return nil
}

// schedule sets when absent players' agents will step in.
func (s *session) schedule() {
	s.reseat()
	s.retireFallen()
	if s.g.Mode == domain.ModeExploration {
		s.comeRound()
	}
	switch {
	case s.g.Mode == domain.ModeOver || !s.g.Started || !s.g.Playing():
		s.g.Deadline = time.Time{}
	case s.g.Mode == domain.ModeExploration && s.g.Deadline.IsZero():
		s.g.Deadline = s.after(s.g.Pace.Explore)
	}
}

// after returns now+d, or no deadline when the pace waits for ever.
func (s *session) after(d time.Duration) time.Time {
	if d <= 0 {
		return time.Time{}
	}
	return s.u.clock.Now().Add(d)
}
