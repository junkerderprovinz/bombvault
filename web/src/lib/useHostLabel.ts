import { useEffect, useState } from "react";
import { getSettings } from "./api";
import { useT } from "./i18n";

// The platform does not change while a page is open, so one request serves every card.
let platform: Promise<string> | null = null;

const PRODUCTS: Record<string, string> = { unraid: "Unraid", truenas: "TrueNAS" };

/** useHostLabel is what the interface calls this server: the product it runs on,
 *  or "Host" on a plain Docker host and until the answer is in. */
export function useHostLabel(): string {
  const { t } = useT();
  const [kind, setKind] = useState("");
  useEffect(() => {
    let alive = true;
    if (!platform) platform = getSettings().then((r) => r.platform ?? "", () => "");
    void platform.then((p) => {
      if (alive) setKind(p);
    });
    return () => {
      alive = false;
    };
  }, []);
  return PRODUCTS[kind] ?? t("placement.hostGeneric");
}
