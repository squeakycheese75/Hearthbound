import { useEffect, useState } from "react";
import type { Invite, KeyInfo, Me, RealmPlayer } from "../api";
import { api, modelName } from "../api";
import { EconomyPanel } from "../components/EconomyPanel";
import { navigate } from "../router";

const keyCopy: Record<KeyInfo["kind"], { name: string; help: string; placeholder: string }> = {
  anthropic: {
    name: "Anthropic (Claude)",
    help: "Pays for your characters' turns and for the Dungeon Master in adventures you lead.",
    placeholder: "sk-ant-…",
  },
  openai: {
    name: "OpenAI (images, optional)",
    help: "Pays for your characters' portraits and, in adventures you lead, monster and scene paintings.",
    placeholder: "sk-…",
  },
};

function KeyRow({ info, onSaved }: { info: KeyInfo; onSaved: () => void }) {
  const [value, setValue] = useState("");
  const [msg, setMsg] = useState<string | null>(null);
  const copy = keyCopy[info.kind];
  const save = async (key: string) => {
    setMsg(null);
    try {
      await api.setKey(info.kind, key);
      setValue("");
      setMsg(key ? "Saved." : "Removed.");
      onSaved();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <div className="rounded-xl bg-ink-2/80 gilded p-4">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="font-display text-bone">{copy.name}</h3>
        <span className={`text-[14px] ${info.set ? "text-verdigris" : "text-faint"}`}>{info.set ? `On file ••••${info.last4}` : "Not set"}</span>
      </div>
      <p className="mt-1 text-[15px] text-mist">{copy.help}</p>
      <form
        className="mt-3 flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void save(value);
        }}
      >
        <input
          type="password"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={info.set ? "Paste a new key to replace it" : copy.placeholder}
          autoComplete="off"
          className="min-w-0 flex-1 rounded-lg bg-ink px-3 py-2 text-bone outline-none focus:ring-1 focus:ring-gold"
        />
        <button disabled={!value.trim()} className="rounded-lg bg-gold px-4 font-display text-ink disabled:opacity-40">
          Save
        </button>
        {info.set && (
          <button type="button" onClick={() => save("")} className="rounded-lg px-3 text-mist ring-1 ring-line hover:text-[#f0a49b]">
            Remove
          </button>
        )}
      </form>
      {msg && <p className="mt-2 text-[14px] text-mist">{msg}</p>}
    </div>
  );
}

