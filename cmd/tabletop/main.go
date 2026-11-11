// Command tabletop runs the AI tabletop web server.
package main

import (
	"context"
	"crypto/rand"
	"dnd/internal/adapter/agents"
	"dnd/internal/adapter/demo"
	"dnd/internal/adapter/dice"
	"dnd/internal/adapter/hub"
	"dnd/internal/adapter/media"
	"dnd/internal/adapter/openai"
	"dnd/internal/adapter/secure"
	"dnd/internal/adapter/sqlite"
	"dnd/internal/adapter/srd"
	"dnd/internal/adapter/system"
	"dnd/internal/delivery/web"
	"dnd/internal/domain"
	"dnd/internal/usecase/accounts"
	"dnd/internal/usecase/backups"
	"dnd/internal/usecase/characters"
	"dnd/internal/usecase/deadlines"
	"dnd/internal/usecase/economy"
	"dnd/internal/usecase/games"
	"dnd/internal/usecase/market"
	"dnd/internal/usecase/newgame"
	"dnd/internal/usecase/play"
	"dnd/internal/usecase/speech"
	"dnd/internal/usecase/tavern"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	webui "dnd/web"
)

type config struct {
	addr            string
	db              string
	agents          string
	model           string
	dmEffort        string
	companionEffort string
	debug           bool
	demo            bool
	logFile         string
	logFormat       string
	art             string
	artPack         string
	stockList       bool
	paintStock      string
	stockLimit      int
	stockDir        string
	stockAccount    string
	imageModel      string
	imageQuality    string
	artWorkers      int
	compactAt       int
	local           bool
	trustProxy      bool
	backupDir       string
	backupEvery     time.Duration
	backupKeep      int
	backupNow       bool
	healthcheck     bool
}

func loadConfig() config {
	c := config{}
	flag.StringVar(&c.addr, "addr", env("TABLETOP_ADDR", "127.0.0.1:8080"), "listen address")
	flag.StringVar(&c.db, "db", env("TABLETOP_DB", "data/tabletop.db"), "SQLite database path")
	flag.StringVar(&c.agents, "agents", env("TABLETOP_AGENTS", ""), "directory of agent definition files (default: the built-in internal/adapter/agents/roles)")
	flag.StringVar(&c.model, "model", env("TABLETOP_MODEL", ""), "override the model of every agent role")
	flag.StringVar(&c.dmEffort, "dm-effort", env("TABLETOP_DM_EFFORT", ""), "override the DM's effort: low|medium|high|xhigh|max")
	flag.StringVar(&c.companionEffort, "companion-effort", env("TABLETOP_COMPANION_EFFORT", ""), "override the effort of characters and the choice helper")
	flag.BoolVar(&c.debug, "debug", os.Getenv("TABLETOP_DEBUG") != "", "debug logging plus GET /api/games/{id}/debug (full agent transcripts)")
	flag.StringVar(&c.logFile, "log-file", env("TABLETOP_LOG_FILE", ""), "also append logs to this file")
	flag.StringVar(&c.logFormat, "log-format", env("TABLETOP_LOG_FORMAT", "text"), "log format: text|json")
	flag.BoolVar(&c.demo, "demo", false, "offline demo: scripted DM and companions, no API key needed")
	flag.StringVar(&c.art, "art", env("TABLETOP_ART", artStock),
		"illustrations: stock (shipped images, free) | paint (custom, each player's OpenAI key) | off")
	flag.StringVar(&c.artPack, "art-pack", env("TABLETOP_ART_PACK", domain.ArtPacks[0].Key), "stock art pack (style) to serve and paint")
	flag.BoolVar(&c.stockList, "stock-list", false, "list the stock images and which are painted, then exit")
	flag.StringVar(&c.paintStock, "paint-stock", "", "paint missing stock images (all, or scenes,monsters,portraits) with OPENAI_API_KEY, then exit")
	flag.IntVar(&c.stockLimit, "stock-limit", 0, "paint at most this many stock images (0 = all missing)")
	flag.StringVar(&c.stockAccount, "stock-account", "local", "without OPENAI_API_KEY, paint stock art with this account's saved OpenAI key")
	flag.StringVar(&c.stockDir, "stock-dir", "web/public/stock", "where art packs are written, one folder per pack (served at /stock/ after make web)")
	flag.BoolVar(&c.local, "local", os.Getenv("TABLETOP_LOCAL") != "",
		"single-user mode: sign in automatically and use ANTHROPIC_API_KEY / OPENAI_API_KEY from the environment")
	flag.StringVar(&c.imageModel, "image-model", env("TABLETOP_IMAGE_MODEL", "gpt-image-2"), "OpenAI image model")
	flag.StringVar(&c.imageQuality, "image-quality", env("TABLETOP_IMAGE_QUALITY", "medium"), "image quality: low|medium|high")
	flag.IntVar(&c.artWorkers, "art-workers", 2, "images painted in parallel")
	flag.IntVar(&c.compactAt, "compact-at", 60_000, "DM context size in tokens that ends a chapter and compacts agent histories (companions at 5/12 of it)")
	flag.BoolVar(&c.trustProxy, "trust-proxy", os.Getenv("TABLETOP_TRUST_PROXY") != "",
		"behind a reverse proxy (Caddy): take the caller's address from X-Forwarded-For")
	flag.StringVar(&c.backupDir, "backup-dir", env("TABLETOP_BACKUP_DIR", ""), "take database backups into this folder (off when empty)")
	flag.DurationVar(&c.backupEvery, "backup-every", envDuration("TABLETOP_BACKUP_EVERY", 24*time.Hour), "how often to back up")
	flag.IntVar(&c.backupKeep, "backup-keep", envInt("TABLETOP_BACKUP_KEEP", 14), "how many backups to keep")
	flag.BoolVar(&c.backupNow, "backup-now", false, "take one backup into -backup-dir, then exit")
	flag.BoolVar(&c.healthcheck, "healthcheck", false, "check that a server on -addr answers /healthz, then exit (for container health checks)")
	flag.Parse()
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if c.demo && !set["local"] {
		c.local = true // the demo is single-user unless -local=false is given
	}
	if c.demo && !set["db"] && os.Getenv("TABLETOP_DB") == "" {
		// Never let the scripted demo play (and corrupt) real adventures.
		c.db = filepath.Join(filepath.Dir(c.db), "demo.db")
	}
	return c
}

