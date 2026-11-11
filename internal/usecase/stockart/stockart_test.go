package stockart_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/stockart"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

type catalog struct{}

func (catalog) Races() []domain.RaceTemplate {
	return []domain.RaceTemplate{{Name: "Elf"}, {Name: "Half-Orc"}}
}

func (catalog) Classes() []domain.ClassTemplate {
	return []domain.ClassTemplate{{Name: "Wizard"}}
}
func (catalog) MonsterKeys() []string { return []string{"goblin", "giant_rat"} }

type shelf struct {
	mu      sync.Mutex
	files   map[string]bool
	saveErr error
}

func (s *shelf) Exists(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.files[key]
}

func (s *shelf) Save(key string, _ []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	s.files[key] = true
	return nil
}

type painter struct {
	mu      sync.Mutex
	prompts []string
	fail    string
	err     error
	cancel  context.CancelFunc
}

func (p *painter) Generate(_ context.Context, payer, prompt, _ string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
	if payer != "" {
		return nil, errors.New("stock art bills nobody")
	}
	p.prompts = append(p.prompts, prompt)
	if p.fail != "" && strings.Contains(prompt, p.fail) {
		return nil, p.err
	}
	return []byte("img"), nil
}

var (
	quiet = slog.New(slog.DiscardHandler)
	pack  = domain.ArtPack{Key: "ink", Name: "Ink", Style: "Style: ink wash."}
)

func TestListCoversScenesMonstersAndPortraits(t *testing.T) {
	t.Parallel()
	st := &shelf{files: map[string]bool{"scenes/tavern": true}}
	u := stockart.New(pack, catalog{}, &painter{}, st, quiet)
	all := u.List()
	if want := 2*len(domain.SceneKeys) + 3*len(domain.NPCTypes) + 2 + 2*3; len(all) != want {
		t.Fatalf("%d items, want %d", len(all), want)
	}
	if all[0].Key != "scenes/tavern" || !all[0].Done || all[1].Done {
		t.Fatalf("done flags: %+v %+v", all[0].Key, all[1].Key)
	}
	portraits := u.List(domain.ArtPortrait)
	// Each race and class has an any-gender portrait, then male and female ones.
	if len(portraits) != 6 || portraits[3].Key != "portraits/half-orc-wizard" || !strings.Contains(portraits[3].Prompt, "a half-orc wizard") ||
		portraits[5].Key != "portraits/half-orc-wizard-f" || !strings.Contains(portraits[5].Prompt, "a female half-orc wizard") || !strings.HasSuffix(portraits[1].Prompt, "Style: ink wash.") {
		t.Fatalf("portraits: %+v", portraits)
	}
	maps := u.List(domain.ArtBattlemap)
	if len(maps) != len(domain.SceneKeys) || maps[0].Key != "battlemaps/tavern" || maps[0].Size != domain.SceneSize ||
		!strings.Contains(maps[0].Prompt, "straight down") || !strings.Contains(maps[0].Prompt, "No grid lines") {
		t.Fatalf("battle maps: %+v", maps[0])
	}
	if m := u.List(domain.ArtMonster); m[1].Key != "monsters/giant-rat" || !strings.Contains(m[1].Prompt, "giant rat") {
		t.Fatalf("monsters: %+v", m)
	}
}

func TestExecutePaintsOnlyWhatIsMissing(t *testing.T) {
	t.Parallel()
	st := &shelf{files: map[string]bool{"monsters/goblin": true}}
	p := &painter{fail: "half-orc", err: errors.New("rate limited")}
	u := stockart.New(pack, catalog{}, p, st, quiet)
	out, err := u.Execute(context.Background(), stockart.Input{Kinds: []domain.ArtKind{domain.ArtMonster, domain.ArtPortrait}, Workers: 3})
	if err != nil {
		t.Fatal(err)
	}
	if out.Painted != 4 || out.Failed != 3 || out.Skipped != 1 || len(p.prompts) != 7 {
		t.Fatalf("run %+v, %d calls", out, len(p.prompts))
	}
	if !st.Exists("portraits/elf-wizard") || !st.Exists("portraits/elf-wizard-m") || st.Exists("portraits/half-orc-wizard") {
		t.Fatal("successes saved, failures not")
	}
	again, _ := u.Execute(context.Background(), stockart.Input{Kinds: []domain.ArtKind{domain.ArtMonster}})
	if again.Painted != 0 || again.Skipped != 2 {
		t.Fatalf("a second run paints nothing new: %+v", again)
	}
}

func TestExecuteLimitSaveErrorsAndCancel(t *testing.T) {
	t.Parallel()
	limited := stockart.New(pack, catalog{}, &painter{}, &shelf{files: map[string]bool{}}, quiet)
	out, err := limited.Execute(context.Background(), stockart.Input{Kinds: []domain.ArtKind{domain.ArtScene}, Limit: 2})
	if err != nil || out.Painted != 2 || out.Skipped != len(domain.SceneKeys)-2 {
		t.Fatalf("limit: %+v %v", out, err)
	}
	full := stockart.New(pack, catalog{}, &painter{}, &shelf{files: map[string]bool{}, saveErr: errors.New("disk full")}, quiet)
	if out, _ := full.Execute(context.Background(), stockart.Input{Kinds: []domain.ArtKind{domain.ArtMonster}}); out.Failed != 2 {
		t.Fatalf("save errors count as failures: %+v", out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &painter{cancel: cancel, fail: "goblin", err: context.Canceled}
	stopped := stockart.New(pack, catalog{}, p, &shelf{files: map[string]bool{}}, quiet)
	if _, err := stopped.Execute(ctx, stockart.Input{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel stops the run: %v", err)
	}
	if len(p.prompts) > 2 {
		t.Fatalf("no new work after cancel: %d calls", len(p.prompts))
	}
}
