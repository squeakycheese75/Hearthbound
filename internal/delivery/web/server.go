// Package web serves the browser UI, the JSON API and the live event stream.
package web

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"dnd/internal/usecase/accounts"
	"dnd/internal/usecase/characters"
	"dnd/internal/usecase/economy"
	"dnd/internal/usecase/market"
	"dnd/internal/usecase/newgame"
	"dnd/internal/usecase/speech"
	"dnd/internal/usecase/tavern"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// Creator sets up new games.
type Creator interface {
	Options() newgame.Options
	Execute(ctx context.Context, in newgame.Input) (newgame.Output, error)
}

// Library manages saved games.
type Library interface {
	List(ctx context.Context, accountID string) ([]domain.Summary, error)
	Get(ctx context.Context, accountID, id string) (*domain.Game, error)
	Delete(ctx context.Context, accountID, id string) error
}

// Table runs play.
type Table interface {
	Begin(ctx context.Context, accountID, gameID string) error
	Act(ctx context.Context, accountID, gameID, text string) error
	Fight(ctx context.Context, accountID, gameID string, opt rules.Option) error
	Options(ctx context.Context, accountID, gameID string) ([]rules.Option, error)
	Autopilot(ctx context.Context, accountID, gameID, characterID string, on bool) error
	Cover(ctx context.Context, accountID, gameID, characterID string, allow bool) error
	Recruit(ctx context.Context, accountID, gameID, characterID string) error
	OfferSeat(ctx context.Context, accountID, gameID, characterID string, open bool) error
	GiveSeatBack(ctx context.Context, accountID, gameID, characterID string) error
	PostWanted(ctx context.Context, accountID, gameID, text string) error
	AskForHero(ctx context.Context, accountID, gameID, characterID string) error
	AnswerCall(ctx context.Context, accountID, gameID, characterID string, yes bool) error
	SendHome(ctx context.Context, accountID, gameID, characterID string) error
	CheckCast(ctx context.Context, accountID, gameID string) error
	SetTyping(ctx context.Context, accountID, gameID string, mode domain.TypingMode) error
	Equip(ctx context.Context, accountID, gameID, characterID, itemID string, slot domain.Slot) error
	Limits() (dm, companion int)
	Busy(gameID string) bool
	Improve(ctx context.Context, accountID, gameID, characterID string, inc map[domain.Ability]int) error
}

// Gallery serves and repaints illustrations.
type Gallery interface {
	CanPaint(ctx context.Context, accountID string) bool
	Pending(key string) bool
	// Picture is the subject's own painting, else its stock image.
	Picture(key string, stockKeys ...string) (string, bool)
	Reroll(ctx context.Context, accountID, gameID, subjectID string) error
	Ensure(g *domain.Game)
	EnsureCharacter(ch *domain.Character)
}

// Roster manages a player's persistent characters.
type Roster interface {
	Create(ctx context.Context, in characters.CreateInput) (domain.Character, error)
	Random(ctx context.Context, in characters.RandomInput) (domain.Character, error)
	List(ctx context.Context, accountID string) ([]domain.Character, error)
	ForHire(ctx context.Context, accountID string) ([]domain.Character, error)
	Get(ctx context.Context, accountID, id string) (domain.Character, error)
	SetHire(ctx context.Context, accountID, id string, policy domain.HirePolicy) error
	SetGender(ctx context.Context, accountID, id string, g domain.Gender) error
	Improve(ctx context.Context, accountID, id string, inc map[domain.Ability]int) error
	Equip(ctx context.Context, accountID, id, itemID string, slot domain.Slot) error
	Retire(ctx context.Context, accountID, id string) error
}

// Market runs listings between players and the town shop.
type Market interface {
	Browse(ctx context.Context, accountID string) (market.View, error)
	List(ctx context.Context, in market.ListInput) (domain.Listing, error)
	Buy(ctx context.Context, accountID, listingID, characterID string) error
	Withdraw(ctx context.Context, accountID, listingID, characterID string) error
	Collect(ctx context.Context, accountID, listingID, characterID string) error
	Shop() []market.ShopItem
	ShopBuy(ctx context.Context, accountID, characterID, key string, qty int) error
	ShopSell(ctx context.Context, accountID, characterID, itemID string) (int, error)
}

// Economy reports on and steers the realm's money (admins only).
type Economy interface {
	Report(ctx context.Context, accountID string) (economy.Report, error)
	SetEconomy(ctx context.Context, accountID string, e domain.Economy) error
}

// Speech reads the story aloud with an AI voice, on the listener's key.
type Speech interface {
	Execute(ctx context.Context, in speech.Input) (speech.Output, error)
}

// Board is the tavern's notice board of open seats.
type Board interface {
	Board(ctx context.Context, accountID string) ([]tavern.Seat, error)
	Take(ctx context.Context, accountID, gameID, characterID string) error
	Offer(ctx context.Context, accountID, gameID, characterID string) error
	Notices(ctx context.Context, accountID string) (tavern.Notices, error)
}

// Auth signs people in and out.
type Auth interface {
	Authenticate(ctx context.Context, token string) (domain.Account, error)
	Login(ctx context.Context, username, password string) (domain.Session, error)
	Register(ctx context.Context, code, username, password string) (domain.Session, error)
	Logout(ctx context.Context, token string) error
}

