package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
)

// errNotHere explains where the party can change.
var errNotHere = fmt.Errorf("%w: the party can only change in a tavern, town or village, outside combat", domain.ErrInvalid)

// Recruit adds an idle roster hero to the party as an agent-played companion:
// one of the leader's own, or another player's hero whose owner lets anyone
// recruit them (HireAuto); that hero's agent runs on its owner's key. It only
// works at a tavern (see Game.AtTavern); the DM introduces the newcomer next.
func (u *UseCase) Recruit(ctx context.Context, accountID, gameID, characterID string) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		g := s.g
		if g.LeaderID != accountID {
			return fmt.Errorf("%w: only the party's leader can recruit", domain.ErrForbidden)
		}
		if err := canChange(g); err != nil {
			return err
		}
		if len(g.Party) >= domain.MaxParty {
			return fmt.Errorf("%w: the party is full (%d)", domain.ErrInvalid, domain.MaxParty)
		}
		ch, err := u.store.Character(ctx, characterID)
		if err != nil {
			return fmt.Errorf("load character: %w", err)
		}
		if err := mayRecruit(ch, accountID); err != nil {
			return err
		}
		return s.welcome(ch)
	})
}

// welcome brings a roster hero into the party, if they're free to come.
func (s *session) welcome(ch domain.Character) error {
	g := s.g
	if !ch.Available() {
		return fmt.Errorf("%w: %s is not free to join (%s)", domain.ErrInvalid, ch.Sheet.Name, ch.Status)
	}
	g.DropCall(ch.ID)
	g.Join(ch)
	c, _ := g.Creature(ch.ID)
	s.u.art.Request(domain.PortraitFor(g, c))
	s.cue(domain.CueArrive, fmt.Sprintf("%s joins the party.", c.Name))
	g.TableNotes = append(g.TableNotes, fmt.Sprintf("%s (%s, a level %d %s %s) has just joined the party here; introduce them.",
		c.Name, c.ID, c.Level, c.Race, c.Class))
	return nil
}

// SendHome takes one of the account's characters out of the party; they go
// back to the roster with what they earned. Their player must be playing
// someone else, and the party keeps at least one member.
func (u *UseCase) SendHome(ctx context.Context, accountID, gameID, characterID string) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		g := s.g
		c, err := g.SeatFor(accountID, characterID)
		if err != nil {
			return err
		}
		if err := canChange(g); err != nil {
			return err
		}
		if domain.Live(c) && !c.Dead {
			return fmt.Errorf("%w: you are playing %s; play as someone else first", domain.ErrInvalid, c.Name)
		}
		if len(g.Party) <= 1 {
			return fmt.Errorf("%w: the party needs at least one member", domain.ErrInvalid)
		}
		g.Leave(c)
		s.cue(domain.CueLeave, c.Name+" leaves the party and heads home.")
		g.TableNotes = append(g.TableNotes, c.Name+" ("+c.ID+") has left the party and gone home.")
		return nil
	})
}

// mayRecruit checks the hero's owner lets this account recruit them.
func mayRecruit(ch domain.Character, accountID string) error {
	switch {
	case ch.OwnerID == accountID, ch.Hire == domain.HireAuto:
		return nil
	case ch.Hire == domain.HireAsk:
		return fmt.Errorf("%w: %s's owner wants to be asked first", domain.ErrForbidden, ch.Sheet.Name)
	}
	return fmt.Errorf("%w: character %s", domain.ErrNotFound, ch.ID)
}

func canChange(g *domain.Game) error {
	if !g.AtTavern() {
		return errNotHere
	}
	return nil
}
