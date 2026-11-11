package speech_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/speech"
	"errors"
	"strings"
	"testing"
)

var errBoom = errors.New("boom")

type voice struct {
	got  []string
	fail bool
}

func (v *voice) Speak(_ context.Context, accountID, text, name, instructions string) ([]byte, error) {
	v.got = []string{accountID, text, name, instructions}
	if v.fail {
		return nil, errBoom
	}
	return []byte("mp3"), nil
}

type games struct{ fail bool }

func (g games) Load(_ context.Context, id string) (*domain.Game, error) {
	if g.fail {
		return nil, errBoom
	}
	return &domain.Game{
		ID: id, LeaderID: "a1",
		Creatures: []*domain.Creature{
			{ID: "pc1", Kind: domain.KindPC, Race: "Dwarf", Class: "Fighter", Gender: domain.Female},
			{ID: "m1", Kind: domain.KindMonster, Name: "Goblin"},
		},
		NPCs: []*domain.NPC{
			{ID: "n1", Role: "innkeeper  of the\nGilded Flagon", Attitude: "friendly", Gender: domain.Male},
			{ID: "n2", Role: strings.Repeat("ogre ", 30), Attitude: "hostile", Monster: true},
		},
	}, nil
}

func TestPersonas(t *testing.T) {
	t.Parallel()
	tests := []struct {
		speaker, account string
		fail             bool
		want             string
		err              error
	}{
		{speaker: "pc1", want: "You are voicing a female dwarf fighter adventurer. Speak"},
		{speaker: "m1", want: "monster: rough"},
		{speaker: "n1", want: "voicing innkeeper of the Gilded Flagon, friendly toward the party. The speaker is male."},
		{speaker: "n2", want: "hostile toward the party. You are voicing a monster"},
		{speaker: "stranger", want: "Speak this line in character"},
		{speaker: "pc1", account: "outsider", err: domain.ErrNotFound},
		{speaker: "pc1", fail: true, err: errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.speaker+tt.account, func(t *testing.T) {
			t.Parallel()
			v := &voice{}
			acct := "a1"
			if tt.account != "" {
				acct = tt.account
			}
			_, err := speech.New(v, games{fail: tt.fail}).Execute(context.Background(), speech.Input{
				AccountID: acct, Text: "Well met!", Voice: "onyx", Style: domain.StyleCharacter, GameID: "g1", Speaker: tt.speaker,
			})
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if tt.err == nil && !strings.Contains(v.got[3], tt.want) {
				t.Fatalf("instructions %q, want %q", v.got[3], tt.want)
			}
			if tt.speaker == "n2" && strings.Count(v.got[3], "ogre") > 16 {
				t.Fatalf("long roles are cut: %q", v.got[3])
			}
		})
	}
}

func TestExecute(t *testing.T) {
	t.Parallel()
	ok := speech.Input{AccountID: "a1", Text: "  The door creaks open.  ", Voice: "fable", Style: domain.StyleNarrator}
	tests := []struct {
		name string
		edit func(*speech.Input)
		fail bool
		want error
	}{
		{name: "narrator", edit: func(*speech.Input) {}},
		{name: "character", edit: func(in *speech.Input) { in.Style, in.Voice = domain.StyleCharacter, "onyx" }},
		{name: "empty", edit: func(in *speech.Input) { in.Text = " " }, want: domain.ErrInvalid},
		{name: "too long", edit: func(in *speech.Input) { in.Text = strings.Repeat("a", domain.MaxSpeechChars+1) }, want: domain.ErrInvalid},
		{name: "unknown voice", edit: func(in *speech.Input) { in.Voice = "gravel" }, want: domain.ErrInvalid},
		{name: "unknown style", edit: func(in *speech.Input) { in.Style = "opera" }, want: domain.ErrInvalid},
		{name: "voice fails", edit: func(*speech.Input) {}, fail: true, want: errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := &voice{fail: tt.fail}
			in := ok
			tt.edit(&in)
			out, err := speech.New(v, games{}).Execute(context.Background(), in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if tt.want != nil {
				return
			}
			if string(out.Audio) != "mp3" || out.ContentType != "audio/mpeg" {
				t.Fatalf("out = %+v", out)
			}
			if v.got[0] != "a1" || v.got[1] != "The door creaks open." || v.got[2] != in.Voice || v.got[3] == "" {
				t.Fatalf("voice got %q", v.got)
			}
		})
	}
}