// Settings manages an account's keys, budget and (for admins) invites.
type Settings interface {
	Keys(ctx context.Context, accountID string) ([]domain.KeyInfo, error)
	SetKey(ctx context.Context, accountID string, kind domain.KeyKind, secret string) error
	SetDailyTokenCap(ctx context.Context, accountID string, tokenCap int) error
	SetAgentModel(ctx context.Context, accountID, model string) error
	AgentModel(ctx context.Context, accountID string) (string, error)
	Username(ctx context.Context, accountID string) (string, error)
	Today(ctx context.Context, accountID string) (accounts.Spend, error)
	CreateInvite(ctx context.Context, adminID string, sponsored bool) (domain.Invite, error)
	Invites(ctx context.Context, adminID string) ([]domain.Invite, error)
	Players(ctx context.Context, adminID string) ([]accounts.Player, error)
	Sponsor(ctx context.Context, adminID, accountID string, tokenCap int) error
}

// Subscriber streams live events for a game.
type Subscriber interface {
	Subscribe(gameID string) (<-chan domain.Event, func())
}

// Clock tells the time, for logging how long turns take.
type Clock interface {
	Now() time.Time
}

// Deps are the server's collaborators. Static (the built UI) and Media (generated
// images) may be nil. Local, when set, signs every cookie-less request in as that
// account (single-user development mode).
type Deps struct {
	Creator  Creator
	Roster   Roster
	Library  Library
	Table    Table
	Events   Subscriber
	Gallery  Gallery
	Auth     Auth
	Settings Settings
	Market   Market
	Economy  Economy
	Speech   Speech
	Board    Board
	Static   fs.FS
	Media    fs.FS
	Clock    Clock
	Log      *slog.Logger
	Local    *domain.Account
	// Demo marks a server whose DM and companions are scripted, not AI.
	Demo bool
	// TrustProxy reads the caller's address from X-Forwarded-For, for a server
	// behind a reverse proxy (Caddy) that sets it.
	TrustProxy bool
	// Roles describes how the table's agents are set up (prompts left out).
	Roles []domain.AgentRole
}

// Server is the HTTP front end. Set Debug before calling Handler to log every
// request and expose GET /api/games/{id}/debug with the full agent transcripts.
type Server struct {
	Debug bool
	Deps
	guard *attempts
}

// New builds the server.
func New(d Deps) *Server {
	return &Server{Deps: d, guard: newAttempts()}
}

// CookieName is the session cookie.
const CookieName = "tabletop_session"

// maxBody caps JSON request bodies.
const maxBody = 1 << 20

