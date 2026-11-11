import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useState } from "react";
import type { Character, TavernHero } from "../api";
import { api } from "../api";
import { navigate } from "../router";
import { HeroCard } from "./HeroCard";
import { WantedPostForm } from "./Calls";
import { SheetDetails } from "./Party";

/** The scene banner's recruit button, shown while the party is in a tavern, town or village. */
export function RecruitButton({ full, onOpen }: { full: boolean; onOpen: () => void }) {
  return (
    <button
      onClick={onOpen}
      disabled={full}
      title={full ? "The party is full" : "See who's at the tavern looking for adventure"}
      className="absolute right-3 top-3 z-10 rounded-full bg-ink/80 px-3.5 py-1.5 font-display text-[13px] tracking-wider text-gold-2 ring-1 ring-gold/50 backdrop-blur enabled:hover:bg-ink enabled:hover:ring-gold disabled:cursor-not-allowed disabled:text-faint disabled:ring-line"
    >
      + Recruit from the tavern
    </button>
  );
}

type Pick = { ch: Character; owner?: string };

/**
 * The tavern: your heroes in town, other players' heroes looking for
 * adventure, or someone new. Pick anyone to inspect their sheet, then recruit.
 */
export function RecruitDialog({
  open,
  partyIds,
  partyClasses,
  asked,
  wanted,
  onClose,
  onRecruit,
  onAsk,
  onPost,
}: {
  open: boolean;
  partyIds: string[];
  partyClasses: string[];
  /** Heroes the leader has already asked for. */
  asked: string[];
  wanted: string;
  onClose: () => void;
  onRecruit: (characterId: string) => Promise<void>;
  onAsk: (characterId: string) => Promise<void>;
  onPost: (text: string) => Promise<void>;
}) {
  const [mine, setMine] = useState<Character[] | null>(null);
  const [others, setOthers] = useState<TavernHero[]>([]);
  const [picked, setPicked] = useState<Pick | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"recruit" | "roll" | null>(null);

  const load = useCallback(async () => {
    const [roster, tavern] = await Promise.all([api.roster(), api.tavern()]);
    setMine(roster.filter((ch) => ch.status === "idle" && !ch.sheet.dead && !partyIds.includes(ch.id)));
    setOthers(tavern.forHire.filter((h) => !partyIds.includes(h.id)));
  }, [partyIds]);

  useEffect(() => {
    if (!open) return;
    setError(null);
    setPicked(null);
    load().catch((e: Error) => setError(e.message));
  }, [open, load]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const recruit = async (id: string, ask: boolean) => {
    setBusy("recruit");
    setError(null);
    try {
      await (ask ? onAsk(id) : onRecruit(id));
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };
  const roll = async () => {
    setBusy("roll");
    setError(null);
    try {
      const ch = await api.randomCharacter(partyClasses);
      await load();
      setPicked({ ch });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const asking = picked?.owner !== undefined && picked.ch.hire === "ask";
  const alreadyAsked = !!picked && asked.includes(picked.ch.id);
  return (
    <AnimatePresence>
      {open && (
        <motion.div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
          <motion.section
            role="dialog"
            aria-label="Recruit from the tavern"
            className="grid max-h-full w-full max-w-6xl grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-xl bg-ink-2 gilded"
            initial={{ y: 16 }}
            animate={{ y: 0 }}
            exit={{ y: 16 }}
            onClick={(e) => e.stopPropagation()}
          >
            <header className="flex items-start gap-4 border-b border-line/60 p-5">
              <div>
                <h2 className="font-display text-xl text-gold-2">Recruit from the tavern</h2>
                <p className="mt-1 text-[14px] text-mist">Pick someone to see their sheet. Whoever joins is played by their agent; the DM introduces them next turn.</p>
              </div>
              <button onClick={onClose} className="ml-auto rounded px-2 text-2xl text-mist hover:text-bone" aria-label="Close">
                ×
              </button>
            </header>
            <div className="grid min-h-0 md:grid-cols-[minmax(0,1fr)_380px]">
              <div className="min-h-0 overflow-y-auto scroll-thin p-5">
                {!mine && !error && <p className="italic text-mist">Looking around the tavern…</p>}
                {mine && (
                  <>
                    <Section title="Your heroes in town" empty="None of your heroes are in town.">
                      {mine.map((ch) => (
                        <HeroCard key={ch.id} ch={ch} selected={picked?.ch.id === ch.id} onClick={() => setPicked({ ch })} />
                      ))}
                    </Section>
                    <Section title="Looking for a party" empty="No other players' heroes are looking for adventure right now.">
                      {others.map((h) => (
                        <HeroCard
                          key={h.id}
                          ch={h}
                          selected={picked?.ch.id === h.id}
                          badge={h.ownerName ? `${h.ownerName}'s` : undefined}
                          onClick={() => setPicked({ ch: h, owner: h.ownerName })}
                        />
                      ))}
                    </Section>
                    <h3 className="mt-6 font-display text-[13px] tracking-[0.2em] uppercase text-gold">Someone new</h3>
                    <div className="mt-2 flex flex-wrap gap-2">
                      <button
                        onClick={() => void roll()}
                        disabled={!!busy}
                        className="rounded-lg border border-gold/50 px-3 py-1.5 text-[14px] text-gold hover:bg-gold/10 disabled:opacity-50"
                      >
                        {busy === "roll" ? "Rolling…" : "🎲 Roll a new hero"}
                      </button>
                      <button onClick={() => navigate("/characters/new")} className="rounded-lg px-3 py-1.5 text-[14px] text-mist hover:text-bone">
                        Create a hero…
                      </button>
                    </div>
                    <WantedPostForm current={wanted} onPost={onPost} />
                  </>
                )}
              </div>
              <aside className="min-h-0 overflow-y-auto scroll-thin border-t border-line/60 bg-ink/50 p-5 md:border-l md:border-t-0">
                {!picked ? (
                  <p className="italic text-faint">Pick someone to see their sheet.</p>
                ) : (
                  <>
                    <div className="flex items-baseline justify-between gap-2">
                      <h3 className="font-display text-xl text-bone">{picked.ch.sheet.name}</h3>
                      {picked.owner && <span className="text-[13px] text-mist">{picked.owner}'s hero</span>}
                    </div>
                    <p className="text-mist">
                      Level {picked.ch.sheet.level} {picked.ch.sheet.race} {picked.ch.sheet.class}
                    </p>
                    <button
                      onClick={() => void recruit(picked.ch.id, asking)}
                      disabled={!!busy || alreadyAsked}
                      className="mt-3 w-full rounded-lg bg-gold px-4 py-2 font-display tracking-wider text-ink hover:bg-gold-2 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {busy === "recruit"
                        ? asking
                          ? "Asking…"
                          : "Joining…"
                        : alreadyAsked
                          ? `Asked ${picked.owner}; waiting`
                          : asking
                            ? `Ask ${picked.owner} if ${picked.ch.sheet.name} may join`
                            : `Recruit ${picked.ch.sheet.name}`}
                    </button>
                    {picked.owner && (
                      <p className="mt-2 text-[13px] text-faint">
                        {asking ? `${picked.owner} wants to be asked first; they'll answer from their notices. ` : ""}
                        Their agent plays them on {picked.owner}'s key, and {picked.owner} can take over any time.
                      </p>
                    )}
                    <SheetDetails c={picked.ch.sheet} />
                  </>
                )}
                {error && <p className="mt-4 rounded border border-blood/50 bg-blood/10 px-3 py-2 text-[#f0a49b]">{error}</p>}
              </aside>
            </div>
          </motion.section>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

function Section({ title, empty, children }: { title: string; empty: string; children: React.ReactNode[] }) {
  return (
    <>
      <h3 className="mt-2 mb-2 font-display text-[13px] tracking-[0.2em] uppercase text-gold first:mt-0">{title}</h3>
      {children.length === 0 ? (
        <p className="mb-4 text-[14px] italic text-faint">{empty}</p>
      ) : (
        <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-3">{children}</div>
      )}
    </>
  );
}
