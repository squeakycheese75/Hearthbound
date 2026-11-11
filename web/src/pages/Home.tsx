import { motion } from "motion/react";
import { useEffect, useMemo, useState } from "react";
import type { Character, Me, Notice, SummaryView } from "../api";
import { NoticeInbox } from "../components/Calls";
import { HeroCard } from "../components/HeroCard";
import { api, untilText } from "../api";
import { SceneArt } from "../components/SceneArt";
import { navigate } from "../router";

/** Where players can get this server's source (AGPL-3.0 §13). Forks: change it to your repository. */
const SOURCE_URL = "https://github.com/squeakycheese75/Hearthbound";

export function Home({ me, onSignOut }: { me: Me; onSignOut: () => void }) {
  const needsKey = !me.local && !me.spend.sponsorCap && !me.keys.find((k) => k.kind === "anthropic")?.set;
  const [games, setGames] = useState<SummaryView[] | null>(null);
  const [roster, setRoster] = useState<Character[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = () =>
    api
      .list()
      .then(setGames)
      .catch((e: Error) => setError(e.message));
  const [openSeats, setOpenSeats] = useState(0);
  const [inbox, setInbox] = useState<Notice[]>([]);
  const [posts, setPosts] = useState(0);
  const loadNotices = () =>
    api
      .notices()
      .then((n) => {
        setInbox(n.inbox);
        setPosts(n.posts.filter((p) => !p.offered).length);
      })
      .catch(() => undefined);
  useEffect(() => {
    void load();
    api.roster().then(setRoster).catch((e: Error) => setError(e.message));
    api
      .tavernSeats()
      .then((s) => setOpenSeats(s.length))
      .catch(() => undefined);
    void loadNotices();
  }, []);
  const onBoard = openSeats + posts;

  const remove = async (g: SummaryView) => {
    if (!confirm(`Delete "${g.title}"? This cannot be undone.`)) return;
    await api.remove(g.id).catch((e: Error) => setError(e.message));
    void load();
  };

  const fallen = roster?.filter((c) => c.status === "fallen") ?? [];
  const waiting = games?.find((g) => g.yourTurn);
  const latest = games?.find((g) => g.mode !== "over");
  const resume = waiting ?? latest;

  return (
    <div className="min-h-full">
      <nav className="absolute inset-x-0 top-0 z-20 flex items-center justify-end gap-4 px-5 py-3 text-[15px]">
        <span className="text-bone/80">{me.account.username}</span>
        <button onClick={() => navigate("/tavern")} className="relative text-gold hover:text-gold-2" title="Seats and parties looking for heroes">
          Tavern
          {onBoard > 0 && (
            <span className="absolute -right-3 -top-2 rounded-full bg-ember px-1.5 text-[11px] font-display text-ink">{onBoard}</span>
          )}
        </button>
        <button onClick={() => navigate("/market")} className="text-gold hover:text-gold-2">
          Market
        </button>
        <button onClick={() => navigate("/settings")} className="text-gold hover:text-gold-2">
          Settings
        </button>
        {!me.local && (
          <button onClick={onSignOut} className="text-mist hover:text-bone">
            Sign out
          </button>
        )}
      </nav>

      <Hearth backdrop={me.backdrop}>
        <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.9 }} className="flex flex-col items-center">
          <span className="font-display text-[12px] sm:text-[13px] tracking-[0.3em] sm:tracking-[0.5em] uppercase text-gold/80">A tale told by firelight</span>
          <h1 className="mt-2 font-display text-[2.6rem] sm:text-7xl font-bold tracking-wide text-gold-2 drop-shadow-[0_4px_24px_rgba(0,0,0,.9)]">
            Hearthbound
          </h1>
          <Flourish />
          <p className="mt-3 max-w-xl px-2 text-base sm:text-lg text-bone/90 drop-shadow-[0_2px_8px_rgba(0,0,0,.9)]">
            Pull up a chair by the fire. An AI Dungeon Master spins the tale, companions fill your party, and every roll is real.
          </p>
          <div className="mt-7 flex flex-wrap justify-center gap-3">
            {resume && (
              <button
                onClick={() => navigate(`/play/${resume.id}`)}
                className="rounded-lg bg-gold px-7 py-3 font-display text-lg tracking-wider text-ink shadow-[0_8px_30px_rgba(201,162,74,.4)] transition-transform hover:-translate-y-0.5 hover:bg-gold-2"
              >
                {waiting ? `Your turn: ${waiting.title}` : `Continue ${resume.title}`}
              </button>
            )}
            <button
              onClick={() => navigate("/new")}
              className={
                resume
                  ? "rounded-lg bg-ink/70 px-7 py-3 font-display text-lg tracking-wider text-gold-2 ring-1 ring-gold/60 backdrop-blur transition-transform hover:-translate-y-0.5 hover:bg-ink-3/80"
                  : "rounded-lg bg-gold px-7 py-3 font-display text-lg tracking-wider text-ink shadow-[0_8px_30px_rgba(201,162,74,.4)] transition-transform hover:-translate-y-0.5 hover:bg-gold-2"
              }
            >
              Begin a new adventure
            </button>
          </div>
        </motion.div>
      </Hearth>

      <main className="relative mx-auto max-w-5xl px-4 pb-16">
        {needsKey && (
          <div className="mt-8 rounded-xl border border-gold/50 bg-gold/10 px-4 py-3 text-bone">
            Add your Anthropic API key in{" "}
            <button onClick={() => navigate("/settings")} className="text-gold-2 underline">
              Settings
            </button>{" "}
            so your characters and Dungeon Master can play. Each player's agents run on their own key.
          </div>
        )}

        <NoticeInbox
          inbox={inbox}
          onAnswered={() => {
            void loadNotices();
            void load();
            api.roster().then(setRoster).catch(() => undefined);
          }}
        />

        <Section title="Your adventures">
          {error && <p className="rounded border border-blood/50 bg-blood/10 px-3 py-2 text-[#f0a49b]">{error}</p>}
          {games === null && !error && <p className="text-mist italic">Unrolling the scrolls…</p>}
          {games?.length === 0 && roster && <FirstAdventure roster={roster} onError={setError} />}
          <div className="grid gap-4 sm:grid-cols-2">
            {games?.map((g) => <AdventureCard key={g.id} g={g} onDelete={() => void remove(g)} />)}
          </div>
        </Section>

        <Section
          title="Your heroes"
          action={
            <button onClick={() => navigate("/characters/new")} className="text-[15px] text-gold hover:text-gold-2">
              + Create a hero
            </button>
          }
        >
          {roster?.length === 0 && (
            <p className="text-mist italic">
              Your roster is empty.{" "}
              <button onClick={() => navigate("/characters/new")} className="not-italic text-gold-2 underline">
                Create your first hero
              </button>{" "}
              to begin.
            </p>
          )}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {roster
              ?.filter((c) => c.status !== "fallen")
              .map((c) => (
                <HeroCard key={c.id} ch={c} onClick={() => navigate(`/characters/${c.id}`)} />
              ))}
          </div>
          {fallen.length > 0 && (
            <details className="mt-6 group">
              <summary className="cursor-pointer select-none font-display text-sm tracking-[0.25em] uppercase text-faint hover:text-mist">
                Hall of the Fallen ({fallen.length})
              </summary>
              <p className="mt-2 text-[14px] italic text-faint">Remembered here with their chronicles. Open one to retire them for good.</p>
              <div className="mt-3 grid grid-cols-2 gap-3 opacity-80 sm:grid-cols-4">
                {fallen.map((c) => (
                  <HeroCard key={c.id} ch={c} onClick={() => navigate(`/characters/${c.id}`)} />
                ))}
              </div>
            </details>
          )}
        </Section>

        <p className="mt-16 text-center text-[13px] text-faint">
          Rules content from the System Reference Document 5.1 by Wizards of the Coast LLC, licensed CC BY 4.0.
          <br />
          {/* AGPL-3.0 §13: a modified public server must offer its source. Forks: point this at yours. */}
          Hearthbound is free software (AGPL-3.0).{" "}
          <a href={SOURCE_URL} target="_blank" rel="noreferrer" className="underline hover:text-mist">
            Source code
          </a>
        </p>
      </main>
    </div>
  );
}