function Invites() {
  const [list, setList] = useState<Invite[]>([]);
  const [copied, setCopied] = useState<string | null>(null);
  const load = () => api.invites().then(setList).catch(() => {});
  useEffect(() => {
    void load();
  }, []);
  const link = (code: string) => `${window.location.origin}/register?invite=${encodeURIComponent(code)}`;
  const [msg, setMsg] = useState<string | null>(null);
  const make = async (sponsored: boolean) => {
    setMsg(null);
    try {
      const inv = await api.createInvite(sponsored);
      await navigator.clipboard?.writeText(link(inv.code)).catch(() => {});
      setCopied(inv.code);
      void load();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <section className="mt-10">
      <h2 className="font-display text-sm tracking-[0.25em] uppercase text-gold">Invite friends</h2>
      <p className="mt-2 text-mist">
        Each link lets one person create an account. They can bring their own API key, or you can pay for their play on yours, up to{" "}
        {(500_000).toLocaleString()} tokens a day each (change it below).
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        <button onClick={() => make(true)} className="rounded-lg bg-gold px-4 py-2 font-display text-ink hover:bg-gold-2">
          Invite: I'll pay for their AI
        </button>
        <button onClick={() => make(false)} className="rounded-lg px-4 py-2 font-display text-gold ring-1 ring-gold/50 hover:text-gold-2">
          Invite: they bring a key
        </button>
      </div>
      {msg && <p className="mt-2 text-[14px] text-[#f0a49b]">{msg}</p>}
      <ul className="mt-4 space-y-2">
        {list.map((inv) => (
          <li key={inv.code} className="flex items-center gap-3 rounded-lg bg-ink-2/70 px-3 py-2 text-[14px]">
            <code className="min-w-0 flex-1 truncate text-mist">{link(inv.code)}</code>
            {inv.sponsored && <span className="text-[13px] text-verdigris">you pay</span>}
            {inv.usedBy ? (
              <span className="text-faint">Used</span>
            ) : (
              <button
                onClick={() => navigator.clipboard?.writeText(link(inv.code)).then(() => setCopied(inv.code))}
                className="text-gold hover:text-gold-2"
              >
                {copied === inv.code ? "Copied" : "Copy"}
              </button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

function Players({ me }: { me: Me }) {
  const [list, setList] = useState<RealmPlayer[]>([]);
  const [msg, setMsg] = useState<string | null>(null);
  const load = () => api.players().then(setList).catch(() => {});
  useEffect(() => {
    void load();
  }, []);
  const set = async (p: RealmPlayer, tokenCap: number) => {
    setMsg(null);
    try {
      await api.sponsor(p.id, tokenCap);
      void load();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  const others = list.filter((p) => p.id !== me.account.id);
  if (others.length === 0) return null;
  return (
    <section className="mt-10">
      <h2 className="font-display text-sm tracking-[0.25em] uppercase text-gold">Players in the realm</h2>
      <p className="mt-2 text-[15px] text-mist">
        Pay for a friend's AI on your Anthropic key, so they can play without one. A key of their own always comes first.
      </p>
      <ul className="mt-3 divide-y divide-line/50 rounded-xl bg-ink-2/80 gilded">
        {others.map((p) => (
          <PlayerRow key={p.id} p={p} onSet={(c) => set(p, c)} />
        ))}
      </ul>
      {msg && <p className="mt-2 text-[14px] text-[#f0a49b]">{msg}</p>}
    </section>
  );
}

function PlayerRow({ p, onSet }: { p: RealmPlayer; onSet: (tokenCap: number) => void }) {
  const [cap, setCap] = useState(String(p.sponsorCap || 500_000));
  useEffect(() => setCap(String(p.sponsorCap || 500_000)), [p.sponsorCap]);
  return (
    <li className="flex flex-wrap items-center gap-3 px-4 py-3">
      <div className="min-w-0 flex-1">
        <span className="text-bone">{p.username}</span>
        {p.admin && <span className="ml-2 text-[13px] text-faint">admin</span>}
        <p className="text-[13px] text-faint">
          {p.ownKey ? "Has their own key" : p.sponsored ? "You pay for their AI" : "No key yet: can't play"}
          {" · "}
          {p.usedToday.toLocaleString()} tokens today
        </p>
      </div>
      {p.sponsored ? (
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            onSet(Number(cap));
          }}
        >
          <input
            aria-label={`Tokens a day you pay for ${p.username}`}
            inputMode="numeric"
            value={cap}
            onChange={(e) => setCap(e.target.value.replace(/\D/g, ""))}
            className="w-32 rounded-lg bg-ink px-2 py-1 text-[14px] text-bone tabular-nums outline-none focus:ring-1 focus:ring-gold"
          />
          <button className="text-[14px] text-gold hover:text-gold-2">Save</button>
          <button type="button" onClick={() => onSet(0)} className="text-[14px] text-mist hover:text-[#f0a49b]">
            Stop paying
          </button>
        </form>
      ) : (
        <button onClick={() => onSet(500_000)} className="rounded-lg px-3 py-1 text-[14px] text-gold ring-1 ring-gold/50 hover:text-gold-2">
          Pay for their AI
        </button>
      )}
    </li>
  );
}

const roleLabels: Record<string, string> = {
  dm: "Dungeon Master",
  character: "Characters",
  choices: "Quick-choice helper",
  cast: "NPC check",
  recap: "Chapter recaps",
};

function AgentSettings({ me, reload }: { me: Me; reload: () => void }) {
  const [msg, setMsg] = useState<string | null>(null);
  const tableDefault = me.roles.find((r) => r.name === "character")?.model ?? "";
  const choose = async (model: string) => {
    setMsg(null);
    try {
      await api.setAgentModel(model);
      setMsg("Saved. Your characters use it from their next turn.");
      reload();
    } catch (err) {
      setMsg((err as Error).message);
    }
  };
  return (
    <>
      <h2 className="mt-10 font-display text-sm tracking-[0.25em] uppercase text-gold">Your characters' AI</h2>
      <p className="mt-2 text-[15px] text-mist">
        The model your characters' agents run on, whoever's table they sit at. It runs on your key: when they play for you, or cover while you're away.
      </p>
      <div className="mt-3 rounded-xl bg-ink-2/80 gilded p-2" role="radiogroup" aria-label="Model for your characters">
        {[{ id: "", name: `Table default (${modelName(tableDefault)})`, provider: "", note: "whatever the server's character role uses" }, ...me.models].map((m) => {
          const on = (me.account.agentModel ?? "") === m.id;
          return (
            <button
              key={m.id || "default"}
              role="radio"
              aria-checked={on}
              onClick={() => !on && void choose(m.id)}
              className={`flex w-full items-baseline justify-between gap-3 rounded-lg px-3 py-2 text-left ${on ? "bg-ink-3 ring-1 ring-gold/60" : "hover:bg-ink-3/60"}`}
            >
              <span className={on ? "text-gold-2" : "text-bone"}>
                {on ? "● " : "○ "}
                {m.name}
              </span>
              <span className="text-[13px] text-faint">{m.note}</span>
            </button>
          );
        })}
      </div>
      {msg && <p className="mt-2 text-[14px] text-mist">{msg}</p>}

      <h2 className="mt-10 font-display text-sm tracking-[0.25em] uppercase text-gold">How the table's agents are set up</h2>
      <p className="mt-2 text-[15px] text-mist">Each role is defined in its own file on the server. Tables you lead run the DM and helpers on your key.</p>
      <ul className="mt-3 divide-y divide-line/50 rounded-xl bg-ink-2/80 gilded">
        {me.roles.map((r) => (
          <li key={r.name} className="px-4 py-3">
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-bone">{roleLabels[r.name] ?? r.name}</span>
              <span className="text-[13px] tabular-nums text-mist">
                {modelName(r.model)} · {r.effort || "default"} effort
              </span>
            </div>
            <p className="mt-0.5 text-[14px] text-faint">{r.description}</p>
          </li>
        ))}
      </ul>
    </>
  );
}

export function Settings({ me, reload }: { me: Me; reload: () => void }) {
  const [cap, setCap] = useState(String(me.account.dailyTokenCap));
  const [capMsg, setCapMsg] = useState<string | null>(null);
  const pct = Math.min(100, (me.spend.used / Math.max(1, me.spend.cap)) * 100);
  return (
    <div className="mx-auto max-w-2xl px-4 py-10">
      <button onClick={() => navigate("/")} className="text-mist hover:text-bone">
        ← Back
      </button>
      <h1 className="mt-4 font-display text-3xl text-gold-2">Settings</h1>
      <p className="text-mist">Signed in as {me.account.username}{me.account.admin ? " (admin)" : ""}.</p>
      {me.local && (
        <p className="mt-3 rounded-lg border border-line px-3 py-2 text-[15px] text-mist">
          Local mode: keys you haven't set here fall back to ANTHROPIC_API_KEY / OPENAI_API_KEY from the server's environment.
        </p>
      )}

      {me.spend.sponsorCap ? (
        <p className="mt-6 rounded-lg border border-verdigris/40 px-3 py-2 text-[15px] text-mist">
          The realm pays for your AI: up to {me.spend.sponsorCap.toLocaleString()} tokens a day. You don't need a key to play. Add your own
          Anthropic key below if you'd like to play more; it's used first.
        </p>
      ) : null}

      <h2 className="mt-8 font-display text-sm tracking-[0.25em] uppercase text-gold">Your API keys</h2>
      <p className="mt-2 text-[15px] text-mist">Keys are encrypted on the server and never shown again. Only the last four characters are displayed.</p>
      <div className="mt-3 space-y-3">
        {me.keys.map((k) => (
          <KeyRow key={k.kind} info={k} onSaved={reload} />
        ))}
      </div>

      <h2 className="mt-10 font-display text-sm tracking-[0.25em] uppercase text-gold">Daily budget</h2>
      <div className="mt-3 rounded-xl bg-ink-2/80 gilded p-4">
        <div className="flex justify-between text-[15px] text-mist">
          <span>Today</span>
          <span className="tabular-nums">
            {me.spend.used.toLocaleString()} / {me.spend.cap.toLocaleString()} tokens
          </span>
        </div>
        <div className="mt-2 h-2 rounded-full bg-black/50">
          <div className="h-full rounded-full" style={{ width: `${pct}%`, background: pct > 85 ? "#c4483f" : "#c9a24a" }} />
        </div>
        <form
          className="mt-4 flex items-center gap-2"
          onSubmit={async (e) => {
            e.preventDefault();
            try {
              await api.setCap(Number(cap));
              setCapMsg("Saved.");
              reload();
            } catch (err) {
              setCapMsg((err as Error).message);
            }
          }}
        >
          <label className="text-[15px] text-mist" htmlFor="cap">
            Cap (tokens/day)
          </label>
          <input id="cap" inputMode="numeric" value={cap} onChange={(e) => setCap(e.target.value.replace(/\D/g, ""))} className="w-40 rounded-lg bg-ink px-3 py-2 text-bone tabular-nums outline-none focus:ring-1 focus:ring-gold" />
          <button className="rounded-lg bg-gold px-4 py-2 font-display text-ink">Save</button>
          {capMsg && <span className="text-[14px] text-mist">{capMsg}</span>}
        </form>
        <p className="mt-2 text-[14px] text-faint">When you hit the cap, your characters' agents pause until tomorrow (UTC). Tokens are weighted the way they are billed: re-reading the cached story counts a tenth.</p>
      </div>

      <AgentSettings me={me} reload={reload} />

      {me.account.admin && (
        <>
          <h2 className="mt-10 font-display text-sm tracking-[0.25em] uppercase text-gold">Realm economy</h2>
          <p className="mt-2 text-[15px] text-mist">Every coin entering or leaving the realm is recorded. You set how much loot comes in and what the market and shop take out.</p>
          <EconomyPanel />
        </>
      )}

      {me.account.admin && !me.local && <Invites />}
      {me.account.admin && !me.local && <Players me={me} />}
    </div>
  );
}
