import { createContext, useContext } from "react";

export interface ArtState {
  art: Record<string, string>;
  painting: string[];
  enabled: boolean;
  repaint?: (subject: string) => void;
}

/** Illustrations for the current game, looked up by creature ID (or "scene"). */
export const ArtContext = createContext<ArtState>({ art: {}, painting: [], enabled: false });

export const useArt = () => useContext(ArtContext);
