// Pairing is the Instances tab where instances become one group by twelve
// words: a sentence and three cards explain it, one card holds the words and
// the members, and one picks the relay for members on other networks.
// Receivers, pull sources and the Fleet tab all build on the group this tab
// sets up.
import { useCallback, useEffect, useState } from "react";
import { getGroup, type GroupState } from "../lib/api";
import { useT } from "../lib/i18n";
import { PageTitle } from "../components/PageTitle";
import { PAGE_SHELL_RESPONSIVE, PAGE_SHELL_TABBED_RESPONSIVE } from "../lib/pageShell";
import { PairingSteps } from "./instances/PairingSteps";
import { PhraseCard } from "./instances/PhraseCard";
import { RelayCard } from "./instances/RelayCard";

// How often the page asks again who is reachable. Members come and go with
// the relay connection and the local network, which nothing pushes here.
const REFRESH_MS = 10_000;

/** With `embedded` the page is a tab of Instances, which owns the shell. */
export function Pairing({ embedded = false }: { embedded?: boolean } = {}) {
  const { t } = useT();
  const [group, setGroup] = useState<GroupState | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    getGroup()
      .then((g) => {
        if (g.ok) {
          setGroup(g);
          setError(null);
        } else {
          setError(g.error ?? t("pairing.loadError"));
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("pairing.loadError")));
  }, [t]);

  useEffect(() => {
    load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, [load]);

  return (
    <div className={embedded ? PAGE_SHELL_TABBED_RESPONSIVE : PAGE_SHELL_RESPONSIVE}>
      {!embedded && <PageTitle>{t("pairing.title")}</PageTitle>}
      <PairingSteps t={t} hues={[0, 1, 2]} />
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}
      {group && <PhraseCard group={group} onGroup={setGroup} onRefresh={load} t={t} hueIndex={3} />}
      {group && <RelayCard group={group} onGroup={setGroup} t={t} hueIndex={4} />}
    </div>
  );
}
