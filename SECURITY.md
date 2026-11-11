# Security

Hearthbound servers store players' API keys (encrypted at rest) and run AI
agents on them, so security reports are taken seriously.

## Reporting a vulnerability

Please **don't open a public issue.** Report it privately through GitHub:
the repository's **Security** tab, then **Report a vulnerability**.

Include what you found, how to reproduce it, and what an attacker could do
with it. You'll get a reply within a week. Once a fix is released you'll be
credited, unless you'd rather not be.

## What's in scope

- Reading or using another player's API keys, games, characters or gold.
- Bypassing sign-in, invites, sponsorship caps or rate limits.
- Prompt injection that escapes `domain.PlayerText` to change rewards,
  hazards or another player's state beyond `rules/limits.go`.
- Story text, player text or keys appearing in logs.

Running a server with `-debug` exposes full agent transcripts by design; it
is not meant for public servers.

## Supported versions

Only the latest `main` is supported. Server owners should update with
`git pull && docker compose up -d --build` (see [docs/DEPLOY.md](docs/DEPLOY.md)).
