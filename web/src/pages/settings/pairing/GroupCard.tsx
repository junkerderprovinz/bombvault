// GroupCard lists who is in the group: this instance with where it stands,
// every paired instance and phone with how it is reached or when it was last
// there, the devices that wait to be let in or did not come along into the
// fortress, and the ZFS servers without BombVault, which join over SSH from
// the same list.
import { useState, type ReactNode } from "react";
import { Card } from "../shared";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { IconConfirm, IconLink, IconReject } from "../../../components/glyphs";
import { IconFleet, IconZFS } from "../../../components/navGlyphs";
import { AddReplicaServerDialog } from "../../../components/zfs/replica/AddReplicaServerDialog";
import { useReplicaServers } from "../../../components/zfs/replica/replicaStore";
import {
  approveMember,
  refuseMember,
  type AwayMember,
  type GroupMember,
  type GroupState,
  type LeftBehindMember,
  type WaitingMember,
} from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { PHONE_PATH } from "../../instances/InstanceCard";
import { RemoveInstanceButton } from "../../instances/RemoveInstanceButton";
import { STAGE_BADGE, clock, type PairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

/** Row is one instance of the group: a glyph, its name and what to say about it. */
function Row({ glyph, name, mark, badge }: { glyph: ReactNode; name: string; mark?: string; badge?: ReactNode }) {
  return (
    <SettingRow as="li" label={name} glyph={<span className="text-accentText [&>svg]:h-4.5 [&>svg]:w-4.5">{glyph}</span>}>
      {mark && <span className="text-caption uppercase tracking-[.12em] text-carbon-textSub">{mark}</span>}
      {badge}
    </SettingRow>
  );
}

/** Paired is a state of the member and nothing to press, so it is a dot and
 *  words, followed by how the member is reached. */
function PairedState({ via, t }: { via?: string; t: T }) {
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap text-subline text-carbon-textSub">
      <i aria-hidden="true" className="h-[7px] w-[7px] flex-none rounded-full bg-statusOkSolid" />
      {via ? `${t("pairing.paired")} · ${via}` : t("pairing.paired")}
    </span>
  );
}

function memberGlyph(kind: string | undefined) {
  if (kind !== "android") return <IconFleet />;
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
      <path fillRule="evenodd" d={PHONE_PATH} />
    </svg>
  );
}

function MemberRow({ m, t }: { m: GroupMember; t: T }) {
  return (
    <Row
      glyph={memberGlyph(m.kind)}
      name={m.name || m.id}
      mark={m.kind === "android" ? t("fleet.kindAndroid") : undefined}
      badge={<PairedState via={m.direct ? t("pairing.direct") : t("pairing.viaRelay")} t={t} />}
    />
  );
}

/** AwayRow is a member that was in the group and is not in reach now, such as
 *  a server that was shut down or a phone with the app closed. It stays until
 *  it is removed on the Instances page, grey rather than red, since being away
 *  is no fault. Right after this instance started it is not known to be away
 *  yet. */
function AwayRow({ m, settling, t }: { m: AwayMember; settling: boolean; t: T }) {
  const seen = m.lastSeen > 0 && !settling ? t("instances.lastSeen").replace("{time}", () => relativeTime(t, m.lastSeen)) : "";
  return (
    <SettingRow
      as="li"
      label={m.name || m.id}
      note={seen || undefined}
      glyph={<span className="text-accentText [&>svg]:h-4.5 [&>svg]:w-4.5">{memberGlyph(m.kind)}</span>}
    >
      {m.kind === "android" && <span className="text-caption uppercase tracking-[.12em] text-carbon-textSub">{t("fleet.kindAndroid")}</span>}
      <span data-new="pairing-inactive" className="flex">
        <Badge tone="neutral">{t(settling ? "instances.checking" : "instances.inactive")}</Badge>
      </span>
    </SettingRow>
  );
}

