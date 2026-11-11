import { useState } from "react";
import { api } from "../api";
import { SceneArt } from "../components/SceneArt";
import { navigate } from "../router";

/** Sign in, or register with an invite code (?invite=CODE). */
export function Login({ onSignedIn }: { onSignedIn: () => void }) {
  const params = new URLSearchParams(window.location.search);
  const [invite, setInvite] = useState(params.get("invite") ?? "");
  const [mode, setMode] = useState<"login" | "register">(invite ? "register" : "login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (mode === "login") await api.login(username, password);
      if (mode === "register") await api.register(invite, username, password);
      navigate("/");
      onSignedIn();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const field = "mt-1 w-full rounded-lg bg-ink px-4 py-2.5 text-lg text-bone gilded outline-none focus:ring-1 focus:ring-gold";
  return (
    <div className="relative grid min-h-full place-items-center px-4 py-10">
      <SceneArt sceneKey="castle" className="fixed inset-0 h-full w-full opacity-60" />
      <form onSubmit={submit} className="relative w-full max-w-sm rounded-2xl bg-ink-2/95 gilded p-7 backdrop-blur">
        <h1 className="font-display text-4xl text-gold-2 text-center">Hearthbound</h1>
        <p className="mt-1 mb-6 text-center text-mist">{mode === "login" ? "Welcome back to the realm." : "You've been invited to the realm."}</p>
        {mode === "register" && (
          <label className="block mb-4 text-[15px] text-mist">
            Invite code
            <input value={invite} onChange={(e) => setInvite(e.target.value)} required className={field} autoComplete="off" />
          </label>
        )}
        <label className="block mb-4 text-[15px] text-mist">
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus className={field} autoComplete="username" />
        </label>
        <label className="block text-[15px] text-mist">
          Password {mode === "register" && <span className="text-faint">(10+ characters)</span>}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={mode === "register" ? 10 : undefined}
            className={field}
            autoComplete={mode === "login" ? "current-password" : "new-password"}
          />
        </label>
        {error && (
          <p className="mt-4 rounded border border-blood/50 bg-blood/10 px-3 py-2 text-[15px] text-[#f0a49b]" role="alert">
            {error}
          </p>
        )}
        <button disabled={busy} className="mt-6 w-full rounded-lg bg-gold py-3 font-display tracking-wider text-ink hover:bg-gold-2 disabled:opacity-60">
          {busy ? "One moment…" : mode === "login" ? "Sign in" : "Join the realm"}
        </button>
        <button type="button" onClick={() => setMode(mode === "login" ? "register" : "login")} className="mt-4 w-full text-[15px] text-mist hover:text-bone">
          {mode === "login" ? "Have an invite code? Register" : "Already have an account? Sign in"}
        </button>
      </form>
    </div>
  );
}