func envDuration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(key)); err == nil && d > 0 {
		return d
	}
	return def
}

func envInt(key string, def int) int {
	var n int
	if _, err := fmt.Sscan(os.Getenv(key), &n); err == nil && n > 0 {
		return n
	}
	return def
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tabletop:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	log, closeLog, err := newLogger(cfg)
	if err != nil {
		return err
	}
	defer closeLog()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.stockList || cfg.paintStock != "" {
		return stockTool(ctx, cfg, log)
	}
	if cfg.backupNow {
		return backupOnce(ctx, cfg, log)
	}
	if cfg.healthcheck {
		return checkHealth(ctx, cfg.addr)
	}
	app, closeFn, err := wire(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer closeFn()

	srv := &http.Server{Addr: cfg.addr, Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("tabletop ready", "url", "http://"+cfg.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// newLogger writes to stderr and, optionally, a log file, as text or JSON.
func newLogger(cfg config) (*slog.Logger, func(), error) {
	level := slog.LevelInfo
	if cfg.debug {
		level = slog.LevelDebug
	}
	var out io.Writer = os.Stderr
	closeFn := func() {}
	if cfg.logFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.logFile), 0o750); err != nil {
			return nil, nil, fmt.Errorf("create log dir: %w", err)
		}
		f, err := os.OpenFile(cfg.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		out = io.MultiWriter(os.Stderr, f)
		closeFn = func() { _ = f.Close() }
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.logFormat == "json" {
		return slog.New(slog.NewJSONHandler(out, opts)), closeFn, nil
	}
	return slog.New(slog.NewTextHandler(out, opts)), closeFn, nil
}

func wire(ctx context.Context, cfg config, log *slog.Logger) (*web.Server, func(), error) {
	dataDir := filepath.Dir(cfg.db)
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("create data dir: %w", err)
	}
	store, err := sqlite.Open(ctx, cfg.db)
	if err != nil {
		return nil, nil, err
	}
	vault, err := openVault(dataDir, log)
	if err != nil {
		return nil, nil, err
	}
	people := accounts.New(store, secure.Passwords{}, vault, secure.Tokens{}, system.Clock{})
	local, err := setUpAccounts(ctx, cfg, people, store, log)
	if err != nil {
		return nil, nil, err
	}
	keys := keyring{accounts: people, local: cfg.local}

	catalog := srd.New()
	store.UseItemCatalogue(catalog.ItemByName)
	events := hub.New()
	pictures, err := media.New(filepath.Join(dataDir, "media"))
	if err != nil {
		return nil, nil, err
	}
	art, err := newArt(ctx, cfg, keys, pictures, webui.Dist(), events, store, log)
	if err != nil {
		return nil, nil, err
	}
	agent, roles, err := newAgents(cfg, keys, log)
	if err != nil {
		return nil, nil, err
	}
	table := play.New(store, agent, roles, events, dice.New(), catalog, art, system.Clock{}, log, play.Config{
		CompactAt: cfg.compactAt,
	})
	importLegacyHeroes(ctx, store, pictures, log)
	go sweepDeadlines(ctx, deadlines.New(store, table, system.Clock{}, log), log)
	if err := startBackups(ctx, cfg, store, log); err != nil {
		return nil, nil, err
	}
	creator := newgame.New(catalog, store, art, tableAI{keys: keys, demo: cfg.demo}, system.Clock{}, system.IDs{})
	roster := characters.New(catalog, catalog, store, art, system.Clock{}, system.IDs{}, dice.New())
	server := web.New(web.Deps{
		Creator: creator, Roster: roster, Library: games.New(store), Table: table, Events: events, Gallery: art,
		Market:  market.New(marketStore{store}, catalog, system.Clock{}, system.IDs{}),
		Economy: economy.New(store, people, system.Clock{}),
		Board:   tavern.New(store, table, people, tableAI{keys: keys, demo: cfg.demo}),
		Speech:  speech.New(voiceBridge{keys: keys, tts: openai.NewSpeech("", "")}, store),
		Auth:    people, Settings: people, Local: local, Demo: cfg.demo, Roles: roles.Roles(),
		Static: webui.Dist(), Media: os.DirFS(pictures.Root()), Clock: system.Clock{}, Log: log,
	})
	server.Debug = cfg.debug
	server.TrustProxy = cfg.trustProxy
	if cfg.debug {
		log.Debug("debug mode: GET /api/games/{id}/debug serves full agent transcripts")
	}
	return server, func() { _ = store.Close() }, nil
}

// newAgents loads the role definitions and builds the bridge every agent call
// goes through (scripted in -demo).
func newAgents(cfg config, keys keyring, log *slog.Logger) (*agentBridge, *agents.Library, error) {
	roles, err := loadRoles(cfg, log)
	if err != nil {
		return nil, nil, err
	}
	agent := &agentBridge{keys: keys, model: roles.Role(domain.RoleDM).Model, log: log}
	if cfg.demo {
		log.Warn("demo mode: the DM and companions are scripted, not AI", "db", cfg.db)
		agent.demo = demo.New(40 * time.Millisecond)
	}
	return agent, roles, nil
}

// loadRoles reads the agent definitions (built in, or from -agents) and applies
// any -model / -dm-effort / -companion-effort overrides.
func loadRoles(cfg config, log *slog.Logger) (*agents.Library, error) {
	lib, err := agents.New()
	if cfg.agents != "" {
		lib, err = agents.Load(os.DirFS(cfg.agents))
	}
	if err != nil {
		return nil, fmt.Errorf("agent definitions: %w", err)
	}
	for _, r := range agents.Required {
		lib.Tune(r, cfg.model, "")
	}
	lib.Tune(domain.RoleDM, "", cfg.dmEffort)
	lib.Tune(domain.RoleCharacter, "", cfg.companionEffort)
	lib.Tune(domain.RoleChoices, "", cfg.companionEffort)
	for _, r := range lib.Roles() {
		log.Info("agent role", "role", r.Name, "model", r.Model, "effort", r.Effort, "tools", len(r.Tools))
	}
	return lib, nil
}

// openVault loads the master key that encrypts players' API keys: from
// TABLETOP_SECRET_KEY, else from <data>/secret.key, creating it on first run.
func openVault(dataDir string, log *slog.Logger) (*secure.Vault, error) {
	if k := os.Getenv("TABLETOP_SECRET_KEY"); k != "" {
		return secure.NewVault(k)
	}
	path := filepath.Join(dataDir, "secret.key")
	b, err := os.ReadFile(path) // #nosec G304 -- path is under the configured data dir
	if errors.Is(err, os.ErrNotExist) {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("generate master key: %w", err)
		}
		b = []byte(base64.StdEncoding.EncodeToString(raw))
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return nil, fmt.Errorf("write master key: %w", err)
		}
		log.Warn("created a new master key; back it up, or stored API keys cannot be decrypted", "path", path)
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("read master key: %w", err)
	}
	return secure.NewVault(strings.TrimSpace(string(b)))
}

