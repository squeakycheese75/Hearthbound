import { AnimatePresence, motion } from "motion/react";
import { useEffect, useState } from "react";
import type { Character, SetupOptions } from "../api";
import { api, durationText } from "../api";
import { HeroCard } from "../components/HeroCard";
import { navigate } from "../router";

const steps = ["Your hero", "Companions", "Dungeon Master"];

function NewHeroTile({ label }: { label: string }) {
  return (
    <button
      type="button"
      onClick={() => navigate("/characters/new?return=/new")}
      className="grid aspect-[4/5] place-items-center rounded-xl border border-dashed border-line text-mist transition hover:border-gold hover:text-gold-2"
    >
      <span className="text-center">
        <span className="block font-display text-4xl">+</span>
        <span className="text-[15px]">{label}</span>
      </span>
    </button>
  );
}

function RandomTile({ label, busy, onClick, disabled }: { label: string; busy: boolean; onClick: () => void; disabled?: boolean }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={busy || disabled}
      className="grid aspect-[4/5] place-items-center rounded-xl border border-dashed border-line text-mist transition hover:border-gold hover:text-gold-2 disabled:opacity-40 disabled:hover:border-line disabled:hover:text-mist"
    >
      <span className="text-center">
        <motion.span
          className="block font-display text-4xl"
          animate={busy ? { rotate: 360 } : { rotate: 0 }}
          transition={busy ? { repeat: Infinity, duration: 0.8, ease: "linear" } : { duration: 0 }}
        >
          ⚄
        </motion.span>
        <span className="text-[15px]">{busy ? "Rolling…" : label}</span>
      </span>
    </button>
  );
}