// Handler returns the routed http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/me", s.authed(s.me))
	mux.HandleFunc("PUT /api/me/keys/{kind}", s.authed(s.setKey))
	mux.HandleFunc("PUT /api/me/cap", s.authed(s.setCap))
	mux.HandleFunc("PUT /api/me/agent-model", s.authed(s.setAgentModel))
	mux.HandleFunc("GET /api/admin/invites", s.authed(s.invites))
	mux.HandleFunc("POST /api/admin/invites", s.authed(s.createInvite))
	mux.HandleFunc("GET /api/admin/players", s.authed(s.players))
	mux.HandleFunc("PUT /api/admin/players/{id}/sponsor", s.authed(s.sponsor))
	mux.HandleFunc("GET /api/options", s.authed(s.options))
	mux.HandleFunc("GET /api/characters", s.authed(s.roster))
	mux.HandleFunc("GET /api/tavern", s.authed(s.tavern))
	mux.HandleFunc("GET /api/market", s.authed(s.browseMarket))
	mux.HandleFunc("POST /api/market/listings", s.authed(s.listItem))
	mux.HandleFunc("POST /api/market/listings/{id}/{action}", s.authed(s.listingAction))
	mux.HandleFunc("GET /api/shop", s.authed(s.shop))
	mux.HandleFunc("POST /api/speech", s.authed(s.speak))
	mux.HandleFunc("GET /api/admin/economy", s.authed(s.economyReport))
	mux.HandleFunc("PUT /api/admin/economy", s.authed(s.setEconomy))
	mux.HandleFunc("POST /api/shop/buy", s.authed(s.shopBuy))
	mux.HandleFunc("POST /api/shop/sell", s.authed(s.shopSell))
	mux.HandleFunc("POST /api/characters", s.authed(s.createCharacter))
	mux.HandleFunc("POST /api/characters/random", s.authed(s.randomCharacter))
	mux.HandleFunc("GET /api/characters/{id}", s.authed(s.character))
	mux.HandleFunc("PUT /api/characters/{id}/hire", s.authed(s.setHire))
	mux.HandleFunc("PUT /api/characters/{id}/gender", s.authed(s.setGender))
	mux.HandleFunc("PUT /api/characters/{id}/improve", s.authed(s.improveInTown))
	mux.HandleFunc("POST /api/games/{id}/improve", s.authed(s.improveInGame))
	mux.HandleFunc("PUT /api/characters/{id}/equip", s.authed(s.equipInTown))
	mux.HandleFunc("POST /api/games/{id}/equip", s.authed(s.equipInGame))
	mux.HandleFunc("DELETE /api/characters/{id}", s.authed(s.retire))
	mux.HandleFunc("GET /api/games", s.authed(s.list))
	mux.HandleFunc("POST /api/games", s.authed(s.create))
	mux.HandleFunc("GET /api/games/{id}", s.authed(s.get))
	mux.HandleFunc("DELETE /api/games/{id}", s.authed(s.remove))
	mux.HandleFunc("POST /api/games/{id}/begin", s.authed(s.begin))
	mux.HandleFunc("POST /api/games/{id}/act", s.authed(s.act))
	mux.HandleFunc("POST /api/games/{id}/fight", s.authed(s.fight))
	mux.HandleFunc("POST /api/games/{id}/autopilot", s.authed(s.autopilot))
	mux.HandleFunc("POST /api/games/{id}/cover", s.authed(s.cover))
	mux.HandleFunc("POST /api/games/{id}/recruit", s.authed(s.partyChange("recruit", Table.Recruit)))
	mux.HandleFunc("POST /api/games/{id}/send-home", s.authed(s.partyChange("send home", Table.SendHome)))
	mux.HandleFunc("POST /api/games/{id}/cast", s.authed(s.checkCast))
	mux.HandleFunc("POST /api/games/{id}/seats", s.authed(s.offerSeat))
	mux.HandleFunc("POST /api/games/{id}/give-back", s.authed(s.giveSeatBack))
	mux.HandleFunc("GET /api/tavern/seats", s.authed(s.openSeats))
	mux.HandleFunc("POST /api/tavern/seats/take", s.authed(s.takeSeat))
	mux.HandleFunc("GET /api/tavern/notices", s.authed(s.notices))
	mux.HandleFunc("POST /api/tavern/offer", s.authed(s.offerHero))
	mux.HandleFunc("POST /api/games/{id}/wanted", s.authed(s.postWanted))
	mux.HandleFunc("POST /api/games/{id}/ask", s.authed(s.askForHero))
	mux.HandleFunc("POST /api/games/{id}/answer", s.authed(s.answerCall))
	mux.HandleFunc("PUT /api/games/{id}/typing", s.authed(s.setTyping))
	mux.HandleFunc("GET /api/games/{id}/events", s.authed(s.stream))
	mux.HandleFunc("POST /api/games/{id}/art/{subject}", s.authed(s.reroll))
	if s.Media != nil {
		media := http.StripPrefix("/media/", http.FileServerFS(s.Media))
		mux.HandleFunc("GET /media/", s.authed(func(w http.ResponseWriter, r *http.Request, _ domain.Account) {
			media.ServeHTTP(w, r)
		}))
	}
	if s.Debug {
		mux.HandleFunc("GET /api/games/{id}/debug", s.authed(s.debug))
	}
	if s.Static != nil {
		mux.Handle("/", spa(s.Static))
	}
	h := securityHeaders(sameOrigin(mux))
	if s.Debug {
		h = s.logRequests(h)
	}
	return h
}

// sameOrigin rejects state-changing requests sent from another site.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-site request refused"})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

type handlerWithAccount func(w http.ResponseWriter, r *http.Request, a domain.Account)

// authed resolves the session cookie (or the local dev account) before calling next.
func (s *Server) authed(next handlerWithAccount) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		c, err := r.Cookie(CookieName)
		if err == nil {
			a, authErr := s.Auth.Authenticate(r.Context(), c.Value)
			if authErr == nil {
				next(w, r, a)
				return
			}
		}
		if s.Local != nil {
			next(w, r, *s.Local)
			return
		}
		s.fail(w, fmt.Errorf("%w: please sign in", domain.ErrUnauthorized))
	}
}

// overHTTPS reports whether the browser reached us over TLS (directly or via Caddy).
func overHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, sess domain.Session) {
	// #nosec G124 -- Secure is set whenever the request arrived over HTTPS; plain HTTP is only local development.
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: sess.Token, Path: "/", Expires: sess.ExpiresAt,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: overHTTPS(r),
	})
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Invite   string `json:"invite"`
}

func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		s.fail(w, fmt.Errorf("%w: %w", domain.ErrInvalid, err))
		return false
	}
	return true
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !s.decode(w, r, &in) {
		return
	}
	now := s.Clock.Now()
	keys := []string{"user:" + strings.ToLower(in.Username), "addr:" + s.clientAddr(r)}
	if s.guard.blocked(now, keys...) {
		s.Log.Warn("sign-in throttled", "username", in.Username, "addr", s.clientAddr(r))
		s.fail(w, fmt.Errorf("%w: too many failed sign-ins; try again in a few minutes", domain.ErrTooManyAttempts))
		return
	}
	sess, err := s.Auth.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		s.Log.Info("sign-in failed", "username", in.Username)
		s.guard.fail(now, keys...)
		s.fail(w, err)
		return
	}
	s.guard.clear(keys[0])
	s.setSession(w, r, sess)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !s.decode(w, r, &in) {
		return
	}
	now, addr := s.Clock.Now(), "addr:"+s.clientAddr(r)
	if s.guard.blocked(now, addr) {
		s.fail(w, fmt.Errorf("%w: too many failed attempts; try again in a few minutes", domain.ErrTooManyAttempts))
		return
	}
	sess, err := s.Auth.Register(r.Context(), in.Invite, in.Username, in.Password)
	if err != nil {
		s.guard.fail(now, addr) // guessing invite codes costs attempts
		s.fail(w, err)
		return
	}
	s.Log.Info("account registered", "username", in.Username)
	s.setSession(w, r, sess)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		if err := s.Auth.Logout(r.Context(), c.Value); err != nil {
			s.fail(w, err)
			return
		}
	}
	// #nosec G124 -- clearing the cookie; Secure mirrors how it was set.
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: overHTTPS(r)})
	w.WriteHeader(http.StatusNoContent)
}