// setUpAccounts creates the first admin from TABLETOP_ADMIN_USER/_PASSWORD, or the
// local dev account in -local mode, and hands pre-account games to that account.
func setUpAccounts(ctx context.Context, cfg config, people *accounts.UseCase, store *sqlite.Store, log *slog.Logger) (*domain.Account, error) {
	if cfg.local {
		a, err := people.Local(ctx)
		if err != nil {
			return nil, err
		}
		claimOrphans(ctx, store, a.ID, log)
		log.Warn("local mode: everyone who can reach this address is signed in as 'local'", "addr", cfg.addr)
		return &a, nil
	}
	user, pass := os.Getenv("TABLETOP_ADMIN_USER"), os.Getenv("TABLETOP_ADMIN_PASSWORD")
	if user == "" {
		if n, _ := store.CountAccounts(ctx); n == 0 {
			log.Warn("no accounts yet: set TABLETOP_ADMIN_USER and TABLETOP_ADMIN_PASSWORD and restart to create the admin")
		}
		return nil, nil
	}
	return nil, bootstrapAdmin(ctx, people, store, user, pass, log)
}

func bootstrapAdmin(ctx context.Context, people *accounts.UseCase, store *sqlite.Store, user, pass string, log *slog.Logger) error {
	created, err := people.Bootstrap(ctx, user, pass)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	admin, err := store.AccountByUsername(ctx, user)
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}
	if created {
		log.Info("admin account created", "username", user)
	}
	claimOrphans(ctx, store, admin.ID, log)
	return nil
}