/** Waiting lists the devices that entered the twelve words and wait for
 *  somebody here to let them in, each with its key to compare with what it
 *  shows, and says when this instance waits on another one. */
function Waiting({ group, onGroup, t }: { group: GroupState; onGroup: (g: GroupState) => void; t: T }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const waiting = group.waiting ?? [];
  const asks = waiting.filter((w) => !w.there);
  const there = waiting.filter((w) => w.there);
  if (asks.length === 0 && there.length === 0) return null;

  async function decide(w: WaitingMember, approve: boolean) {
    setBusy(w.id);
    try {
      const res = await (approve ? approveMember(w.id) : refuseMember(w.id));
      if (res.ok) {
        onGroup(res);
        setError(null);
      } else {
        setError(res.error ?? t("pairing.loadError"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("pairing.loadError"));
    } finally {
      setBusy(null);
    }
  }

  const ownKey = group.fingerprint ? t("pairing.ownKey").replace("{fingerprint}", () => group.fingerprint ?? "") : "";
  return (
    <div data-testid="waiting" className="flex flex-col gap-2 border-t border-carbon-border/60 pt-3">
      {asks.length > 0 && (
        <>
          <SettingRow label={t("pairing.pendingTitle")} hint={t("pairing.pendingTip")} />
          <Rows as="ul">
            {asks.map((w) => (
              <SettingRow
                key={w.id}
                as="li"
                label={w.name || w.id}
                note={w.fingerprint ? t("pairing.fingerprint").replace("{fingerprint}", () => w.fingerprint ?? "") : undefined}
                glyph={<span className="text-accentText [&>svg]:h-4.5 [&>svg]:w-4.5">{memberGlyph(w.kind)}</span>}
              >
                <Button
                  label={t("pairing.refuse")}
                  labelKey="pairing.refuse"
                  glyph={<IconReject />}
                  tone="neutral"
                  onClick={() => void decide(w, false)}
                  disabled={busy !== null}
                />
                <Button
                  label={t("pairing.approve")}
                  labelKey="pairing.approve"
                  glyph={<IconConfirm />}
                  tone="accent"
                  onClick={() => void decide(w, true)}
                  disabled={busy !== null}
                  busy={busy === w.id}
                />
              </SettingRow>
            ))}
          </Rows>
        </>
      )}
      {there.length > 0 && <p className="text-sm text-carbon-textSub">{t("pairing.awaiting")}</p>}
      {ownKey && (
        <SettingRow label={ownKey} hint={t("pairing.ownKeyTip")} />
      )}
      {error && <p className="text-caption text-statusFail">{error}</p>}
    </div>
  );
}

/** Behind lists the devices that had not followed into the fortress when the
 *  old phrase closed, by name and kind alone. Entering the fortress phrase on
 *  one brings it along, and removing it here shuts its key out. */
function Behind({ behind, onRefresh, t }: { behind: LeftBehindMember[]; onRefresh: () => void; t: T }) {
  if (behind.length === 0) return null;
  return (
    <div data-testid="behind" className="flex flex-col gap-2 border-t border-carbon-border/60 pt-3">
      <SettingRow label={t("pairing.behindTitle")} hint={t("pairing.behindTip")} />
      <Rows as="ul">
        {behind.map((m) => (
          <SettingRow
            key={m.id}
            as="li"
            label={m.name || m.id}
            glyph={<span className="text-accentText [&>svg]:h-4.5 [&>svg]:w-4.5">{memberGlyph(m.kind)}</span>}
          >
            <span className="text-caption uppercase tracking-[.12em] text-carbon-textSub">
              {t(m.kind === "android" ? "fleet.kindAndroid" : "pairing.kindInstance")}
            </span>
            <RemoveInstanceButton instance={{ memberId: m.id, name: m.name || m.id }} onRemoved={onRefresh} />
          </SettingRow>
        ))}
      </Rows>
    </div>
  );
}

function WaitRow({ text, seconds }: { text: string; seconds?: number }) {
  return (
    <div className="flex items-center gap-3 border-t border-carbon-border/60 pt-3 text-carbon-textSub">
      <span className="flex shrink-0 gap-1" aria-hidden="true">
        {[0, 1, 2].map((i) => (
          <span key={i} className="glim-wait-dot h-1.5 w-1.5 rounded-full bg-accentText" style={{ animationDelay: `${i * 200}ms` }} />
        ))}
      </span>
      <span className="text-sm">{text}</span>
      {seconds !== undefined && <span className="ms-auto text-xs tabular-nums text-carbon-textMuted">{clock(seconds)}</span>}
    </div>
  );
}

export function GroupCard({
  group,
  onGroup,
  onRefresh,
  stage,
  joinedAgo,
  zfs,
  t,
  hueIndex,
}: {
  group: GroupState;
  onGroup: (g: GroupState) => void;
  /** Reads the group again, after a device left behind was removed. */
  onRefresh: () => void;
  stage: PairStage;
  joinedAgo: number;
  /** The ZFS section is on, so a ZFS server can be connected. */
  zfs: boolean;
  t: T;
  hueIndex?: number;
}) {
  const { servers, reload } = useReplicaServers();
  const [adding, setAdding] = useState(false);

  const badge = stage !== "unpaired" && STAGE_BADGE[stage];
  return (
    <Card title={t("instances.title")} hint={t("pairing.group.hint")} hueIndex={hueIndex}>
      <div className="flex min-w-0 flex-col gap-2">
        <div data-testid="members">
          <Rows as="ul">
            <Row
              glyph={<IconFleet />}
              name={group.name}
              mark={t("instances.thisInstance")}
              badge={
                badge && (
                  <span data-testid="pair-state" className="flex">
                    {stage === "paired" ? <PairedState t={t} /> : <Badge tone={badge.tone}>{t(badge.key)}</Badge>}
                  </span>
                )
              }
            />
            {group.members.map((m) => (
              <MemberRow key={m.id} m={m} t={t} />
            ))}
            {group.away.map((m) => (
              <AwayRow key={m.id} m={m} settling={group.settling} t={t} />
            ))}
          </Rows>
        </div>
        <Waiting group={group} onGroup={onGroup} t={t} />
        <Behind behind={group.behind ?? []} onRefresh={onRefresh} t={t} />
        {(group.older ?? []).length > 0 && (
          <p data-testid="older" className="border-t border-carbon-border/60 pt-3 text-sm text-statusWarn">
            {t("pairing.older").replace("{names}", () => (group.older ?? []).join(", "))}
          </p>
        )}
        {stage === "new" ? (
          <WaitRow text={t("pairing.waitNext")} />
        ) : stage !== "paired" && stage !== "unpaired" ? (
          <WaitRow text={t("pairing.searching")} seconds={stage === "searching" ? joinedAgo : undefined} />
        ) : null}
        {zfs && (
          <div data-testid="zfs-row" className="border-t border-carbon-border/60 pt-3">
            <SettingRow
              label={t("pairing.group.zfsTitle")}
              glyph={<span className="text-accentText [&>svg]:h-4.5 [&>svg]:w-4.5"><IconZFS /></span>}
              hint={t("pairing.group.zfsTip")}
              note={servers.length > 0 ? servers.map((s) => s.name).join(", ") : t("pairing.group.zfsNone")}
            >
              <Button
                label={t("pairing.group.zfsConnect")}
                labelKey="pairing.group.zfsConnect"
                glyph={<IconLink />}
                tone="neutral"
                onClick={() => setAdding(true)}
              />
            </SettingRow>
          </div>
        )}
      </div>
      {adding && (
        <AddReplicaServerDialog
          onClose={() => setAdding(false)}
          onSaved={() => {
            setAdding(false);
            reload();
          }}
        />
      )}
    </Card>
  );
}
