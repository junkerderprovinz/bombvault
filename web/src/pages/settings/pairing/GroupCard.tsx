// GroupCard lists who is in the group: this instance with where it stands,
// every paired instance and phone with how it is reached or when it was last
// there, and the ZFS servers without BombVault, which join over SSH from the
// same list.
import { useState, type ReactNode } from "react";
import { Card } from "../shared";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { IconLink } from "../../../components/glyphs";
import { IconFleet, IconZFS } from "../../../components/navGlyphs";
import { AddReplicaServerDialog } from "../../../components/zfs/replica/AddReplicaServerDialog";
import { useReplicaServers } from "../../../components/zfs/replica/replicaStore";
import type { AwayMember, GroupMember, GroupState } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { PHONE_PATH } from "../../instances/InstanceCard";
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
  stage,
  joinedAgo,
  zfs,
  t,
  hueIndex,
}: {
  group: GroupState;
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
