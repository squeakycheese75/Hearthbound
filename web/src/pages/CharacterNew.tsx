import { useEffect, useState } from "react";
import type { NewCharacter, SetupOptions } from "../api";
import { api } from "../api";
import { Portrait } from "../components/Portrait";
import { navigate } from "../router";

function Choice({ selected, onClick, title, sub, children }: { selected: boolean; onClick: () => void; title: string; sub: string; children?: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-left transition-colors ${selected ? "bg-gold/15 ring-1 ring-gold" : "bg-ink-2/80 hover:bg-ink-3 gilded"}`}
    >
      {children}
      <div className="min-w-0">
        <div className={`font-display text-[15px] ${selected ? "text-gold-2" : "text-bone"}`}>{title}</div>
        <div className="text-[14px] leading-snug text-mist">{sub}</div>
      </div>
    </button>
  );
}

const emblem = (cls: string) => ({ id: cls, name: cls, kind: "pc" as const, class: cls, controller: "ai" as const, level: 1, xp: 0, abilities: {}, maxHp: 1, hp: 1, ac: 10, speed: 30, actions: [], deathSaves: { successes: 0, failures: 0 }, pos: { x: 0, y: 0 }, gold: 0 });

/** Create a persistent hero. ?return=/new sends you back to party setup afterwards. */
export function CharacterNew() {
  const back = new URLSearchParams(window.location.search).get("return");
  const [opts, setOpts] = useState<SetupOptions | null>(null);
  const [c, setC] = useState<NewCharacter>({ name: "", class: "Fighter", race: "Human", gender: "", archetype: "", backstory: "" });
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api
      .options()
      .then((o) => {
        setOpts(o);
        setC((c) => ({ ...c, archetype: o.archetypes[0].archetype }));
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  if (!opts) return <div className="grid h-full place-items-center text-mist italic">{error ?? "Sharpening quills…"}</div>;

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      const ch = await api.createCharacter(c);
      navigate(back ?? `/characters/${ch.id}`);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };
  const label = "font-display text-sm tracking-[0.2em] uppercase text-gold";

  return (
    <div className="mx-auto max-w-4xl px-4 py-8 sm:py-12">
      <button onClick={() => navigate(back ?? "/")} className="text-mist hover:text-bone">
        ← Back
      </button>
      <h1 className="mt-4 font-display text-3xl sm:text-4xl text-gold-2">Create a hero</h1>
      <p className="mt-2 text-mist">Heroes live in your roster and keep their XP, gold and gear from one adventure to the next. When you're not playing one, its personality guides how its agent plays it.</p>

      <label className="mt-8 block">
        <span className={label}>Name</span>
        <input
          autoFocus
          value={c.name}
          onChange={(e) => setC({ ...c, name: e.target.value })}
          placeholder="What do the bards call you?"
          maxLength={40}
          className="mt-2 w-full rounded-lg bg-ink-2 px-4 py-3 text-xl text-bone placeholder:text-faint gilded outline-none focus:ring-1 focus:ring-gold"
        />
      </label>

      <div className="mt-8">
        <span className={label}>Gender</span>
        <div className="mt-2 grid max-w-sm grid-cols-2 gap-2">
          {(
            [
              ["m", "Male"],
              ["f", "Female"],
            ] as const
          ).map(([g, name]) => (
            <Choice key={g} selected={c.gender === g} onClick={() => setC({ ...c, gender: g })} title={name} sub="Shapes their portrait and voice." />
          ))}
        </div>
      </div>

      <div className="mt-8">
        <span className={label}>Class</span>
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {opts.classes.map((k) => (
            <Choice key={k.name} selected={c.class === k.name} onClick={() => setC({ ...c, class: k.name })} title={k.name} sub={k.summary}>
              <Portrait c={emblem(k.name)} size={40} />
            </Choice>
          ))}
        </div>
      </div>

      <div className="mt-8">
        <span className={label}>Ancestry</span>
        <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3">
          {opts.races.map((r) => (
            <Choice key={r.name} selected={c.race === r.name} onClick={() => setC({ ...c, race: r.name })} title={r.name} sub={r.summary} />
          ))}
        </div>
      </div>

      <div className="mt-8">
        <span className={label}>Personality</span>
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {opts.archetypes.map((p) => (
            <Choice key={p.archetype} selected={c.archetype === p.archetype} onClick={() => setC({ ...c, archetype: p.archetype })} title={p.archetype} sub={p.personality} />
          ))}
        </div>
      </div>

      <label className="mt-8 block">
        <span className={label}>Backstory (optional)</span>
        <textarea
          value={c.backstory}
          onChange={(e) => setC({ ...c, backstory: e.target.value })}
          maxLength={400}
          rows={3}
          placeholder="A hook the DM can use: a debt, a lost sibling, a secret…"
          className="mt-2 w-full rounded-lg bg-ink-2 px-4 py-3 text-bone placeholder:text-faint gilded outline-none focus:ring-1 focus:ring-gold"
        />
      </label>

      {error && <p className="mt-6 rounded border border-blood/50 bg-blood/10 px-3 py-2 text-[#f0a49b]">{error}</p>}
      <div className="mt-8 flex justify-end">
        <button onClick={save} disabled={saving || !c.name.trim() || !c.gender} className="rounded-lg bg-gold px-6 py-2.5 font-display tracking-wider text-ink hover:bg-gold-2 disabled:opacity-40">
          {saving ? "Recording in the annals…" : "Create hero"}
        </button>
      </div>
    </div>
  );
}
