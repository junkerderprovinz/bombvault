// ---------------------------------------------------------------------------
// Advanced mode — global toggle, persisted per-browser in localStorage.
// Default OFF (clean/simple UI); ON reveals expert/advanced controls.
// ---------------------------------------------------------------------------

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { ADOPTED_EVENT, save as saveDisplayPrefs } from "./displayPrefs";

const KEY = "bombvault.advanced";

const Ctx = createContext<{ advanced: boolean; setAdvanced: (v: boolean) => void }>({
  advanced: false,
  setAdvanced: () => {},
});

export function AdvancedProvider({ children }: { children: ReactNode }) {
  const [advanced, setState] = useState<boolean>(() => {
    try {
      return localStorage.getItem(KEY) === "1";
    } catch {
      return false;
    }
  });

  // The initial read above happens once, at mount. A browser that adopts the
  // server's look a moment later changes this key underneath that read, and
  // without this the page would sit in the simple view with "1" in storage —
  // one half of what #191 looked like. Reading storage again is enough; this
  // must not save, or adopting would echo straight back to the server.
  useEffect(() => {
    const onAdopted = () => {
      try {
        setState(localStorage.getItem(KEY) === "1");
      } catch {
        /* storage disabled: nothing to adopt */
      }
    };
    window.addEventListener(ADOPTED_EVENT, onAdopted);
    return () => window.removeEventListener(ADOPTED_EVENT, onAdopted);
  }, []);

  const setAdvanced = (v: boolean) => {
    setState(v);
    try {
      localStorage.setItem(KEY, v ? "1" : "0");
      saveDisplayPrefs();
    } catch {
      /* ignore */
    }
  };

  return <Ctx.Provider value={{ advanced, setAdvanced }}>{children}</Ctx.Provider>;
}

export function useAdvanced() {
  return useContext(Ctx);
}

export function Advanced({ when = true, children }: { when?: boolean; children: ReactNode }) {
  const { advanced } = useAdvanced();
  return advanced && when ? <>{children}</> : null;
}
