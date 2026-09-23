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

/** useHostLabel is what the interface calls this server: the product it runs on,
 *  or "Host" on a plain Docker host and until the answer is in. */
export function useHostLabel(): string {
  const { t } = useT();
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
  return PRODUCTS[kind] ?? t("placement.hostGeneric");
}

/** The host label settled rather than reactive, for a flow that builds its
 *  text once inside an effect and has no re-render left to pick up a later
 *  update. Empty until the platform is unrecognised; the caller supplies its
 *  own fallback text. */
export function hostLabelSettled(): Promise<string> {
  return loadPlatform().then((p) => PRODUCTS[p] ?? "");
}
