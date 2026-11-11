import { Tavern } from "./pages/Tavern";
import { useCallback, useEffect, useState } from "react";
import type { Me } from "./api";
import { api, SignedOut } from "./api";
import { CharacterNew } from "./pages/CharacterNew";
import { CharacterPage } from "./pages/CharacterPage";
import { Home } from "./pages/Home";
import { Login } from "./pages/Login";
import { Play } from "./pages/Play";
import { Settings } from "./pages/Settings";
import { Setup } from "./pages/Setup";
import { Market } from "./pages/Market";
import { navigate, usePath } from "./router";

export function App() {
  const path = usePath();
  const [me, setMe] = useState<Me | null | "signed-out">(null);

  const loadMe = useCallback(() => {
    api
      .me()
      .then(setMe)
      .catch((e) => setMe(e instanceof SignedOut ? "signed-out" : null));
  }, []);

  useEffect(() => {
    loadMe();
    const out = () => setMe("signed-out");
    window.addEventListener("tabletop:signed-out", out);
    return () => window.removeEventListener("tabletop:signed-out", out);
  }, [loadMe]);

  const signedIn = me !== null && me !== "signed-out";
  useEffect(() => {
    if (signedIn && path === "/login") navigate("/");
  }, [signedIn, path]);

  if (me === "signed-out" || path === "/register") return <Login onSignedIn={loadMe} />;
  if (me === null) return <div className="grid h-full place-items-center text-mist italic">Opening the gates…</div>;

  const demoBadge = me.demo && (
    <div className="pointer-events-none fixed left-1/2 top-2 z-50 -translate-x-1/2 rounded-full bg-ember/90 px-3 py-0.5 text-[12px] font-display tracking-widest text-ink shadow">
      DEMO · scripted DM, separate save file
    </div>
  );
  const page = route(path, me, loadMe, () => api.logout().finally(() => setMe("signed-out")));
  return (
    <>
      {demoBadge}
      {page}
    </>
  );
}

function route(path: string, me: Me, loadMe: () => void, signOut: () => void) {
  const play = path.match(/^\/play\/([\w-]+)/);
  if (play) return <Play key={play[1]} id={play[1]} />;
  if (path === "/new") return <Setup />;
  if (path === "/characters/new") return <CharacterNew />;
  const character = path.match(/^\/characters\/([\w-]+)/);
  if (character) return <CharacterPage key={character[1]} id={character[1]} />;
  if (path === "/settings") return <Settings me={me} reload={loadMe} />;
  if (path === "/market") return <Market />;
  if (path === "/tavern") return <Tavern />;
  return <Home me={me} onSignOut={signOut} />;
}