// Me is the signed-in account's profile, keys (never the secrets) and spend.
type Me struct {
	Account domain.Account   `json:"account"`
	Keys    []domain.KeyInfo `json:"keys"`
	Spend   accounts.Spend   `json:"spend"`
	Local   bool             `json:"local"`
	Demo    bool             `json:"demo"`
	// Roles are the table's agent definitions; Models are what a player may
	// pick for their own characters.
	Roles  []domain.AgentRole  `json:"roles"`
	Models []domain.AgentModel `json:"models"`
	// Voices are the AI narration voices on offer.
	Voices []string `json:"voices"`
	// Backdrop is the art pack's tavern painting, for the front page ("" if none).
	Backdrop string `json:"backdrop,omitempty"`
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, a domain.Account) {
	keys, err := s.Settings.Keys(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	spend, err := s.Settings.Today(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	// The signed-in account can be a snapshot (-local), so read settings fresh.
	if a.AgentModel, err = s.Settings.AgentModel(r.Context(), a.ID); err != nil {
		s.fail(w, err)
		return
	}
	a.DailyTokenCap = spend.Cap
	writeJSON(w, http.StatusOK, Me{
		Account: a, Keys: keys, Spend: spend, Local: s.Local != nil && s.Local.ID == a.ID, Demo: s.Demo,
		Roles: s.Roles, Models: domain.AgentModels, Voices: domain.SpeechVoices, Backdrop: s.backdrop(),
	})
}

func (s *Server) setKey(w http.ResponseWriter, r *http.Request, a domain.Account) {
	kind, err := domain.ParseKeyKind(r.PathValue("kind"))
	if err != nil {
		s.fail(w, err)
		return
	}
	var in struct {
		Key string `json:"key"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Settings.SetKey(r.Context(), a.ID, kind, in.Key); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setCap(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Cap int `json:"cap"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Settings.SetDailyTokenCap(r.Context(), a.ID, in.Cap); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setAgentModel(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Model string `json:"model"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Settings.SetAgentModel(r.Context(), a.ID, in.Model); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) invites(w http.ResponseWriter, r *http.Request, a domain.Account) {
	out, err := s.Settings.Invites(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Sponsored bool `json:"sponsored"`
	}
	if r.ContentLength > 0 && !s.decode(w, r, &in) {
		return
	}
	inv, err := s.Settings.CreateInvite(r.Context(), a.ID, in.Sponsored)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) players(w http.ResponseWriter, r *http.Request, a domain.Account) {
	out, err := s.Settings.Players(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// sponsor sets how many tokens a day the admin pays for a player (0 stops).
func (s *Server) sponsor(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		TokenCap int `json:"tokenCap"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Settings.Sponsor(r.Context(), a.ID, r.PathValue("id"), in.TokenCap))
}

// debug returns the whole saved game, including what every agent was sent and replied.
func (s *Server) debug(w http.ResponseWriter, r *http.Request, a domain.Account) {
	g, err := s.Library.Get(r.Context(), a.ID, r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// statusWriter records the response status and keeps http.Flusher working for SSE.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.Clock.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.Log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", sw.status,
			"took", s.Clock.Now().Sub(start).Round(time.Millisecond))
	})
}

// CharacterView is a roster character with its portrait.
type CharacterView struct {
	domain.Character
	Portrait string `json:"portrait,omitempty"`
	Painting bool   `json:"painting"`
	// XPLevels is the XP needed to reach each level (index 0 is level 1), up
	// to the highest level the game supports, for the XP progress bar.
	XPLevels []int `json:"xpLevels"`
}

func (s *Server) view(ch domain.Character) CharacterView {
	s.Gallery.EnsureCharacter(&ch)
	key := domain.CharacterArtKey(ch.ID)
	var stock []string
	if ch.Sheet != nil {
		stock = domain.StockKeysFor(ch.Sheet)
	}
	url, _ := s.Gallery.Picture(key, stock...)
	return CharacterView{Character: ch, Portrait: url, Painting: s.Gallery.Pending(key), XPLevels: rules.XPThresholds[:rules.MaxLevel]}
}

func (s *Server) roster(w http.ResponseWriter, r *http.Request, a domain.Account) {
	list, err := s.Roster.List(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]CharacterView, 0, len(list))
	for _, ch := range list {
		out = append(out, s.view(ch))
	}
	writeJSON(w, http.StatusOK, out)
}