/** Start an adventure: pick a hero and companions from your roster, then a DM. */
export function Setup() {
  const [opts, setOpts] = useState<SetupOptions | null>(null);
  const [roster, setRoster] = useState<Character[] | null>(null);
  const [step, setStep] = useState(0);
  const [hero, setHero] = useState<string | null>(null);
  const [companions, setCompanions] = useState<string[]>([]);
  const [dmStyle, setDmStyle] = useState("heroic");
  const [pace, setPace] = useState("daily");
  const [title, setTitle] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [rolling, setRolling] = useState(false);

  useEffect(() => {
    Promise.all([api.options(), api.roster()])
      .then(([o, r]) => {
        setOpts(o);
        setRoster(r);
        const first = r.find((c) => c.status === "idle");
        if (first) setHero(first.id);
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  if (!opts || !roster) return <div className="grid h-full place-items-center text-mist italic">{error ?? "Gathering the roster…"}</div>;

  const available = roster.filter((c) => c.status === "idle");
  const heroChar = roster.find((c) => c.id === hero);
  const toggle = (id: string) =>
    setCompanions((cs) => (cs.includes(id) ? cs.filter((x) => x !== id) : cs.length < 3 ? [...cs, id] : cs));

  const create = async () => {
    if (!hero) return;
    setCreating(true);
    setError(null);
    try {
      const { gameId } = await api.create({ title, dmStyle, pace, heroId: hero, companionIds: companions });
      navigate(`/play/${gameId}`);
    } catch (e) {
      setError((e as Error).message);
      setCreating(false);
    }
  };

  // roll creates random heroes on the roster, each avoiding classes already in the party.
  const roll = async (count: number, asHero = false) => {
    setRolling(true);
    setError(null);
    const classes = roster.filter((c) => c.id === hero || companions.includes(c.id)).flatMap((c) => (c.sheet.class ? [c.sheet.class] : []));
    try {
      for (let i = 0; i < count; i++) {
        const ch = await api.randomCharacter(asHero ? [] : classes);
        if (ch.sheet.class) classes.push(ch.sheet.class);
        setRoster((r) => [...(r ?? []), ch]);
        if (asHero) {
          setHero(ch.id);
          setCompanions((cs) => cs.filter((x) => x !== ch.id));
        } else {
          setCompanions((cs) => [...cs, ch.id]);
        }
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRolling(false);
    }
  };

  const unavailable = roster.length - available.length;
  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:py-12">
      <button onClick={() => navigate("/")} className="text-mist hover:text-bone">
        ← Back
      </button>
      <h1 className="mt-4 font-display text-3xl sm:text-4xl text-gold-2">Gather your party</h1>
      <ol className="mt-5 flex gap-2 text-[14px]">
        {steps.map((s, i) => (
          <li key={s} className={`flex-1 border-t-2 pt-2 ${i <= step ? "border-gold text-bone" : "border-line text-faint"}`}>
            <span className="font-display tracking-wider">{i + 1}.</span> {s}
          </li>
        ))}
      </ol>

      <AnimatePresence mode="wait">
        <motion.section key={step} initial={{ opacity: 0, x: 16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -16 }} className="mt-8">
          {step === 0 && (
            <>
              <p className="text-mist">Who will you play? Only heroes resting in town can set out.</p>
              <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                {available.map((c) => (
                  <HeroCard
                    key={c.id}
                    ch={c}
                    selected={hero === c.id}
                    onClick={() => {
                      setHero(c.id);
                      setCompanions((cs) => cs.filter((x) => x !== c.id));
                    }}
                    badge={hero === c.id ? "You" : undefined}
                  />
                ))}
                <RandomTile label="Roll a random hero" busy={rolling} onClick={() => roll(1, true)} />
                <NewHeroTile label="Create a hero" />
              </div>
            </>
          )}

          {step === 1 && (
            <>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-mist">
                  Pick up to three companions from your roster. Each is played by its own AI agent, in character. {companions.length}/3 chosen.
                </p>
                {companions.length < 3 && (
                  <button
                    type="button"
                    onClick={() => roll(3 - companions.length)}
                    disabled={rolling}
                    className="rounded-lg px-4 py-2 font-display text-[14px] tracking-wider text-gold-2 ring-1 ring-gold/60 transition hover:bg-gold/15 disabled:opacity-50"
                  >
                    {rolling ? "Rolling…" : companions.length === 0 ? "⚄ Roll a random party" : "⚄ Fill the party at random"}
                  </button>
                )}
              </div>
              <p className="mt-1 text-[14px] text-faint">Random companions get a class the party is missing, and join your roster.</p>
              <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                {available
                  .filter((c) => c.id !== hero)
                  .map((c) => (
                    <HeroCard
                      key={c.id}
                      ch={c}
                      selected={companions.includes(c.id)}
                      onClick={() => toggle(c.id)}
                      disabled={!companions.includes(c.id) && companions.length >= 3}
                      badge={companions.includes(c.id) ? "Agent" : undefined}
                    />
                  ))}
                <RandomTile label="Random companion" busy={rolling} disabled={companions.length >= 3} onClick={() => roll(1)} />
                <NewHeroTile label="Create a companion" />
              </div>
              {unavailable > 0 && <p className="mt-3 text-[14px] text-faint">{unavailable} of your heroes are away or fallen.</p>}
            </>
          )}

          {step === 2 && (
            <div className="space-y-6">
              <div className="grid gap-3 sm:grid-cols-2">
                {opts.dmStyles.map((s) => (
                  <button
                    key={s.key}
                    type="button"
                    onClick={() => setDmStyle(s.key)}
                    aria-pressed={dmStyle === s.key}
                    className={`rounded-xl px-4 py-3 text-left transition-colors ${dmStyle === s.key ? "bg-gold/15 ring-1 ring-gold" : "bg-ink-2/80 hover:bg-ink-3 gilded"}`}
                  >
                    <div className={`font-display ${dmStyle === s.key ? "text-gold-2" : "text-bone"}`}>{s.name}</div>
                    <div className="text-[14px] text-mist">{s.description}</div>
                  </button>
                ))}
              </div>
              <div>
                <span className="font-display text-sm tracking-[0.2em] uppercase text-gold">Pace</span>
                <p className="mt-1 text-[15px] text-mist">
                  How long the table waits for a player before their character's agent acts for them. Pick "always wait" to play strictly yourself.
                </p>
                <div className="mt-3 flex flex-wrap gap-2">
                  {opts.paces.map((p) => (
                    <button
                      key={p.key}
                      type="button"
                      onClick={() => setPace(p.key)}
                      aria-pressed={pace === p.key}
                      className={`rounded-lg px-3 py-2 text-left transition ${pace === p.key ? "bg-gold/15 ring-1 ring-gold" : "bg-ink-2/80 ring-1 ring-line hover:bg-ink-3"}`}
                    >
                      <div className={`font-display text-[14px] ${pace === p.key ? "text-gold-2" : "text-bone"}`}>{p.name}</div>
                      <div className="text-[12px] text-faint">
                        {p.explore ? `${durationText(p.explore)} a round · ${durationText(p.combat)} a combat turn` : "Agents only play when you hand over"}
                      </div>
                    </button>
                  ))}
                </div>
              </div>
              <label className="block">
                <span className="font-display text-sm tracking-[0.2em] uppercase text-gold">Adventure title (optional)</span>
                <input
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder={`The Adventures of ${heroChar?.sheet.name ?? "…"}`}
                  maxLength={80}
                  className="mt-2 w-full rounded-lg bg-ink-2 px-4 py-3 text-lg text-bone placeholder:text-faint gilded outline-none focus:ring-1 focus:ring-gold"
                />
              </label>
            </div>
          )}
        </motion.section>
      </AnimatePresence>

      {error && (
        <p className="mt-6 rounded border border-blood/50 bg-blood/10 px-3 py-2 text-[#f0a49b]">
          {error}
          {error.includes("Anthropic key") && (
            <button onClick={() => navigate("/settings")} className="ml-2 text-gold-2 underline">
              Open Settings
            </button>
          )}
        </p>
      )}

      <div className="mt-10 flex justify-between">
        <button onClick={() => setStep(step - 1)} disabled={step === 0} className="rounded-lg px-4 py-2 text-mist hover:text-bone disabled:invisible">
          Back
        </button>
        {step < 2 ? (
          <button onClick={() => setStep(step + 1)} disabled={!hero} className="rounded-lg bg-gold px-6 py-2.5 font-display tracking-wider text-ink hover:bg-gold-2 disabled:opacity-40">
            Continue
          </button>
        ) : (
          <button onClick={create} disabled={creating || !hero} className="rounded-lg bg-gold px-6 py-2.5 font-display tracking-wider text-ink hover:bg-gold-2 disabled:opacity-60">
            {creating ? "Setting out…" : "Set out"}
          </button>
        )}
      </div>
    </div>
  );
}
