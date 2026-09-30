// PairingSection is the Pairing tab of Settings, where instances become one
// group by twelve words: a sentence and three cards explain it, one card holds
// the words and the members, and one picks the relay for members on other
// networks. Receivers, pull sources and the Instances page all build on the
// group it sets up.
import { useCallback, useEffect, useState } from "react";
import { getGroup, type GroupState } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { PairingSteps } from "./PairingSteps";
import { PhraseCard } from "./PhraseCard";
import { RelayCard } from "./RelayCard";

// How often the tab asks again who is reachable. Members come and go with the
// relay connection and the local network, which nothing pushes here.
const REFRESH_MS = 10_000;

export function PairingSection({ t, nextHue }: { t: ReturnType<typeof useT>["t"]; nextHue: () => number }) {
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

  const hues: [number, number, number] = [nextHue(), nextHue(), nextHue()];
  const phraseHue = nextHue();
  const relayHue = nextHue();

  return (
    <>
      <PairingSteps t={t} hues={hues} />
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}
      {group && <PhraseCard group={group} onGroup={setGroup} onRefresh={load} t={t} hueIndex={phraseHue} />}
      {group && <RelayCard group={group} onGroup={setGroup} t={t} hueIndex={relayHue} />}
    </>
  );
}