// sweepDeadlines lets agents act for absent players whenever a deadline passes.
func sweepDeadlines(ctx context.Context, sweeper *deadlines.UseCase, log *slog.Logger) {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if n, err := sweeper.Sweep(ctx); err != nil || n > 0 {
				log.Info("deadline sweep", "games_moved_on", n, "err", err)
			}
		}
	}
}

// checkHealth asks the local server for /healthz.
func checkHealth(ctx context.Context, addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("address %q: %w", addr, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: HTTP %d", res.StatusCode)
	}
	return nil
}

// startBackups runs scheduled backups when a backup folder is configured.
func startBackups(ctx context.Context, cfg config, store *sqlite.Store, log *slog.Logger) error {
	if cfg.backupDir == "" {
		return nil
	}
	if err := os.MkdirAll(cfg.backupDir, 0o700); err != nil {
		return fmt.Errorf("backup folder: %w", err)
	}
	go backUpEvery(ctx, cfg.backupEvery, newBackups(store, cfg, log), log)
	return nil
}

// newBackups wires the backup use case to the store and the backup folder.
func newBackups(store *sqlite.Store, cfg config, log *slog.Logger) *backups.UseCase {
	return backups.New(store, system.Files{}, system.Clock{}, backups.Config{Dir: cfg.backupDir, Keep: cfg.backupKeep}, log)
}

// backUpEvery takes a backup shortly after start and then at each interval.
func backUpEvery(ctx context.Context, every time.Duration, b *backups.UseCase, log *slog.Logger) {
	wait := time.NewTimer(time.Minute)
	defer wait.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-wait.C:
			if _, err := b.Run(ctx); err != nil {
				log.Error("backup failed", "err", err)
			}
			wait.Reset(every)
		}
	}
}

// backupOnce takes a single backup and exits (for cron or by hand).
func backupOnce(ctx context.Context, cfg config, log *slog.Logger) error {
	if cfg.backupDir == "" {
		return errors.New("-backup-now needs -backup-dir (or TABLETOP_BACKUP_DIR)")
	}
	if err := os.MkdirAll(cfg.backupDir, 0o700); err != nil {
		return fmt.Errorf("backup folder: %w", err)
	}
	store, err := sqlite.Open(ctx, cfg.db)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	path, err := newBackups(store, cfg, log).Run(ctx)
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

// importLegacyHeroes copies party members of pre-roster games into their owners'
// rosters (once), bringing their portraits along.
func importLegacyHeroes(ctx context.Context, store *sqlite.Store, pictures *media.Store, log *slog.Logger) {
	got, err := store.ImportLegacyParties(ctx, system.IDs{}.New, system.Clock{}.Now())
	if err != nil {
		log.Warn("could not import heroes from older games", "err", err)
	}
	for _, im := range got {
		from := domain.PortraitKey(im.GameID, im.CreatureID)
		if _, err := pictures.Copy(from, domain.CharacterArtKey(im.CharacterID)); err != nil {
			log.Warn("could not copy portrait", "from", from, "err", err)
		}
	}
	if len(got) > 0 {
		log.Info("heroes from older games added to rosters", "characters", len(got))
	}
}

func claimOrphans(ctx context.Context, store *sqlite.Store, accountID string, log *slog.Logger) {
	n, err := store.ClaimOrphans(ctx, accountID)
	if err != nil {
		log.Warn("could not assign existing games", "err", err)
		return
	}
	if n > 0 {
		log.Info("existing games assigned to your account", "games", n)
	}
}
