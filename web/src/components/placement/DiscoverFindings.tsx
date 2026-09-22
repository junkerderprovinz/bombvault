import { useEffect, useState } from "react";
import { Button } from "../Button";
import { connectRepo, listOffsiteTargets, type DirectRepoFinding, type PlacementDomain } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { domainLabel, formatList } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { reposChanged } from "../../lib/useNamedRepos";

/** DiscoverFindings says what a discover left for the placement: paused
 *  domains, items it left open, and direct backups that lost their target.
 *
 *  Discover's own candidates are not filtered by whether a target is
 *  switched on, so this card does it: `enabledTargets` starts `null` and
 *  every candidate shows, then narrows to the confirmed ones once the fetch
 *  answers, rather than flashing empty while it is in flight. */
export function DiscoverFindings({
  paused,
  leftOpen,
  directRepos,
}: {
  paused: PlacementDomain[];
  leftOpen: string[];
  directRepos: DirectRepoFinding[];
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [connected, setConnected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [enabledTargets, setEnabledTargets] = useState<Set<string> | null>(null);
  const where = [t("nav.settings"), t("settings.tab.storage"), t("placementDefaults.title")].join(" > ");

  useEffect(() => {
    let active = true;
    listOffsiteTargets()
      .then((r) => {
        if (active) setEnabledTargets(new Set(r.ok ? (r.targets ?? []).filter((x) => x.enabled).map((x) => x.id) : []));
      })
      .catch(() => {
        if (active) setEnabledTargets(new Set());
      });
    return () => {
      active = false;
    };
  }, []);

  async function connect(finding: DirectRepoFinding, target: { id: string; name: string }) {
    setBusy(true);
    try {
      const r = await connectRepo(finding.repoId, target.id);
      if (!r.ok) {
        push(placementErrorText(t, lang, r, "settings.error"), "fail");
        return;
      }
      push(t("discover.connected").replace("{name}", () => finding.name).replace("{target}", () => target.name), "success");
      setConnected((prev) => [...prev, finding.repoId]);
      reposChanged();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      {paused.length > 0 && (
        <p className="text-sm text-statusWarn">
          {t("discover.paused")
            .replace("{domains}", () => formatList(lang, paused.map((d) => domainLabel(t, d))))
            .replace("{where}", () => where)}
        </p>
      )}
      {leftOpen.length > 0 && (
        <p className="text-sm text-carbon-textSub">{t("discover.leftOpen").replace("{list}", () => formatList(lang, leftOpen))}</p>
      )}
      {directRepos
        .filter((f) => !connected.includes(f.repoId))
        .map((f) => (
          <div key={f.repoId} className="flex flex-col gap-1">
            <p className="text-sm text-carbon-textSub">{t("discover.directFound").replace("{name}", () => f.name)}</p>
            <div className="flex items-center gap-2 flex-wrap">
              {f.targets
                .filter((target) => !enabledTargets || enabledTargets.has(target.id))
                .map((target) => (
                  <Button
                    key={target.id}
                    label={t("discover.connectDirect").replace("{target}", () => target.name)}
                    labelKey="discover.connectDirect"
                    tone="neutral"
                    disabled={busy}
                    onClick={() => void connect(f, target)}
                  />
                ))}
            </div>
          </div>
        ))}
    </div>
  );
}
