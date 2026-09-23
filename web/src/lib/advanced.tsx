// Advanced mode: a global toggle kept in localStorage. Off by default; on
// reveals the expert controls.

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

  // Adopting the server's stored look can change this key after the mount
  // read, so read it again. No save here, or the adopted value would echo
  // straight back to the server.
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