// ListingView is a listing with its seller's name, from the viewer's side.
type ListingView struct {
	domain.Listing
	SellerName string `json:"sellerName"`
	Mine       bool   `json:"mine"`
}

func (s *Server) listingViews(ctx context.Context, viewer string, ls []domain.Listing, names map[string]string) []ListingView {
	out := make([]ListingView, 0, len(ls))
	for _, l := range ls {
		if _, ok := names[l.SellerID]; !ok {
			names[l.SellerID], _ = s.Settings.Username(ctx, l.SellerID) // a missing seller just shows no name
		}
		out = append(out, ListingView{Listing: l, SellerName: names[l.SellerID], Mine: l.SellerID == viewer})
	}
	return out
}

func (s *Server) browseMarket(w http.ResponseWriter, r *http.Request, a domain.Account) {
	v, err := s.Market.Browse(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	names := map[string]string{}
	writeJSON(w, http.StatusOK, map[string]any{
		"open": s.listingViews(r.Context(), a.ID, v.Open, names), "mine": s.listingViews(r.Context(), a.ID, v.Mine, names),
		"feePercent": v.FeePercent,
	})
}

func (s *Server) listItem(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in market.ListInput
	if !s.decode(w, r, &in) {
		return
	}
	in.AccountID = a.ID
	l, err := s.Market.List(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

// listingAction buys, withdraws or collects a listing with one of the
// account's heroes in town.
func (s *Server) listingAction(w http.ResponseWriter, r *http.Request, a domain.Account) {
	actions := map[string]func(ctx context.Context, accountID, listingID, characterID string) error{
		"buy": s.Market.Buy, "withdraw": s.Market.Withdraw, "collect": s.Market.Collect,
	}
	act, ok := actions[r.PathValue("action")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	var in struct {
		CharacterID string `json:"characterId"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := act(r.Context(), a.ID, r.PathValue("id"), in.CharacterID); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) shop(w http.ResponseWriter, r *http.Request, a domain.Account) {
	v, err := s.Market.Browse(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.Market.Shop(), "buyBackPercent": v.BuyBackPercent})
}

func (s *Server) economyReport(w http.ResponseWriter, r *http.Request, a domain.Account) {
	rep, err := s.Economy.Report(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) setEconomy(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var e domain.Economy
	if !s.decode(w, r, &e) {
		return
	}
	if err := s.Economy.SetEconomy(r.Context(), a.ID, e); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) speak(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in speech.Input
	if !s.decode(w, r, &in) {
		return
	}
	in.AccountID = a.ID
	out, err := s.Speech.Execute(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", out.ContentType)
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(out.Audio)
}

func (s *Server) shopBuy(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		CharacterID string `json:"characterId"`
		Key         string `json:"key"`
		Qty         int    `json:"qty"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Market.ShopBuy(r.Context(), a.ID, in.CharacterID, in.Key, in.Qty); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) shopSell(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		CharacterID string `json:"characterId"`
		ItemID      string `json:"itemId"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	paid, err := s.Market.ShopSell(r.Context(), a.ID, in.CharacterID, in.ItemID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"paid": paid})
}

// TavernHero is another player's hero looking for adventure.
type TavernHero struct {
	CharacterView
	OwnerName string `json:"ownerName"`
}

// tavern lists other players' heroes in town who are for hire.
func (s *Server) tavern(w http.ResponseWriter, r *http.Request, a domain.Account) {
	list, err := s.Roster.ForHire(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	names := map[string]string{}
	out := make([]TavernHero, 0, len(list))
	for _, ch := range list {
		if _, ok := names[ch.OwnerID]; !ok {
			names[ch.OwnerID], _ = s.Settings.Username(r.Context(), ch.OwnerID) // a missing owner just shows no name
		}
		out = append(out, TavernHero{CharacterView: s.view(ch), OwnerName: names[ch.OwnerID]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"forHire": out})
}

func (s *Server) createCharacter(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in characters.CreateInput
	if !s.decode(w, r, &in) {
		return
	}
	in.AccountID = a.ID
	ch, err := s.Roster.Create(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.view(ch))
}

func (s *Server) randomCharacter(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in characters.RandomInput
	if !s.decode(w, r, &in) {
		return
	}
	in.AccountID = a.ID
	ch, err := s.Roster.Random(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.view(ch))
}

func (s *Server) character(w http.ResponseWriter, r *http.Request, a domain.Account) {
	ch, err := s.Roster.Get(r.Context(), a.ID, r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view(ch))
}

// equipRequest is an item to put in a slot ("" to take it off).
type equipRequest struct {
	CharacterID string `json:"characterId"`
	ItemID      string `json:"itemId"`
	Slot        string `json:"slot"`
}

func (s *Server) readEquip(w http.ResponseWriter, r *http.Request) (equipRequest, domain.Slot, bool) {
	var in equipRequest
	if !s.decode(w, r, &in) {
		return in, "", false
	}
	slot, err := domain.ParseSlot(in.Slot)
	if err != nil {
		s.fail(w, err)
		return in, "", false
	}
	return in, slot, true
}

func (s *Server) equipInTown(w http.ResponseWriter, r *http.Request, a domain.Account) {
	in, slot, ok := s.readEquip(w, r)
	if !ok {
		return
	}
	if err := s.Roster.Equip(r.Context(), a.ID, r.PathValue("id"), in.ItemID, slot); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) equipInGame(w http.ResponseWriter, r *http.Request, a domain.Account) {
	in, slot, ok := s.readEquip(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	s.timed(w, "equip", a, id, func() error {
		return s.Table.Equip(context.WithoutCancel(r.Context()), a.ID, id, in.CharacterID, in.ItemID, slot)
	})
}

func (s *Server) setHire(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Hire string `json:"hire"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	policy, err := domain.ParseHirePolicy(in.Hire)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.Roster.SetHire(r.Context(), a.ID, r.PathValue("id"), policy); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setGender(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Gender domain.Gender `json:"gender"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Roster.SetGender(r.Context(), a.ID, r.PathValue("id"), in.Gender); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// improvement is an Ability Score Improvement choice, e.g. {"STR":2}.
type improvement struct {
	CharacterID string                 `json:"characterId"`
	Increases   map[domain.Ability]int `json:"increases"`
}

func (s *Server) improveInTown(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in improvement
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Roster.Improve(r.Context(), a.ID, r.PathValue("id"), in.Increases); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) improveInGame(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in improvement
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.Table.Improve(r.Context(), a.ID, r.PathValue("id"), in.CharacterID, in.Increases); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// seatRequest names a seat: a character in a game.
type seatRequest struct {
	GameID      string `json:"gameId"`
	CharacterID string `json:"characterId"`
	Open        bool   `json:"open"`
	Yes         bool   `json:"yes"`
	Text        string `json:"text"`
}

func (s *Server) notices(w http.ResponseWriter, r *http.Request, a domain.Account) {
	out, err := s.Board.Notices(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) offerHero(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Board.Offer(r.Context(), a.ID, in.GameID, in.CharacterID))
}

func (s *Server) postWanted(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Table.PostWanted(r.Context(), a.ID, r.PathValue("id"), in.Text))
}

func (s *Server) askForHero(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Table.AskForHero(r.Context(), a.ID, r.PathValue("id"), in.CharacterID))
}

func (s *Server) answerCall(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Table.AnswerCall(r.Context(), a.ID, r.PathValue("id"), in.CharacterID, in.Yes))
}

func (s *Server) offerSeat(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Table.OfferSeat(r.Context(), a.ID, r.PathValue("id"), in.CharacterID, in.Open))
}

func (s *Server) giveSeatBack(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Table.GiveSeatBack(r.Context(), a.ID, r.PathValue("id"), in.CharacterID))
}

func (s *Server) takeSeat(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in seatRequest
	if !s.decode(w, r, &in) {
		return
	}
	s.done(w, s.Board.Take(r.Context(), a.ID, in.GameID, in.CharacterID))
}

// SeatView is an open seat with its character's portrait.
type SeatView struct {
	tavern.Seat
	Portrait string `json:"portrait,omitempty"`
}

func (s *Server) openSeats(w http.ResponseWriter, r *http.Request, a domain.Account) {
	seats, err := s.Board.Board(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]SeatView, 0, len(seats))
	for _, seat := range seats {
		c := seat.Character
		url, _ := s.Gallery.Picture(domain.CharacterArtKey(c.ID), domain.StockKeysFor(&c)...)
		out = append(out, SeatView{Seat: seat, Portrait: url})
	}
	writeJSON(w, http.StatusOK, out)
}

// done answers 204, or the error.
func (s *Server) done(w http.ResponseWriter, err error) {
	if err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) retire(w http.ResponseWriter, r *http.Request, a domain.Account) {
	if err := s.Roster.Retire(r.Context(), a.ID, r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) options(w http.ResponseWriter, _ *http.Request, _ domain.Account) {
	writeJSON(w, http.StatusOK, s.Creator.Options())
}

// backdrop is the front page's painting: the art pack's tavern.
func (s *Server) backdrop() string {
	url, _ := s.Gallery.Picture("", domain.StockSceneKey("tavern"))
	return url
}

// SummaryView is an adventure in the list, with its current scene's painting.
type SummaryView struct {
	domain.Summary
	Art string `json:"art,omitempty"`
}

func (s *Server) list(w http.ResponseWriter, r *http.Request, a domain.Account) {
	games, err := s.Library.List(r.Context(), a.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]SummaryView, 0, len(games))
	for _, g := range games {
		v := SummaryView{Summary: g}
		if g.SceneKey != "" {
			scene := domain.Scene{Key: g.SceneKey, Title: g.Scene}
			v.Art, _ = s.Gallery.Picture(domain.SceneArtKey(g.ID, scene), domain.StockSceneKey(g.SceneKey))
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) create(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in newgame.Input
	if !s.decode(w, r, &in) {
		return
	}
	in.AccountID = a.ID
	out, err := s.Creator.Execute(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// GameView is a game as the browser sees it: no agent transcripts, plus the
// hero's options and whether a turn is still being resolved.
type GameView struct {
	*domain.Game
	Options []rules.Option `json:"options"`
	Busy    bool           `json:"busy"`
	// Art maps creature IDs (and "scene") to finished illustration URLs.
	Art map[string]string `json:"art"`
	// Painting lists creature IDs (and "scene") whose art is being generated.
	Painting   []string `json:"painting"`
	ArtEnabled bool     `json:"artEnabled"`
	// Mine lists the characters the viewer plays in this party.
	Mine []string `json:"mine"`
	// Players names the friends playing seats, by character ID.
	Players map[string]string `json:"players"`
	// YourTurn is true when the table is waiting on the viewer.
	YourTurn bool `json:"yourTurn"`
	// WaitingOn names the live characters the table is waiting for.
	WaitingOn []string `json:"waitingOn"`
	// Paused is true when every character is left to its agent, so the story waits.
	Paused bool `json:"paused"`
	// You is the viewer's account ID, so the UI can say whose key pays for what.
	You string `json:"you"`
	// AtTavern is true when the party can take on or send home members.
	AtTavern bool `json:"atTavern"`
	// MayType is false when this table plays from the quick choices for the viewer.
	MayType bool `json:"mayType"`
	// MaxAction is the longest action the viewer may type.
	MaxAction int `json:"maxAction"`
	// CastStale is true when the latest narration has not been checked for
	// NPCs yet; the browser asks for a check (POST /cast) when it sees it.
	CastStale bool `json:"castStale"`
	// PartyLimit is the most characters a party can hold.
	PartyLimit int `json:"partyLimit"`
	// ChapterAt is the prompt size (tokens) at which each kind of agent closes a chapter.
	ChapterAt ChapterLimits `json:"chapterAt"`
}

// ChapterLimits are the DM's and a companion's chapter thresholds.
type ChapterLimits struct {
	DM        int `json:"dm"`
	Companion int `json:"companion"`
}

// addSeats fills in who plays what, from the viewer's point of view.
func (v *GameView) addSeats(accountID string) {
	g := v.Game
	v.Paused = g.Mode != domain.ModeOver && !g.Playing()
	for _, c := range g.PartyMembers() {
		if domain.PlayerOf(c) == accountID {
			v.Mine = append(v.Mine, c.ID)
		}
	}
	if !g.Awaiting {
		return
	}
	for _, c := range g.WaitingSeats() {
		v.WaitingOn = append(v.WaitingOn, c.Name)
		v.YourTurn = v.YourTurn || domain.PlayerOf(c) == accountID
	}
}

func (s *Server) get(w http.ResponseWriter, r *http.Request, a domain.Account) {
	id := r.PathValue("id")
	g, err := s.Library.Get(r.Context(), a.ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	opts, err := s.Table.Options(r.Context(), a.ID, id)
	if err != nil {
		s.fail(w, err)
		return
	}
	g.Transcripts = nil
	if g.Log == nil {
		g.Log = []domain.Event{} // a game that hasn't begun has no story yet
	}
	s.Gallery.Ensure(g) // retries any art lost to a restart
	busy := s.Table.Busy(id)
	if busy {
		g.Awaiting = false
	}
	view := GameView{
		Game: g, Options: opts, Busy: busy, ArtEnabled: s.Gallery.CanPaint(r.Context(), a.ID),
		Art: map[string]string{}, Painting: []string{}, Mine: []string{}, WaitingOn: []string{}, Players: map[string]string{},
	}
	view.addSeats(a.ID)
	for _, c := range g.PartyMembers() {
		if c.PlayerID != "" {
			view.Players[c.ID], _ = s.Settings.Username(r.Context(), c.PlayerID)
		}
	}
	view.You = a.ID
	view.AtTavern, view.PartyLimit, view.CastStale = g.AtTavern(), domain.MaxParty, g.CastStale()
	view.MayType, view.MaxAction = g.MayType(a.ID), domain.MaxAction
	g.Choices = g.ChoicesNow() // the fallbacks show when the DM offered none
	view.ChapterAt.DM, view.ChapterAt.Companion = s.Table.Limits()
	s.addArt(&view, domain.SceneSubject, domain.SceneArtKey(g.ID, g.Scene), domain.StockSceneKey(g.Scene.Key))
	s.addArt(&view, domain.BattlemapSubject, "", domain.StockBattlemapKey(g.Scene.Key))
	for _, n := range g.NPCs {
		s.addArt(&view, n.ID, domain.NPCArtKey(g.ID, n), domain.StockKeysForNPC(n)...)
	}
	for _, c := range g.Creatures {
		s.addArt(&view, c.ID, domain.CreatureArtKey(g, c), domain.StockKeysFor(c)...)
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) addArt(v *GameView, subject, key string, stockKeys ...string) {
	if url, ok := s.Gallery.Picture(key, stockKeys...); ok {
		v.Art[subject] = url
	}
	if s.Gallery.Pending(key) {
		v.Painting = append(v.Painting, subject)
	}
}

func (s *Server) reroll(w http.ResponseWriter, r *http.Request, a domain.Account) {
	if err := s.Gallery.Reroll(r.Context(), a.ID, r.PathValue("id"), r.PathValue("subject")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request, a domain.Account) {
	if err := s.Library.Delete(r.Context(), a.ID, r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Play actions run detached from the request so a closed tab never cuts a turn in half.
func (s *Server) begin(w http.ResponseWriter, r *http.Request, a domain.Account) {
	id := r.PathValue("id")
	s.timed(w, "begin", a, id, func() error { return s.Table.Begin(context.WithoutCancel(r.Context()), a.ID, id) })
}

// timed runs one play action, logs how long it took and how it ended, and writes the response.
func (s *Server) timed(w http.ResponseWriter, action string, a domain.Account, gameID string, fn func() error) {
	start := s.Clock.Now()
	s.Log.Info("turn started", "action", action, "game", gameID, "account", a.Username)
	err := fn()
	took := s.Clock.Now().Sub(start).Round(time.Millisecond)
	if err != nil {
		s.Log.Warn("turn failed", "action", action, "game", gameID, "account", a.Username, "took", took, "err", err)
		s.fail(w, err)
		return
	}
	s.Log.Info("turn finished", "action", action, "game", gameID, "account", a.Username, "took", took)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) act(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Text string `json:"text"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	s.timed(w, "act", a, id, func() error { return s.Table.Act(context.WithoutCancel(r.Context()), a.ID, id, in.Text) })
}

func (s *Server) fight(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var opt rules.Option
	if !s.decode(w, r, &opt) {
		return
	}
	id := r.PathValue("id")
	s.timed(w, "fight", a, id, func() error { return s.Table.Fight(context.WithoutCancel(r.Context()), a.ID, id, opt) })
}

func (s *Server) autopilot(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		CharacterID string `json:"characterId"`
		On          bool   `json:"on"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	s.timed(w, "autopilot", a, id, func() error {
		return s.Table.Autopilot(context.WithoutCancel(r.Context()), a.ID, id, in.CharacterID, in.On)
	})
}

func (s *Server) cover(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		CharacterID string `json:"characterId"`
		Allow       bool   `json:"allow"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	s.timed(w, "cover", a, id, func() error {
		return s.Table.Cover(context.WithoutCancel(r.Context()), a.ID, id, in.CharacterID, in.Allow)
	})
}

func (s *Server) setTyping(w http.ResponseWriter, r *http.Request, a domain.Account) {
	var in struct {
		Mode string `json:"mode"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	mode, err := domain.ParseTypingMode(in.Mode)
	if err != nil {
		s.fail(w, err)
		return
	}
	id := r.PathValue("id")
	s.timed(w, "typing", a, id, func() error { return s.Table.SetTyping(context.WithoutCancel(r.Context()), a.ID, id, mode) })
}

func (s *Server) checkCast(w http.ResponseWriter, r *http.Request, a domain.Account) {
	id := r.PathValue("id")
	s.timed(w, "cast check", a, id, func() error { return s.Table.CheckCast(context.WithoutCancel(r.Context()), a.ID, id) })
}

// partyChange handles recruit and send-home, which take a character ID.
func (s *Server) partyChange(name string, fn func(t Table, ctx context.Context, accountID, gameID, characterID string) error) func(http.ResponseWriter, *http.Request, domain.Account) {
	return func(w http.ResponseWriter, r *http.Request, a domain.Account) {
		var in struct {
			CharacterID string `json:"characterId"`
		}
		if !s.decode(w, r, &in) {
			return
		}
		id := r.PathValue("id")
		s.timed(w, name, a, id, func() error {
			return fn(s.Table, context.WithoutCancel(r.Context()), a.ID, id, in.CharacterID)
		})
	}
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request, a domain.Account) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	if _, err := s.Library.Get(r.Context(), a.ID, id); err != nil {
		s.fail(w, err)
		return
	}
	events, cancel := s.Events.Subscribe(id)
	defer cancel()
	s.Log.Debug("event stream opened", "game", id, "account", a.Username)
	defer s.Log.Debug("event stream closed", "game", id, "account", a.Username)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case e := <-events:
			data, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	status := statusFor(err)
	if status >= http.StatusInternalServerError {
		s.Log.Error("request failed", "err", err)
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrBusy):
		return http.StatusConflict
	case errors.Is(err, domain.ErrOverBudget), errors.Is(err, domain.ErrTooManyAttempts):
		return http.StatusTooManyRequests
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, domain.ErrUnknown), errors.Is(err, domain.ErrNoKey),
		errors.Is(err, domain.ErrWrongMode), errors.Is(err, domain.ErrNotYourTurn):
		return http.StatusBadRequest
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// spa serves static files, falling back to index.html for client-side routes.
func spa(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[1:]
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(static, path); err != nil {
			if filepath.Ext(path) != "" {
				http.NotFound(w, r) // a missing file, not an app route: never answer it with the page
				return
			}
			r.URL.Path, path = "/", "index.html"
		}
		w.Header().Set("Cache-Control", cacheFor(path))
		files.ServeHTTP(w, r)
	})
}

// cacheFor keeps the page itself fresh, so a rebuilt binary's UI is picked up
// on the next load, while Vite's content-hashed assets cache for good.
func cacheFor(path string) string {
	if strings.HasPrefix(path, "assets/") {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}
