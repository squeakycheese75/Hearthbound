package agents_test

import (
	"dnd/internal/adapter/agents"
	"dnd/internal/domain"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuiltInRoles(t *testing.T) {
	t.Parallel()
	lib, err := agents.New()
	if err != nil {
		t.Fatal(err)
	}
	dm := lib.Role(domain.RoleDM)
	if dm.Model != "claude-opus-5-5" || dm.Effort != "medium" || len(dm.Tools) != 16 || !strings.HasPrefix(dm.Prompt, "You are the Dungeon Master") {
		t.Fatalf("dm role: %+v", dm)
	}
	if names := len(lib.Roles()); names != 5 || lib.Roles()[0].Name != "cast" {
		t.Fatalf("roles sorted by name: %d", names)
	}
	lib.Tune(domain.RoleCharacter, "claude-haiku-5-5", "")
	lib.Tune("ghost", "x", "y")
	if c := lib.Role(domain.RoleCharacter); c.Model != "claude-haiku-5-5" || c.Effort != "low" {
		t.Fatalf("tune overrides only what is set: %+v", c)
	}
	lib.Tune(domain.RoleDM, "", "high")
	if lib.Role(domain.RoleDM).Effort != "high" || lib.Role("ghost").Name != "" {
		t.Fatal("effort override")
	}
}

func role(name, extra string) string {
	return "---\nname: " + name + "\nmodel: m\n" + extra + "---\nHello {{.Name}}\n"
}

func TestLoadRejectsBadFiles(t *testing.T) {
	t.Parallel()
	all := func(files map[string]string) fstest.MapFS {
		fsys := fstest.MapFS{}
		for _, r := range agents.Required {
			fsys[r+".md"] = &fstest.MapFile{Data: []byte(role(r, "ignored line\n"))}
		}
		for k, v := range files {
			fsys[k] = &fstest.MapFile{Data: []byte(v)}
		}
		return fsys
	}
	if _, err := agents.Load(all(nil)); err != nil {
		t.Fatalf("minimal valid set: %v", err)
	}
	cases := map[string]fstest.MapFS{
		"no frontmatter":   all(map[string]string{"x.md": "just text"}),
		"unterminated":     all(map[string]string{"x.md": "---\nname: x\n"}),
		"no model":         all(map[string]string{"x.md": "---\nname: x\n---\nhi"}),
		"bad template":     all(map[string]string{"x.md": role("x", "") + "{{.Oops"}),
		"missing required": {"dm.md": &fstest.MapFile{Data: []byte(role("dm", ""))}},
	}
	for name, fsys := range cases {
		if _, err := agents.Load(fsys); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := agents.Load(fstest.MapFS{"dm.md": &fstest.MapFile{Mode: 0o200 | 1<<31}}); err == nil {
		t.Error("an unreadable entry fails")
	}
}
