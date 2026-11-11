import { useState } from "react";
import type { Call, Notice } from "../api";
import { api } from "../api";
import { navigate } from "../router";

/**
 * The leader's open calls at the table: heroes offered in answer to the
 * party's post (welcome or decline), and heroes the leader asked for (withdraw).
 */
export function TableCalls({ calls, onAnswer }: { calls: Call[]; onAnswer: (characterId: string, yes: boolean) => Promise<void> }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  if (calls.length === 0) return null;
  const answer = async (c: Call, yes: boolean) => {
    setBusy(c.characterId);
    setError(null);
    try {
      await onAnswer(c.characterId, yes);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };
  return (
    <div className="mb-3 space-y-1.5" aria-label="Heroes waiting on an answer">
      {calls.map((c) => (
        <div key={c.characterId} className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-ink/70 px-3 py-2 text-[14px] ring-1 ring-gold/30">
          <span className="min-w-0 flex-1 text-mist">
            {c.kind === "offer" ? (
              <>
                📜 <span className="text-bone">{c.name}</span>, {c.summary}, answered your post and would like to join.
              </>
            ) : (
              <>
                ✉️ You asked for <span className="text-bone">{c.name}</span> ({c.summary}). Waiting for their owner.
              </>
            )}
          </span>
          {c.kind === "offer" && (
            <button
              disabled={busy !== null}
              onClick={() => void answer(c, true)}
              className="rounded-md bg-gold px-3 py-1 font-display text-[13px] text-ink hover:bg-gold-2 disabled:opacity-50"
            >
              Welcome {c.name}
            </button>
          )}
          <button disabled={busy !== null} onClick={() => void answer(c, false)} className="text-[13px] text-mist hover:text-[#f0a49b] disabled:opacity-50">
            {c.kind === "offer" ? "No thanks" : "Withdraw"}
          </button>
        </div>
      ))}
      {error && <p className="text-[13px] text-[#f0a49b]">{error}</p>}
    </div>
  );
}

/** The leader's post on the tavern's notice board, from the recruit dialog. */
export function WantedPostForm({ current, onPost }: { current: string; onPost: (text: string) => Promise<void> }) {
  const [text, setText] = useState(current);
  const [msg, setMsg] = useState<string | null>(null);
  const post = async (value: string) => {
    setMsg(null);
    try {
      await onPost(value);
      setText(value);
      setMsg(value ? "Posted. Other players can offer their heroes from the Tavern." : "Taken down.");
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <form
      className="mt-6"
      onSubmit={(e) => {
        e.preventDefault();
        void post(text.trim());
      }}
    >
      <h3 className="font-display text-[13px] tracking-[0.2em] uppercase text-gold">Post on the notice board</h3>
      <p className="mt-1 text-[13px] text-faint">Tell other players who you're looking for. They offer a hero; you decide.</p>
      <div className="mt-2 flex gap-2">
        <input
          value={text}
          maxLength={200}
          onChange={(e) => setText(e.target.value)}
          placeholder="e.g. Looking for a healer before we brave the crypt"
          className="min-w-0 flex-1 rounded-lg bg-ink px-3 py-1.5 text-[14px] text-bone outline-none ring-1 ring-line focus:ring-gold"
        />
        <button disabled={!text.trim() || text.trim() === current} className="rounded-lg bg-gold px-3 font-display text-[13px] text-ink disabled:opacity-40">
          {current ? "Update" : "Post"}
        </button>
        {current && (
          <button type="button" onClick={() => void post("")} className="text-[13px] text-mist hover:text-[#f0a49b]">
            Take down
          </button>
        )}
      </div>
      {msg && <p className="mt-1 text-[13px] text-mist">{msg}</p>}
    </form>
  );
}

/**
 * The front page's notices: owners answer requests for their heroes here;
 * leaders are pointed to the game to answer offers.
 */
export function NoticeInbox({ inbox, onAnswered }: { inbox: Notice[]; onAnswered: () => void }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  if (inbox.length === 0) return null;
  const answer = async (n: Notice, yes: boolean) => {
    setBusy(n.gameId + n.call.characterId);
    setError(null);
    try {
      await api.answerCall(n.gameId, n.call.characterId, yes);
      onAnswered();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };
  return (
    <div className="mt-8 space-y-2" aria-label="Notices">
      {inbox.map((n) => (
        <div key={n.gameId + n.call.characterId} className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-gold/50 bg-gold/10 px-4 py-3">
          {n.call.kind === "ask" ? (
            <>
              <p className="min-w-0 flex-1 text-bone">
                ✉️ <span className="text-gold-2">{n.from}</span> asks if <span className="text-gold-2">{n.call.name}</span> may join their party in{" "}
                <i>{n.title}</i>. {n.call.name}'s agent would play on your key, and you can take over any time.
              </p>
              <button
                disabled={busy !== null}
                onClick={() => void answer(n, true)}
                className="rounded-lg bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-50"
              >
                Send {n.call.name}
              </button>
              <button disabled={busy !== null} onClick={() => void answer(n, false)} className="text-[14px] text-mist hover:text-[#f0a49b]">
                Decline
              </button>
            </>
          ) : (
            <>
              <p className="min-w-0 flex-1 text-bone">
                📜 <span className="text-gold-2">{n.from}</span> offers <span className="text-gold-2">{n.call.name}</span> ({n.call.summary}) for{" "}
                <i>{n.title}</i>.
              </p>
              <button onClick={() => navigate(`/play/${n.gameId}`)} className="rounded-lg bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2">
                Answer at the table
              </button>
            </>
          )}
        </div>
      ))}
      {error && <p className="text-[14px] text-[#f0a49b]">{error}</p>}
    </div>
  );
}
