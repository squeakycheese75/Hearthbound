# Working on Hearthbound

Read [README.md](README.md), [Quickstart.md](Quickstart.md) and
[docs/ROADMAP.md](docs/ROADMAP.md) first. The roadmap holds the design rules
and the spec for the next step.

## Definition of done

`make check` passes: golangci-lint (0 issues), `go test -race ./...`, and
**100% statement coverage of `internal/usecase/...`**. The UI must build
(`cd web && npm run build`; this also type-checks).

## Conventions (from the global Go standards)

- **Clean Architecture,** enforced by depguard:
  - `domain` uses stdlib only. `domain/rules` may import `domain`.
  - `usecase` imports `domain` only; never another use case.
  - `adapter` must not import `usecase` or `delivery`.
  - `delivery` must not import `adapter`. Wiring lives in `cmd/tabletop`.
- **Interfaces:** small, defined where consumed. Every side effect is injected:
  clock, IDs, randomness (`rules.Roller`), logging (`*slog.Logger`), storage, agents.
- **Style:**
  - No `else` (a ruleguard rule).
  - Cyclomatic complexity of 10 or less.
  - gofumpt formatting. It groups `dnd/...` imports with the stdlib; that is expected.
  - Wrap errors with `%w`. The sentinel errors live in `domain/errors.go` and `domain/account.go`.
- **Tests:**
  - Black-box (`package x_test`), table-driven, `t.Parallel()`, hand-written fakes.
  - Time-based behaviour uses fake clocks (`play` has a movable `clock`).
  - Background workers use `testing/synctest`. No sleeps.
- **Logs:** metadata only, never story text or secrets.
- **Player text in prompts:** anything a player wrote (actions, speech,
  backstories, later notice-board posts) goes to agents through
  `domain.PlayerText`. Rewards and hazards stay inside the limits in
  `internal/domain/rules/limits.go`.

## Gotchas

- **Never run `-demo` against `data/tabletop.db`.** The scripted agent writes
  its turns into real adventures and corrupts their transcripts (its assistant
  turns have empty content, which the real API rejects). `-demo` defaults to
  `data/demo.db`. For anything that touches real data, test on a copy:
  `cp data/tabletop.db /tmp/x.db && ./bin/tabletop -demo -db /tmp/x.db -addr 127.0.0.1:8099`.
- **Transcripts are append-only.** Don't rewrite agent histories. The Claude
  adapter replays assistant turns from `AgentMessage.Raw` and merges
  consecutive user messages.
- **Forced tool choice is unavailable** on current Claude models. Tools the
  model must call (for example `offer_choices`) need a per-turn reminder in the
  user message and a fallback (see `suggestChoices` in `usecase/play/combat.go`).
- **The UI is embedded.** After frontend changes, run `make web && make build`
  before running the binary.
- **SQLite JSON:** game data is stored as BLOB. Don't use `json_extract` on it
  (newer SQLite reads BLOBs as JSONB); decode in Go instead.
- **Shell quoting:** the Bash tool runs zsh. Put multi-step curl and JSON tests in a
  `bash` script file. Don't name a shell function `code` (it collides with the VS Code CLI).

## Verifying changes in a real browser

The server keeps an SSE stream open, so `chrome --screenshot` never finishes.
Use the DevTools-protocol helper instead (Chrome is installed; no npm deps):

```sh
node tools/shoot.mjs <url> <out.png> <width> <height> [waitMs] ["js step||js step"]
```

It runs each JS step (for example clicking a button by its text) before capturing.

**Multi-account smoke test:** start a server with
`TABLETOP_ADMIN_USER=james TABLETOP_ADMIN_PASSWORD=... ./bin/tabletop -demo -local=false -db /tmp/x.db`.
Drive it with two curl cookie jars (`-c/-b a.jar`, `-c/-b b.jar`):
log in, create an invite, register, then check that each account only sees its own games.

## Model choices

Each agent role has a definition file in `internal/adapter/agents/roles/`
(`dm`, `character`, `choices`, `cast`, `recap`): frontmatter for model, effort and
tools, and a `text/template` prompt as the body. The DM runs at `medium`
effort; characters and the choice helper at `low`. All default to
`claude-opus-5-5`. Players can pick their own characters' model in Settings
(`domain.AgentModels`), and the bridge in `cmd/tabletop/agent.go` applies it.
Refusal fallbacks (`fallbacks: "default"`) are on for every model except
Haiku, which has none.

- **Prompts must render the same way every time.** System prompts are cached
  per seat, so templates must produce identical text for the same game: no
  timestamps, no map iteration.
- **Tools stay in Go.** A role's `tools:` line only picks from what the code
  offers. A role that lists none gets them all.
