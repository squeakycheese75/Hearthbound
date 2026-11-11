# Contributing to Hearthbound

Thanks for wanting to help at the table. Bug reports, rules fixes, new SRD
monsters and spells, UI polish and docs are all welcome.

## Getting started

You don't need an API key to work on most of the game:

```sh
make web && make demo   # http://127.0.0.1:8080, a scripted DM on data/demo.db
```

[Quickstart.md](Quickstart.md) covers building, running and configuring, and
[docs/ROADMAP.md](docs/ROADMAP.md) lists what's planned and the design rules
that must keep holding. Pick something from the roadmap, or open an issue
first if you'd like to discuss a bigger change.

## How the code is laid out

Clean Architecture, enforced by the linter. [CLAUDE.md](CLAUDE.md) is the
detailed guide (it is also read by AI coding sessions); the short version:

- `internal/domain`: entities and the 5e rules engine (`domain/rules`), stdlib only.
- `internal/usecase/<name>`: application logic. Imports `domain` only.
- `internal/adapter/<name>`: SQLite, Claude, OpenAI, dice and the rest.
- `internal/delivery/web`: the HTTP server. `web/` is the React UI.
- `cmd/tabletop`: wiring only.

Every side effect (clock, IDs, dice, storage, agents) is injected, so tests
use hand-written fakes and never sleep.

## Before you open a pull request

The definition of done:

```sh
make check              # golangci-lint (0 issues), go test -race, 100% use case coverage
cd web && npm run build # the UI builds and type-checks
```

CI runs both on every pull request. A few house rules:

- No `else` (a lint rule): use guard clauses and early returns.
- Tests are black-box (`package x_test`), table-driven and `t.Parallel()`.
- Logs carry metadata only: never story text, player text or keys.
- Anything a player writes reaches agents as `domain.PlayerText`.
- Rules content must come from the SRD 5.1 (CC BY 4.0). Don't add material
  from other 5e books.
- Never run `-demo` against a real save (`data/tabletop.db`).

Keep pull requests focused on one change, and describe how you checked it
(tests, and a screenshot for UI changes).

## Licence

By contributing, you agree that your contributions are licensed under the
project's licences: code under the [GNU AGPL-3.0](LICENSE), and art under
[CC BY 4.0](web/public/stock/LICENSE.md).
