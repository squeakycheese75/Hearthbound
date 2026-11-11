package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"slices"
)

// OfferSeat opens one of the party's agent-played companions to friends on
// the tavern's notice board, or closes the offer. Only the leader offers
// seats, and not during a fight.
func (u *UseCase) OfferSeat(ctx context.Context, accountID, gameID, characterID string, open bool) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.LeaderID != accountID {
			return fmt.Errorf("%w: only the party's leader offers seats", domain.ErrForbidden)
		}
		c, err := s.partyMember(characterID)
		if err != nil {
			return err
		}
		if !open {
			s.g.OpenSeats = slices.DeleteFunc(s.g.OpenSeats, func(id string) bool { return id == c.ID })
			return nil
		}
		if err := s.offerable(c); err != nil {
			return err
		}
		if !s.g.SeatOpen(c.ID) {
			s.g.OpenSeats = append(s.g.OpenSeats, c.ID)
			s.system(fmt.Sprintf("A seat at the table is open: %s is looking for a player.", c.Name))
		}
		return nil
	})
}

// offerable checks a companion can be offered to a friend.
func (s *session) offerable(c *domain.Creature) error {
	switch {
	case s.g.Mode != domain.ModeExploration:
		return fmt.Errorf("%w: offer seats outside a fight", domain.ErrWrongMode)
	case c.Dead:
		return fmt.Errorf("%w: %s has fallen", domain.ErrInvalid, c.Name)
	case c.PlayerID != "":
		return fmt.Errorf("%w: a friend already plays %s", domain.ErrInvalid, c.Name)
	case domain.Live(c):
		return fmt.Errorf("%w: someone is playing %s; offer one of the agent-played companions", domain.ErrInvalid, c.Name)
	}
	return nil
}

// partyMember finds a character in the party.
func (s *session) partyMember(id string) (*domain.Creature, error) {
	if !slices.Contains(s.g.Party, id) {
		return nil, fmt.Errorf("%w: no such party member", domain.ErrNotFound)
	}
	return s.g.Creature(id)
}

// TakeSeat lets a friend play an open seat. They become the character's
// player: its turns, and the bill for its agent, follow them. The character
// still belongs to its owner, and its gains settle to the owner's roster.
// name is the friend's display name, for the table.
func (u *UseCase) TakeSeat(ctx context.Context, accountID, name, gameID, characterID string) error {
	return u.run(ctx, gameID, func(g *domain.Game) error {
		return seatTakeable(g, accountID, characterID)
	}, func(s *session) error {
		c, _ := s.g.Creature(characterID) // seatTakeable found it
		s.g.OpenSeats = slices.DeleteFunc(s.g.OpenSeats, func(id string) bool { return id == c.ID })
		c.PlayerID, c.Controller = accountID, domain.ControlHuman
		s.back(c)
		s.cue(domain.CueArrive, fmt.Sprintf("%s joins the table to play %s.", name, c.Name))
		s.g.TableNotes = append(s.g.TableNotes, fmt.Sprintf("%s (%s) is now played by another human player, %s.", c.Name, c.ID, domain.PlayerText(name)))
		return nil
	})
}

// seatTakeable checks an account may take a seat: it is open, not their own,
// and they don't already play someone at this table.
func seatTakeable(g *domain.Game, accountID, characterID string) error {
	if !g.SeatOpen(characterID) {
		return fmt.Errorf("%w: that seat is no longer open", domain.ErrNotFound)
	}
	c, err := g.Creature(characterID)
	if err != nil {
		return err
	}
	if c.OwnerID == accountID || g.LeaderID == accountID {
		return fmt.Errorf("%w: this is your own party; play your characters from the party panel", domain.ErrInvalid)
	}
	if other, err := g.SeatFor(accountID, ""); err == nil {
		return fmt.Errorf("%w: you already play %s at this table", domain.ErrInvalid, other.Name)
	}
	return nil
}

// GiveSeatBack hands a seat a friend holds back to its owner's agent, any
// time, anywhere.
func (u *UseCase) GiveSeatBack(ctx context.Context, accountID, gameID, characterID string) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		c, err := s.g.SeatFor(accountID, characterID)
		if err != nil {
			return err
		}
		if c.PlayerID != accountID {
			return fmt.Errorf("%w: %s is your own character", domain.ErrInvalid, c.Name)
		}
		c.PlayerID = ""
		s.handOver(c)
		s.cue(domain.CueLeave, fmt.Sprintf("%s's player leaves the table.", c.Name))
		s.g.TableNotes = append(s.g.TableNotes, fmt.Sprintf("%s (%s) is played by their agent again; the friend who played them has left the table.", c.Name, c.ID))
		return nil
	})
}
