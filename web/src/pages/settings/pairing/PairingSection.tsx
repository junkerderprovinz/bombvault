// PairingSection is the Pairing page of Settings, where instances become one
// group by twelve words: three cards explain it, one holds the words, one
// lists who is in the group, one names this instance, and one picks the relay
// for members on other networks. Receivers, fetchers and the Instances page
// all build on the group it sets up.
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { getGroup, type GroupState } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { GroupCard } from "./GroupCard";
import { PairingSteps } from "./PairingSteps";
import { PhraseCard, useJoinedAgo } from "./PhraseCard";
import { RelayCard, SelfAddressField } from "./RelayCard";
import { pairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

// How often the page asks again who is reachable. Members come and go with the
// relay connection and the local network, which nothing pushes here.
const REFRESH_MS = 10_000;

function GroupCards({
  group,
  onGroup,
  onRefresh,
  zfs,
  instanceCard,
  t,
  nextHue,
}: {
  group: GroupState;
  onGroup: (g: GroupState) => void;
  onRefresh: () => void;
  zfs: boolean;
  instanceCard: (address: ReactNode, hueIndex: number) => ReactNode;
  t: T;
  nextHue: () => number;
}) {
  const [createdHere, setCreatedHere] = useState(false);
  const joinedAgo = useJoinedAgo(group);
  const stage = pairStage(group, joinedAgo, createdHere);
  return (
    <>
      <PhraseCard group={group} stage={stage} onCreatedHere={setCreatedHere} onGroup={onGroup} onRefresh={onRefresh} t={t} hueIndex={nextHue()} />
      <GroupCard group={group} stage={stage} joinedAgo={joinedAgo} zfs={zfs} t={t} hueIndex={nextHue()} />
      {instanceCard(<SelfAddressField group={group} onGroup={onGroup} t={t} />, nextHue())}
      <RelayCard group={group} stage={stage} onGroup={onGroup} onRefresh={onRefresh} t={t} hueIndex={nextHue()} />
    </>
  );
}

export function PairingSection({
  t,
  nextHue,
  zfs,
  instanceCard,
}: {
  t: T;
  nextHue: () => number;
  /** The ZFS section is on, so the group can take a ZFS server over SSH. */
  zfs: boolean;
  /** Draws the card that names this instance, around the field for its own
   *  address. */
  instanceCard: (address: ReactNode, hueIndex: number) => ReactNode;
}) {
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

  return (
    <>
      <PairingSteps t={t} hues={hues} />
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}
      {group && (
        <GroupCards group={group} onGroup={setGroup} onRefresh={load} zfs={zfs} instanceCard={instanceCard} t={t} nextHue={nextHue} />
      )}
    </>
  );
}
