package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// QuestStatus says where a quest stands.
type QuestStatus string

// Quest statuses.
const (
	QuestOpen   QuestStatus = "open"
	QuestDone   QuestStatus = "done"
	QuestFailed QuestStatus = "failed"
)

// Quest is one of the party's goals, kept in their journal by the DM.
type Quest struct {
	Title  string      `json:"title"`
	Status QuestStatus `json:"status"`
	Note   string      `json:"note,omitempty"`
}

// Journal limits.
const (
	MaxQuests     = 10
	MaxQuestTitle = 80
	MaxQuestNote  = 240
)

// UpdateQuest adds a quest to the journal or updates the one with the same
// title (any case). An empty note keeps the old one. A full journal makes
// room by forgetting the oldest finished quest. It reports whether the
// quest is new.
func (g *Game) UpdateQuest(title string, status QuestStatus, note string) (bool, error) {
	title, note = strings.TrimSpace(title), strings.TrimSpace(note)
	if err := validQuest(title, status, note); err != nil {
		return false, err
	}
	if i := slices.IndexFunc(g.Quests, func(q Quest) bool { return strings.EqualFold(q.Title, title) }); i >= 0 {
		q := &g.Quests[i]
		q.Status = status
		if note != "" {
			q.Note = note
		}
		return false, nil
	}
	if len(g.Quests) >= MaxQuests {
		i := slices.IndexFunc(g.Quests, func(q Quest) bool { return q.Status != QuestOpen })
		if i < 0 {
			return false, fmt.Errorf("%w: the journal is full of open quests; finish or fail one first", ErrInvalid)
		}
		g.Quests = slices.Delete(g.Quests, i, i+1)
	}
	g.Quests = append(g.Quests, Quest{Title: title, Status: status, Note: note})
	return true, nil
}

func validQuest(title string, status QuestStatus, note string) error {
	switch {
	case title == "" || utf8.RuneCountInString(title) > MaxQuestTitle:
		return fmt.Errorf("%w: a quest title is 1-%d characters", ErrInvalid, MaxQuestTitle)
	case utf8.RuneCountInString(note) > MaxQuestNote:
		return fmt.Errorf("%w: keep the note under %d characters", ErrInvalid, MaxQuestNote)
	case status != QuestOpen && status != QuestDone && status != QuestFailed:
		return fmt.Errorf("%w: quest status %q", ErrInvalid, status)
	}
	return nil
}