/** The front page's banner: the art pack's tavern hearth, firelight and rising embers. */
function Hearth({ backdrop, children }: { backdrop?: string; children: React.ReactNode }) {
  const embers = useMemo(
    () =>
      Array.from({ length: 22 }, (_, i) => ({
        left: `${(i * 37) % 100}%`,
        delay: `${(i * 1.3) % 9}s`,
        duration: `${7 + ((i * 7) % 6)}s`,
        drift: `${((i * 23) % 80) - 40}px`,
        size: 2 + (i % 3),
      })),
    [],
  );
  return (
    <header className="relative h-[68vh] min-h-[440px] overflow-hidden">
      {backdrop ? (
        <img src={backdrop} alt="" className="slow-zoom absolute inset-0 h-full w-full object-cover" />
      ) : (
        <SceneArt sceneKey="tavern" className="absolute inset-0 h-full w-full" />
      )}
      {/* Warm firelight, a vignette, and a fade into the page below. */}
      <div className="firelight absolute inset-0 bg-[radial-gradient(ellipse_70%_60%_at_50%_85%,rgba(255,140,50,.28),transparent_70%)] mix-blend-screen" />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_80%_70%_at_50%_45%,transparent_30%,rgba(14,12,10,.85)_100%)]" />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_55%_40%_at_50%_62%,rgba(14,12,10,.7),transparent)]" />
      <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-b from-transparent to-ink" />
      <div className="pointer-events-none absolute inset-0" aria-hidden>
        {embers.map((e, i) => (
          <span
            key={i}
            className="ember"
            style={{ left: e.left, animationDelay: e.delay, animationDuration: e.duration, width: e.size, height: e.size, ["--drift" as string]: e.drift }}
          />
        ))}
      </div>
      <div className="relative z-10 flex h-full flex-col items-center justify-end px-4 pb-16 text-center">{children}</div>
    </header>
  );
}

