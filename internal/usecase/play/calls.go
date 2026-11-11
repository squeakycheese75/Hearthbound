package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
)

// PostWanted puts the party's post on the tavern's notice board ("" takes it
// down), so other players can offer their heroes. Leader only.
func (u *UseCase) PostWanted(ctx context.Context, accountID, gameID, text string) error {
	text, err := domain.ParseWanted(text)
	if err != nil {
		return err
	}
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.LeaderID != accountID {
			return fmt.Errorf("%w: only the party's leader posts to the board", domain.ErrForbidden)
		}
		s.g.Wanted = text
		return nil
	})
}

// AskForHero asks the owner of a hero who wants to be asked first whether
// they may join the party. The owner answers from their notices.
func (u *UseCase) AskForHero(ctx context.Context, accountID, gameID, characterID string) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.LeaderID != accountID {
			return fmt.Errorf("%w: only the party's leader can recruit", domain.ErrForbidden)
		}
		ch, err := s.canTakeOn(ctx, characterID)
		if err != nil {
			return err
		}
		switch {
		case ch.OwnerID == accountID || ch.Hire == domain.HireAuto:
			return fmt.Errorf("%w: %s can join without asking; recruit them", domain.ErrInvalid, ch.Sheet.Name)
		case ch.Hire != domain.HireAsk:
			return fmt.Errorf("%w: character %s", domain.ErrNotFound, ch.ID)
		}
		s.g.Calls = append(s.g.Calls, domain.CallAbout(domain.CallAsk, ch))
		s.system(fmt.Sprintf("You asked %s's owner whether they may join; they'll answer from their notices.", ch.Sheet.Name))
		return nil
	})
}

// OfferHero offers one of the account's idle heroes to a party whose leader
// has posted to the board. The leader answers from the game.
func (u *UseCase) OfferHero(ctx context.Context, accountID, gameID, characterID string) error {
	return u.run(ctx, gameID, func(g *domain.Game) error {
		if g.Wanted == "" || g.LeaderID == accountID {
			return fmt.Errorf("%w: that party isn't looking for heroes", domain.ErrNotFound)
		}
		return nil
	}, func(s *session) error {
		ch, err := s.canTakeOn(ctx, characterID)
		if err != nil {
			return err
		}
		if ch.OwnerID != accountID {
			return fmt.Errorf("%w: character %s", domain.ErrNotFound, ch.ID)
		}
		call := domain.CallAbout(domain.CallOffer, ch)
		s.g.Calls = append(s.g.Calls, call)
		s.system(fmt.Sprintf("Answering your post: %s, a %s, would like to join the party.", call.Name, call.Summary))
		return nil
	})
}

// canTakeOn checks the party could take on a hero now: at the tavern, with
// room, the hero free, and nobody already asked about them.
func (s *session) canTakeOn(ctx context.Context, characterID string) (domain.Character, error) {
	if err := canChange(s.g); err != nil {
		return domain.Character{}, err
	}
	if len(s.g.Party)+len(s.g.Calls) >= domain.MaxParty {
		return domain.Character{}, fmt.Errorf("%w: the party is full (%d, counting heroes already asked)", domain.ErrInvalid, domain.MaxParty)
	}
	if _, asked := s.g.CallFor(characterID); asked {
		return domain.Character{}, fmt.Errorf("%w: already asked; waiting for an answer", domain.ErrInvalid)
	}
	ch, err := s.u.store.Character(ctx, characterID)
	if err != nil {
		return domain.Character{}, fmt.Errorf("load character: %w", err)
	}
	if !ch.Available() {
		return domain.Character{}, fmt.Errorf("%w: %s is not free to join (%s)", domain.ErrInvalid, ch.Sheet.Name, ch.Status)
	}
	return ch, nil
}

// AnswerCall answers a call about a hero: the owner answers an ask, the
// leader an offer. Whoever made the call can withdraw it (yes = false).
// Yes brings the hero into the party.
func (u *UseCase) AnswerCall(ctx context.Context, accountID, gameID, characterID string, yes bool) error {
	return u.run(ctx, gameID, func(g *domain.Game) error {
		c, ok := g.CallFor(characterID)
		switch {
		case !ok:
			return fmt.Errorf("%w: nobody is waiting on that answer", domain.ErrNotFound)
		case c.Answerer(g.LeaderID) == accountID, c.Asker(g.LeaderID) == accountID && !yes:
			return nil
		}
		return fmt.Errorf("%w: that call isn't yours to answer", domain.ErrForbidden)
	}, func(s *session) error {
		c, _ := s.g.CallFor(characterID)
		if !yes {
			s.g.DropCall(characterID)
			s.system(fmt.Sprintf("No %s this time: the call was declined or withdrawn.", c.Name))
			return nil
		}
		if err := canChange(s.g); err != nil {
			return err
		}
		ch, err := s.u.store.Character(ctx, characterID)
		if err != nil {
			return fmt.Errorf("load character: %w", err)
		}
		return s.welcome(ch)
	})
}
