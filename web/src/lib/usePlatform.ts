import { useEffect, useState } from "react";
import { getSettings } from "./api";

// The platform does not change while a page is open, so one request serves every card.
let platform: Promise<string> | null = null;

function loadPlatform(): Promise<string> {
  if (!platform) platform = getSettings().then((r) => r.platform ?? "", () => "");
  return platform;
}

/** usePlatform is the kind of server BombVault runs on: "unraid", "truenas" or
 *  "generic", and "" until the answer is in. */
export function usePlatform(): string {
  const [kind, setKind] = useState("");
  useEffect(() => {
    let alive = true;
    void loadPlatform().then((p) => {
      if (alive) setKind(p);
    });
    return () => {
      alive = false;
    };
  }, []);
  return kind;
}
