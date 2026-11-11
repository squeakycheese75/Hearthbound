# Deploying Hearthbound

Run your own realm on any Linux server: a single container for the game, and
Caddy in front of it for automatic HTTPS. Players' AI runs on their own API
keys, so the server only needs to be small (1 vCPU and 1–2 GB of RAM is plenty
to start).

## What you need

- A Linux server with Docker and the Docker Compose plugin (for example a small
  VPS at Hetzner or DigitalOcean, Ubuntu 24.04).
- A domain name with an **A record** (and **AAAA** for IPv6) pointing at the
  server. Caddy needs it to get a certificate.
- Ports **80** and **443** open to the internet, and **22** for SSH. With
  `ufw`: `ufw allow OpenSSH && ufw allow 80,443/tcp && ufw allow 443/udp && ufw enable`.

## First start

```sh
git clone git@github.com:squeakycheese75/Hearthbound.git && cd Hearthbound
cp deploy/.env.example .env
nano .env                  # DOMAIN, ADMIN_USER, ADMIN_PASSWORD
docker compose up -d --build
docker compose logs -f app # watch it start; Ctrl-C to stop watching
```

Open `https://your-domain`, sign in as the admin, add your Anthropic key in
**Settings**, and create invites there for your players. Everyone who registers
needs an invite code. Friends without an API key can play on yours: make the
invite with **I'll pay for their AI**, and set how much each may spend a day
under **Players in the realm**. Once the admin exists you can delete `ADMIN_PASSWORD`
from `.env`: it is only read on first start, and the server warns in its log if
there is no account and no password to create one.

To try the whole stack on your own machine first, set `DOMAIN=localhost`. Caddy
then serves `https://localhost` with its own local certificate (the browser asks
you to accept it).

## What runs

| Service | Does |
|---|---|
| `app` | The game: the Go server with the UI built in, on port 8080 inside the network only. It runs as a non-root user in a minimal image, and Docker checks `/healthz`. |
| `caddy` | Gets and renews the HTTPS certificate, redirects HTTP to HTTPS, compresses replies, and passes live events through unbuffered. |

The app runs with `TABLETOP_TRUST_PROXY=1`, so sign-in limits apply to the real
player's address that Caddy forwards. Every response carries security headers:
a strict content policy, no framing, and HSTS over HTTPS.

## Your data

Everything lives in the `data` volume, mounted at `/data`:

| Path | What |
|---|---|
| `/data/tabletop.db` | the realm: accounts, heroes, adventures, the market and the gold ledger |
| `/data/secret.key` | the master key that encrypts players' API keys. **Without it, saved keys can't be read.** |
| `/data/media/` | custom paintings (only with `-art paint`) |
| `/data/backups/` | the automatic backups |

### Backups

The server backs the database up **a minute after it starts and then every 24
hours**. It writes a consistent copy while the game keeps running and keeps the
newest 14 (`BACKUP_KEEP` in `.env`). Each backup is a complete SQLite database,
named like `tabletop-20261102-031500.db` (times in UTC).

Take one by hand at any time:

```sh
docker compose exec app /tabletop -backup-now
```

**These backups sit on the same server.** Copy them off it too. Either turn on
your provider's server snapshots, or copy the folder out nightly, for example
from another machine:

```sh
docker compose cp app:/data/backups ./hearthbound-backups   # run on the server
scp -r server:Hearthbound/hearthbound-backups ~/backups/     # or pull it with rsync/rclone
```

Back up `/data/secret.key` once and keep it somewhere safe and separate. It
doesn't change.

### Restoring

```sh
docker compose stop app
docker compose cp ./tabletop-20261102-031500.db app:/data/tabletop.db
docker compose start app
```

Stopping the app closes the database cleanly first. The restored file must sit
beside the same `secret.key`, or players will need to re-enter their API keys.

## Updating

```sh
git pull
docker compose up -d --build
```

The database upgrades itself on start. Take a backup first if you like
(`docker compose exec app /tabletop -backup-now`).

## Settings

The app reads `TABLETOP_*` environment variables (see the Quickstart for all of
them). The useful ones for a server go under `app: environment:` in
`docker-compose.yml`:

| Variable | Default | |
|---|---|---|
| `TABLETOP_BACKUP_EVERY` | `24h` | how often to back up |
| `TABLETOP_BACKUP_KEEP` | `14` | how many backups to keep |
| `TABLETOP_ART` | `stock` | `stock`, `paint` (custom paintings on players' OpenAI keys) or `off` |
| `TABLETOP_LOG_FORMAT` | `json` | `text` or `json` |
| `TABLETOP_DEBUG` | off | debug logs and the transcript endpoint; not for a public server |
| `TABLETOP_SECRET_KEY` | from `secret.key` | the master key as base64, if you'd rather keep it out of the volume |

## Logs and health

```sh
docker compose ps            # both services up, app "healthy"
docker compose logs -f app   # metadata only: never story text or keys
docker compose logs -f caddy # certificates and requests
```
