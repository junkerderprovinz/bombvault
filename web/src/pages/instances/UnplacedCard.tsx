import { useState } from "react";

import type { PullSourceView, ReceivedRepoStatus } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { Card } from "../settings/shared";
import { MeshOffers } from "./MeshOffers";
import { PullDialog, PullSourceCard } from "./PullSourceCard";
import { ReceivedRepoCard, ReceiverDialog } from "./ReceivedRepoCard";
import type { Unplaced } from "./instancesModel";
import type { InstancesState } from "./useInstances";

/** UnplacedCard keeps the rows no instance's window can show: received
 *  repositories and pull sources from before pairing, and storage offers
 *  whose sender matches no card. Editing a row and choosing its instance
 *  moves it into that instance's window. */
export function UnplacedCard({
  unplaced,
  state,
  hueIndex,
}: {
  unplaced: Unplaced;
  state: InstancesState;
  hueIndex: number;
}) {
  const { t } = useT();
  const [received, setReceived] = useState<ReceivedRepoStatus | null>(null);
  const [pull, setPull] = useState<PullSourceView | null>(null);

  return (
    <Card title={t("instances.unplaced.title")} hint={t("instances.unplaced.hint")} hueIndex={hueIndex}>
      <MeshOffers offers={unplaced.offers} onChanged={() => void state.reloadOffers()} />
      {unplaced.received.map((r, i) => (
        <ReceivedRepoCard
          key={r.id}
          repo={r}
          t={t}
          index={i}
          onRefresh={() => void state.reloadReceived()}
          onEdit={() => setReceived(r)}
        />
      ))}
      {unplaced.pulls.map((s, i) => (
        <PullSourceCard
          key={s.id}
          source={s}
          t={t}
          index={unplaced.received.length + i}
          onRefresh={() => void state.reloadPulls()}
          onEdit={() => setPull(s)}
        />
      ))}
      {received && (
        <ReceiverDialog
          initial={received}
          t={t}
          onClose={() => setReceived(null)}
          onSaved={() => {
            setReceived(null);
            void state.reloadReceived();
          }}
        />
      )}
      {pull && (
        <PullDialog
          initial={pull}
          t={t}
          onClose={() => setPull(null)}
          onSaved={() => {
            setPull(null);
            void state.reloadPulls();
          }}
        />
      )}
    </Card>
  );
}
