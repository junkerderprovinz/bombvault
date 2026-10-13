// The name this server goes by in its pairing group and in the Android app's
// list of servers. The layout hands it down with the settings it reads anyway,
// so a page has it before the app answers any question.
import { createContext, useContext } from "react";

export const InstanceNameContext = createContext("");

/** useInstanceName is the server's own name, "BombVault" while it has none,
 *  the same name its group shows. */
export function useInstanceName(): string {
  return useContext(InstanceNameContext).trim() || "BombVault";
}
