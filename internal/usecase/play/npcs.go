package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"slices"
	"strings"
)

// maxNPCText caps what the DM writes about an NPC (it reaches every prompt).
const maxNPCText = 200

// showNPC puts an NPC on the table, or updates one already there.
func (s *session) showNPC(in dmInput) (string, error) {
	name, monster, err := s.checkNPC(in)
	if err != nil {
		return "", err
	}
	gender, err := domain.ParseGender(in.Gender)
	if err != nil {
		return "", err
	}
	n := s.g.FindNPC(name)
	arrived := n == nil || !n.Here
	if n == nil {
		n = &domain.NPC{ID: domain.NPCID(name), Name: name}
		s.g.NPCs = append(s.g.NPCs, n)
	}
	n.Type, n.Monster, n.Role, n.Attitude, n.Here = in.Type, monster, clip(in.Role, maxNPCText), in.Attitude, true
	n.Gender = gender
	if look := clip(in.Look, maxNPCText); look != "" {
		n.Look = look
	}
	if in.Follows != nil {
		n.Follows = *in.Follows
	}
	s.u.art.Request(domain.NPCPortrait(s.g, n))
	if arrived {
		s.cue(domain.CueArrive, fmt.Sprintf("%s (%s) is here.", n.Name, n.Role))
		return n.Name + " is on the table.", nil
	}
	return n.Name + " updated.", nil
}

// checkNPC validates show_npc's input, returning the clipped name and
// whether the NPC is a monster type.
func (s *session) checkNPC(in dmInput) (string, bool, error) {
	name := clip(in.Name, 60)
	if name == "" {
		return "", false, fmt.Errorf("%w: an NPC needs a name", domain.ErrInvalid)
	}
	if !slices.Contains(domain.Attitudes, in.Attitude) {
		return "", false, fmt.Errorf("%w: attitude %q (use %s)", domain.ErrInvalid, in.Attitude, strings.Join(domain.Attitudes, ", "))
	}
	monster, err := s.npcType(in.Type)
	return name, monster, err
}

// npcType checks an NPC type and reports whether it is a monster.
func (s *session) npcType(t string) (bool, error) {
	if slices.Contains(domain.NPCTypes, t) {
		return false, nil
	}
	if _, err := s.u.catalog.Monster(t, "npc"); err == nil {
		return true, nil
	}
	return false, fmt.Errorf("%w: NPC type %q (use a person type such as %s, or a monster key)", domain.ErrUnknown, t, strings.Join(domain.NPCTypes[:6], ", "))
}

// npcLeaves takes an NPC off the table.
func (s *session) npcLeaves(in dmInput) (string, error) {
	n := s.g.FindNPC(in.Name)
	if n == nil || !n.Here {
		return "", fmt.Errorf("%w: %q is not on the table", domain.ErrNotFound, in.Name)
	}
	n.Here, n.Follows = false, false
	s.cue(domain.CueLeave, n.Name+" leaves.")
	return n.Name + " has left.", nil
}

// npcStatus lists the NPCs in the scene for the DM's per-turn digest.
func npcStatus(g *domain.Game) string {
	here := g.NPCsHere()
	if len(here) == 0 {
		return ""
	}
	lines := make([]string, 0, len(here))
	for _, n := range here {
		lines = append(lines, "- "+n.Describe())
	}
	return "NPCs here:\n" + strings.Join(lines, "\n") + "\n"
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// castSeat is a throwaway seat that checks the DM's narration for NPCs.
const castSeat = "cast"

// castCheck makes sure everyone the DM just put in the scene is on the table:
// models often forget show_npc, so a small separate call reads the narration
// and adds (or removes) NPCs. It only uses the NPC tools.
func (s *session) castCheck(ctx context.Context) {
	if s.g.Mode != domain.ModeExploration {
		return
	}
	here := "nobody"
	if st := npcStatus(s.g); st != "" {
		here = "\n" + st
	}
	msg := fmt.Sprintf("Player characters (not NPCs): %s.\nOn the table now: %s\n\nThe Dungeon Master just narrated:\n%s",
		partyNames(s.g), here, s.lastNarration())
	_, err := s.runAgent(ctx, seatRun{
		seat: castSeat, role: domain.RoleCast, payer: s.g.LeaderID, actor: domain.DMSeat, quiet: true,
		tools: npcSpecs(), message: msg, final: domain.EvThinking,
		handle: func(call domain.ToolCall) (string, error) {
			if call.Name != "show_npc" && call.Name != "npc_leaves" {
				return "", fmt.Errorf("%w: only show_npc and npc_leaves are available", domain.ErrInvalid)
			}
			return s.dmTool(call)
		},
	})
	delete(s.g.Transcripts, castSeat) // each check starts fresh
	if err != nil {
		s.u.log.Warn("could not check the cast", "game", s.g.ID, "err", err)
		return
	}
	s.g.CastAt = s.g.LastNarrationAt() + 1
}

func partyNames(g *domain.Game) string {
	names := make([]string, 0, len(g.Party))
	for _, c := range g.PartyMembers() {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}

// CheckCast runs the NPC check for an adventure whose latest narration has not
// been checked (for example, a scene from before NPCs were shown). It does
// nothing when the cast is up to date.
func (u *UseCase) CheckCast(ctx context.Context, accountID, gameID string) error {
	return u.withGame(ctx, accountID, gameID, func(s *session) error {
		if s.g.CastStale() {
			s.castCheck(ctx)
		}
		return nil
	})
}
