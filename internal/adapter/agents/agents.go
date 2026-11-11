// Package agents loads the agent definition files: one Markdown file per role
// (dm, character, choices, recap) with frontmatter for the model, effort and
// tools, and the prompt template as the body.
package agents

import (
	"bufio"
	"dnd/internal/domain"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"text/template"
)

//go:embed roles/*.md
var builtIn embed.FS

// Library holds every role's definition.
type Library struct {
	roles map[string]domain.AgentRole
}

// Required are the roles the table cannot run without.
var Required = []string{domain.RoleDM, domain.RoleCharacter, domain.RoleChoices, domain.RoleCast, domain.RoleRecap}

// New loads the built-in definitions shipped in the binary.
func New() (*Library, error) {
	sub, err := fs.Sub(builtIn, "roles")
	if err != nil {
		return nil, fmt.Errorf("built-in roles: %w", err)
	}
	return Load(sub)
}

// Load reads every *.md file in fsys. Each must have frontmatter with at least
// a name and a model, and a body that parses as a text/template.
func Load(fsys fs.FS) (*Library, error) {
	names, err := fs.Glob(fsys, "*.md")
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	lib := &Library{roles: map[string]domain.AgentRole{}}
	for _, name := range names {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		r, err := parse(string(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path.Base(name), err)
		}
		lib.roles[r.Name] = r
	}
	for _, need := range Required {
		if _, ok := lib.roles[need]; !ok {
			return nil, fmt.Errorf("%w: no definition for the %q role", domain.ErrInvalid, need)
		}
	}
	return lib, nil
}

// Role returns a role's definition (the zero value if there is none).
func (l *Library) Role(name string) domain.AgentRole {
	return l.roles[name]
}

// Roles lists every definition, sorted by name.
func (l *Library) Roles() []domain.AgentRole {
	out := make([]domain.AgentRole, 0, len(l.roles))
	for _, r := range l.roles {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b domain.AgentRole) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// Tune overrides a role's model and/or effort (from command-line flags).
// Empty values keep the file's setting.
func (l *Library) Tune(role, model, effort string) {
	r, ok := l.roles[role]
	if !ok {
		return
	}
	if model != "" {
		r.Model = model
	}
	if effort != "" {
		r.Effort = effort
	}
	l.roles[role] = r
}

func parse(doc string) (domain.AgentRole, error) {
	front, body, ok := strings.Cut(strings.TrimPrefix(doc, "---\n"), "\n---\n")
	if !ok || !strings.HasPrefix(doc, "---\n") {
		return domain.AgentRole{}, fmt.Errorf("%w: missing --- frontmatter", domain.ErrInvalid)
	}
	r := domain.AgentRole{Prompt: strings.TrimSpace(body)}
	sc := bufio.NewScanner(strings.NewReader(front))
	for sc.Scan() {
		key, val, found := strings.Cut(sc.Text(), ":")
		if !found {
			continue
		}
		setField(&r, strings.TrimSpace(key), strings.TrimSpace(val))
	}
	if r.Name == "" || r.Model == "" {
		return domain.AgentRole{}, fmt.Errorf("%w: frontmatter needs name and model", domain.ErrInvalid)
	}
	if _, err := template.New(r.Name).Parse(r.Prompt); err != nil {
		return domain.AgentRole{}, fmt.Errorf("%w: prompt template: %w", domain.ErrInvalid, err)
	}
	return r, nil
}

func setField(r *domain.AgentRole, key, val string) {
	switch key {
	case "name":
		r.Name = val
	case "description":
		r.Description = val
	case "model":
		r.Model = val
	case "effort":
		r.Effort = val
	case "tools":
		for t := range strings.SplitSeq(val, ",") {
			if t = strings.TrimSpace(t); t != "" {
				r.Tools = append(r.Tools, t)
			}
		}
	}
}
