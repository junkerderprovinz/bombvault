// ---------------------------------------------------------------------------
// Advanced mode — global toggle, persisted per-browser in localStorage.
// Default OFF (clean/simple UI); ON reveals expert/advanced controls.
// ---------------------------------------------------------------------------

import { createContext, useContext, useState, type ReactNode } from "react";

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

  const setAdvanced = (v: boolean) => {
    setState(v);
    try {
      localStorage.setItem(KEY, v ? "1" : "0");
    } catch {
      /* ignore */
    }
  };

  return <Ctx.Provider value={{ advanced, setAdvanced }}>{children}</Ctx.Provider>;
}

export function useAdvanced() {
  return useContext(Ctx);
}