/**
 * A new player's way in: one click rolls a hero and two companions (or uses
 * heroes already on the roster) and starts a guided first adventure.
 */
function FirstAdventure({ roster, onError }: { roster: Character[]; onError: (msg: string) => void }) {
  const [busy, setBusy] = useState(false);
  const start = async () => {
    setBusy(true);
    try {
      const idle = roster.filter((c) => c.status === "idle");
      const hero = idle[0] ?? (await api.randomCharacter([]));
      const party = [hero, ...idle.slice(1, 3)];
      while (party.length < 3) {
        party.push(await api.randomCharacter(party.map((c) => c.sheet.class ?? "")));
      }
      const { gameId } = await api.create({
        title: "Your First Adventure",
        dmStyle: "heroic",
        pace: "off",
        heroId: hero.id,
        companionIds: party.slice(1).map((c) => c.id),
        tutorial: true,
      });
      navigate(`/play/${gameId}`);
    } catch (e) {
      onError((e as Error).message);
      setBusy(false);
    }
  };
  return (
    <div className="flex flex-col gap-5 rounded-xl bg-gold/10 p-5 ring-1 ring-gold/50 sm:flex-row sm:items-center">
      <div className="flex-1">
        <h3 className="font-display text-xl text-gold-2">New to Hearthbound?</h3>
        <p className="mt-1 text-[16px] leading-snug text-bone/90">
          One click and you're at the table. We'll roll you a hero and two companions, and the Dungeon Master will guide you through a
          short first tale: a conversation, a test of skill, a small fight and a reward. About fifteen minutes, and nobody needs to know the
          rules.
        </p>
        <button onClick={() => navigate("/new")} className="mt-2 text-[14px] text-mist underline hover:text-bone">
          Or gather your own party
        </button>
      </div>
      <button
        onClick={() => void start()}
        disabled={busy}
        className="shrink-0 rounded-lg bg-gold px-6 py-3 font-display text-lg tracking-wider text-ink shadow-[0_8px_30px_rgba(201,162,74,.35)] transition hover:-translate-y-0.5 hover:bg-gold-2 disabled:opacity-60"
      >
        {busy ? "Rolling your party…" : "Start your first adventure"}
      </button>
    </div>
  );
}

