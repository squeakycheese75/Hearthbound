package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"strings"
)

// compact closes a chapter when any agent's context has grown past its limit:
// the DM writes a recap, and every seat's history restarts from it. It only
// happens between rounds outside combat, so no turn is cut in half. Failures
// are logged and leave the histories as they were.
func (s *session) compact(ctx context.Context) {
	if s.g.Mode != domain.ModeExploration || !s.g.Started || !s.overLimit() {
		return
	}
	last := s.lastNarration()
	recap, err := s.runAgent(ctx, seatRun{
		seat: domain.DMSeat, role: domain.RoleRecap, prompt: domain.RoleDM, payer: s.g.LeaderID, actor: domain.DMSeat,
		data: s.dmPrompt(), tools: dmTools(), message: s.u.roles.Role(domain.RoleRecap).Prompt,
		final: domain.EvThinking, quiet: true, ledger: domain.RoleRecap,
		handle: func(call domain.ToolCall) (string, error) {
			return "", fmt.Errorf("%w: no tools while writing the recap", domain.ErrInvalid)
		},
	})
	if err != nil || strings.TrimSpace(recap) == "" {
		s.u.log.Warn("chapter recap failed; keeping full histories", "game", s.g.ID, "err", err)
		return
	}
	s.g.Chapter++
	s.g.Recap = strings.TrimSpace(recap)
	recent := s.recentTable(8)
	s.g.Transcripts = map[string][]domain.AgentMessage{
		domain.DMSeat: {{Role: domain.RoleUser, Text: fmt.Sprintf(
			"Your notes so far (end of chapter %d):\n%s\n\nThe last thing you told the players:\n%s\n\nThe story continues from here.", s.g.Chapter, s.g.Recap, last)}},
	}
	for _, c := range s.g.PartyMembers() {
		s.g.Transcripts[c.ID] = []domain.AgentMessage{{Role: domain.RoleUser, Text: fmt.Sprintf(
			"The story so far:\n%s\n\nWhat happened most recently:\n%s", s.g.Recap, recent)}}
	}
	s.g.Context = nil
	s.u.log.Info("chapter compacted", "game", s.g.ID, "chapter", s.g.Chapter, "recap_chars", len(s.g.Recap))
	s.cue(domain.CueChapter, fmt.Sprintf("Chapter %d ends.", s.g.Chapter))
}

// overLimit reports whether the DM or any companion carries too much history.
func (s *session) overLimit() bool {
	dm, companion := s.u.Limits()
	for seat, size := range s.g.Context {
		limit := companion
		if seat == domain.DMSeat {
			limit = dm
		}
		if size >= limit {
			return true
		}
	}
	return false
}

// recentTable is the last few things said and done at the table, for companions.
func (s *session) recentTable(n int) string {
	var lines []string
	for i := len(s.g.Log) - 1; i >= 0 && len(lines) < n; i-- {
		e := s.g.Log[i]
		switch e.Type {
		case domain.EvNarration:
			lines = append(lines, "DM: "+e.Text)
		case domain.EvSpeech, domain.EvPlayer:
			lines = append(lines, s.nameOf(e.Actor)+": "+domain.PlayerText(e.Text))
		}
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return strings.Join(lines, "\n")
}

func (s *session) nameOf(id string) string {
	if c, err := s.g.Creature(id); err == nil {
		return c.Name
	}
	return id
}
