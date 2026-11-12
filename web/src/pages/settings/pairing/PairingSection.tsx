// PairingSection is the Pairing page of Settings, where instances, phones and
// the other apps of the family come together under twelve words. Three cards
// explain it, the card of the phrase follows with this instance's name, then
// whoever waits to be let in, the members as the Instances tab draws them,
// the other apps of the fortress, and last the relay and the ZFS servers,
// which are set up once.
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { storedShowOthers, storeShowOthers } from "@glimstone/core";
import { getFortress, getGroup, type FortressState, type GroupState } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { fortressApps } from "./fortressModel";
import { OtherApps, type GiveBack } from "./FortressSection";
import { MemberCards, WaitingCard, ZfsCard } from "./Members";
import { PairingSteps } from "./PairingSteps";
import { PhraseCard, useJoinedAgo } from "./PhraseCard";
import { RelayCard } from "./RelayCard";
import { pairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

// How often the page asks again who is reachable. Members come and go with the
// relay connection and the local network, which nothing pushes here.
const REFRESH_MS = 10_000;

/** take reserves n rainbow positions in a row, for the fortress's switch, its
 *  cards and its permissions, and returns the first. */
function take(nextHue: () => number, n: number): number {
  const first = nextHue();
  for (let i = 1; i < n; i++) nextHue();
  return first;
}

function Cards({
  group,
  view,
  onGroup,
  onRefresh,
  zfs,
  nameRow,
  giveBack,
  t,
  nextHue,
}: {
  group: GroupState;
  view: FortressState;
  onGroup: (g: GroupState) => void;
  onRefresh: () => void;
  zfs: boolean;
  nameRow: (defaultName: string) => ReactNode;
  giveBack?: GiveBack;
  t: T;
  nextHue: () => number;
}) {
  const [createdHere, setCreatedHere] = useState(false);
  const [showOthers, setShowOthers] = useState(storedShowOthers);
  const joinedAgo = useJoinedAgo(group);
  const apps = view.joined ? fortressApps(view.members, (unix) => relativeTime(t, unix)) : [];
  // With other apps in the fortress nobody waits for a first member, so the
  // card has nothing to suggest and no line counts the seconds.
  const found = pairStage(group, joinedAgo, createdHere);
  const stage = apps.length > 0 && (found === "new" || found === "searching" || found === "alone") ? "paired" : found;
  const others =
    apps.length > 0
      ? {
          apps,
          on: showOthers,
          onChange: (on: boolean) => {
            storeShowOthers(on);
            setShowOthers(on);
          },
        }
      : undefined;
  return (
    <>
      <PhraseCard
        group={group}
        view={view}
        stage={stage}
        nameRow={nameRow(view.defaultDevice)}
        onCreatedHere={setCreatedHere}
        onGroup={onGroup}
        onRefresh={onRefresh}
        t={t}
        hueIndex={nextHue()}
        others={others}
      />
      <WaitingCard group={group} onGroup={onGroup} t={t} hueIndex={nextHue()} />
      {(group.active || view.joined) && (
        <MemberCards group={group} stage={stage} joinedAgo={joinedAgo} waitFor={apps.length === 0} onRefresh={onRefresh} nextHue={nextHue} t={t} />
      )}
      {apps.length > 0 && <OtherApps view={view} apps={apps} group={group} giveBack={giveBack} showOthers={showOthers} hueStart={take(nextHue, apps.length + 2)} onChanged={onRefresh} />}
      <RelayCard group={group} stage={stage} onGroup={onGroup} onRefresh={onRefresh} t={t} hueIndex={nextHue()} />
      {zfs && <ZfsCard t={t} hueIndex={nextHue()} />}
    </>
  );
}

export function PairingSection({
  t,
  nextHue,
  zfs,
  nameRow,
  giveBack,
}: {
  t: T;
  nextHue: () => number;
  /** The ZFS section is on, so a ZFS server can be connected over SSH. */
  zfs: boolean;
  /** Draws the row that names this instance, with the name it goes by without one. */
  nameRow: (defaultName: string) => ReactNode;
  giveBack?: GiveBack;
}) {
  const [group, setGroup] = useState<GroupState | null>(null);
  const [view, setView] = useState<FortressState | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    Promise.all([getGroup(), getFortress()])
      .then(([g, f]) => {
        if (g.ok && f.ok) {
          setGroup(g);
          setView(f);
          setError(null);
        } else {
          setError((g.ok ? f.error : g.error) ?? t("pairing.loadError"));
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
      {group && view && (
        <Cards
          group={group}
          view={view}
          onGroup={setGroup}
          onRefresh={load}
          zfs={zfs}
          nameRow={nameRow}
          giveBack={giveBack}
          t={t}
          nextHue={nextHue}
        />
      )}
    </>
  );
}
