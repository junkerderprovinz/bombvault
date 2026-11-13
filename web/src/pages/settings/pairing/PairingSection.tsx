// PairingSection is the Pairing page of Settings, where instances, phones and
// the other apps of the family come together under twelve words. Three cards
// explain it, the card of the phrase follows with this instance's name, then
// whoever waits to be let in, the members as the Instances tab draws them,
// the other apps of the fortress, and last the relay and the ZFS servers,
// which are set up once.
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { storedShowOthers, storeShowOthers } from "@glimstone/core";
import { Button } from "../../../components/Button";
import { IconClose } from "../../../components/navGlyphs";
import { getFortress, getGroup, openedAsPairedPhone, type FortressState, type GroupState } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { useOpenPasswordField } from "../shared";
import { fortressApps } from "./fortressModel";
import { OtherApps, type GiveBack } from "./FortressSection";
import { MemberCards, WaitingCard, ZfsCard } from "./Members";
import { PairingSteps } from "./PairingSteps";
import { PhraseCard, useJoinedAgo } from "./PhraseCard";
import { RelayCard } from "./RelayCard";
import { PhraseGlyph } from "./pairingArt";
import { pairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

// How often the page asks again who is reachable. Members come and go with the
// relay connection and the local network, which nothing pushes here.
const REFRESH_MS = 10_000;

// Where a closed warning about the missing password is remembered, on this device.
const NO_PASSWORD_CLOSED = "bv-no-password-closed";

function noPasswordClosed(): boolean {
  try {
    return localStorage.getItem(NO_PASSWORD_CLOSED) === "1";
  } catch {
    return false;
  }
}

/** NoPasswordNote warns that pairing without a login password hands the words,
 *  and with them every member's backups, to whoever opens the page. It stands
 *  until a password is set or it is closed on this device. */
function NoPasswordNote({ group, t }: { group: GroupState; t: T }) {
  const openPasswordField = useOpenPasswordField();
  const [closed, setClosed] = useState(noPasswordClosed);
  if (group.passwordSet || closed) return null;
  const close = () => {
    try {
      localStorage.setItem(NO_PASSWORD_CLOSED, "1");
    } catch {
      // A browser that keeps nothing shows it again next time.
    }
    setClosed(true);
  };
  return (
    <section className="flex flex-wrap items-center gap-x-4 gap-y-3 rounded-card bg-statusWarnBgSoft p-4" data-testid="no-password">
      <span className="shrink-0 text-statusWarn">
        <PhraseGlyph kind="warn" size={22} />
      </span>
      <div className="min-w-0 flex-[1_1_18rem]">
        <p className="text-title font-semibold text-carbon-text">{t("pairing.noPasswordTitle")}</p>
        <p className="mt-0.5 text-sm leading-relaxed text-carbon-text">{t("pairing.noPasswordHint")}</p>
      </div>
      <span className="flex items-center gap-2">
        <Button label={t("auth.setPassword")} labelKey="auth.setPassword" tone="accent" onClick={openPasswordField} />
        <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} glyphOnly tone="subtle" onClick={close} />
      </span>
    </section>
  );
}

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
  const apps = view.joined ? fortressApps(view.members, (unix) => relativeTime(t, unix), !openedAsPairedPhone()) : [];
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
      {/* The instances and the other apps' cards keep the gap of one grid between them. */}
      <div className="flex flex-col gap-4 empty:hidden">
        {(group.active || view.joined) && (
          <MemberCards group={group} stage={stage} joinedAgo={joinedAgo} waitFor={apps.length === 0} onRefresh={onRefresh} nextHue={nextHue} t={t} />
        )}
        {apps.length > 0 && (
          <OtherApps view={view} apps={apps} group={group} giveBack={giveBack} showOthers={showOthers} hueStart={take(nextHue, apps.length + 2)} onChanged={onRefresh} />
        )}
      </div>
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
      {group && <NoPasswordNote group={group} t={t} />}
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
