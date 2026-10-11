import { useEffect, useState } from "react";
import { getSettings } from "./api";
import { useT } from "./i18n";

// The platform does not change while a page is open, so one request serves every card.
let platform: Promise<string> | null = null;

const PRODUCTS: Record<string, string> = { unraid: "Unraid", truenas: "TrueNAS" };

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

/** useHostLabel is what the interface calls this server: the product it runs on,
 *  or "Host" on a plain Docker host and until the answer is in. */
export function useHostLabel(): string {
  const { t } = useT();
  return PRODUCTS[usePlatform()] ?? t("placement.hostGeneric");
}

/** The host label settled rather than reactive, for a flow that builds its
 *  text once inside an effect and has no re-render left to pick up a later
 *  update. Empty when the platform is not one this interface names; the caller
 *  supplies its own fallback text. */
export function hostLabelSettled(): Promise<string> {
  return loadPlatform().then((p) => PRODUCTS[p] ?? "");
}
