import { useEffect, useState } from "react";
import { getSettings } from "./api";
import { PLACE_DOMAINS, type PlaceDomain } from "./places";

/** useOfferedDomains is the domains a place or a copy can be set up for. The
 *  Domains card has no ZFS row while ZFS is off, so a copy made for ZFS then
 *  could be neither seen nor switched off there. */
export function useOfferedDomains(): PlaceDomain[] {
  const [zfs, setZFS] = useState(false);
  useEffect(() => {
    let alive = true;
    const load = () =>
      void getSettings().then(
        (r) => {
          if (alive) setZFS(r.settings?.zfsEnabled === true);
        },
        () => {}
      );
    load();
    window.addEventListener("bv:settings-changed", load);
    return () => {
      alive = false;
      window.removeEventListener("bv:settings-changed", load);
    };
  }, []);
  return zfs ? PLACE_DOMAINS : PLACE_DOMAINS.filter((d) => d !== "zfs");
}