/** A gilt rule with a diamond, under the title. */
function Flourish() {
  return (
    <div className="mt-3 flex items-center gap-3 text-gold/70" aria-hidden>
      <span className="h-px w-16 bg-gradient-to-r from-transparent to-gold/70 sm:w-24" />
      <span className="h-2 w-2 rotate-45 border border-gold/80" />
      <span className="h-px w-16 bg-gradient-to-l from-transparent to-gold/70 sm:w-24" />
    </div>
  );
}

function Section({ title, action, children }: { title: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="mt-12">
      <div className="mb-4 flex items-center gap-4">
        <h2 className="shrink-0 font-display text-sm tracking-[0.25em] uppercase text-gold">{title}</h2>
        <span className="h-px flex-1 bg-gradient-to-r from-gold/40 to-transparent" />
        {action}
      </div>
      {children}
    </section>
  );
}

/** An adventure as a card: its current scene's painting, title, party and state. */
function AdventureCard({ g, onDelete }: { g: SummaryView; onDelete: () => void }) {
  const state = g.mode === "over" ? "Concluded" : g.mode === "combat" ? "In battle" : g.scene || "Setting out";
  return (
    <div className={`group relative overflow-hidden rounded-xl bg-ink-2 gilded transition hover:-translate-y-0.5 ${g.yourTurn ? "ring-1 ring-gold/70" : ""}`}>
      <button onClick={() => navigate(`/play/${g.id}`)} className="block w-full text-left">
        <div className="relative h-36 overflow-hidden">
          {g.art ? (
            <img src={g.art} alt="" className={`h-full w-full object-cover transition duration-700 group-hover:scale-105 ${g.mode === "over" ? "grayscale-[60%]" : ""}`} />
          ) : (
            <SceneArt sceneKey={g.sceneKey || "road"} className="h-full w-full" />
          )}
          <div className="absolute inset-0 bg-gradient-to-b from-ink/10 via-transparent to-ink-2" />
          <div className="absolute left-3 top-3 flex gap-1.5">
            {g.yourTurn && (
              <span className="animate-pulse rounded-full bg-gold px-2 py-0.5 font-display text-[11px] tracking-wider text-ink">Your turn</span>
            )}
            {g.mode === "combat" && <span className="rounded-full bg-blood/80 px-2 py-0.5 font-display text-[11px] tracking-wider text-bone">Combat</span>}
          </div>
        </div>
        <div className="-mt-6 relative px-4 pb-3">
          <div className="font-display text-xl text-bone drop-shadow-[0_2px_6px_rgba(0,0,0,.9)] truncate">{g.title}</div>
          <div className="text-[15px] text-mist truncate">{g.party}</div>
          <div className="mt-1 text-[13px] text-faint truncate">
            {state} · {new Date(g.updatedAt).toLocaleDateString(undefined, { day: "numeric", month: "short" })}
            {g.yourTurn && untilText(g.deadline) ? ` · your agent acts ${untilText(g.deadline)}` : ""}
          </div>
        </div>
      </button>
      <button
        onClick={onDelete}
        className="absolute right-2 top-2 rounded bg-ink/70 px-2 py-0.5 text-[13px] text-faint opacity-0 transition hover:text-[#f0a49b] group-hover:opacity-100 focus:opacity-100"
        aria-label={`Delete ${g.title}`}
      >
        Delete
      </button>
    </div>
  );
}
