import { useEffect, useState } from "react";
import { Navigate, useLocation } from "react-router-dom";

import { PageTitle } from "../components/PageTitle";
import { listItems } from "../lib/api";
import { legacyLookupName, legacyRedirect, type EntryKind } from "../lib/backupEntry";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";

/**
 * KindRedirect answers a link to a kind's own page, which bookmarks, release
 * notes and support answers carry. Such a link names a folder set or a ZFS
 * item by its name, so the list is read first to find the id its page is
 * addressed by.
 */
export function KindRedirect({ kind }: { kind: EntryKind }) {
  const { t } = useT();
  const { search } = useLocation();
  const name = legacyLookupName(kind, search);
  const [keys, setKeys] = useState<Map<string, string> | null>(null);

  useEffect(() => {
    if (name === "") return;
    let alive = true;
    // A list that cannot be read finds no entry, and the link then leads to
    // the list of its kind.
    listItems()
      .then((res) => (res.ok ? res.items : []))
      .catch(() => [])
      .then((items) => {
        if (alive) setKeys(new Map(items.filter((item) => item.kind === kind).map((item) => [item.name, item.key])));
      });
    return () => {
      alive = false;
    };
  }, [kind, name]);

  if (name === "") return <Navigate to={legacyRedirect(kind, search)} replace />;
  if (keys) return <Navigate to={legacyRedirect(kind, search, (wanted) => keys.get(wanted))} replace />;
  return (
    <div className={PAGE_SHELL}>
      <PageTitle>{t("backups.title")}</PageTitle>
      <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
    </div>
  );
}
