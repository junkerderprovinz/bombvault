// GroupCard lists who is in the group: this instance with where it stands,
// every paired instance and phone with how it is reached, and the ZFS servers
// without BombVault, which join over SSH from the same list. Beside the list
// the group stands as a network.
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Card } from "../shared";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { InfoBubble } from "../../../components/InfoBubble";
import { IconLink } from "../../../components/glyphs";
import { IconFleet, IconZFS } from "../../../components/navGlyphs";
import { AddReplicaServerDialog } from "../../../components/zfs/replica/AddReplicaServerDialog";
import { useReplicaServers } from "../../../components/zfs/replica/replicaStore";
import { listRoles, type GroupMember, type GroupState, type MemberRoles } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import { PHONE_PATH } from "../../instances/InstanceCard";
import { NetPicture, useNetHover } from "../../instances/NetPicture";
import { buildInstances } from "../../instances/instancesModel";
import { groupNodes } from "../../instances/netModel";
import { STAGE_BADGE, clock, type PairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

/** Row is one instance of the group: a glyph, its name and what to say about
 *  it. Rows share one height on the desktop, so one without a badge lines up
 *  with the rest. */
function Row({ netKey, hot, glyph, name, mark, badge }: { netKey: string; hot: boolean; glyph: ReactNode; name: string; mark?: string; badge?: ReactNode }) {
  return (
    <li
      data-net={netKey}
      className={`glim-net-entry flex min-h-11 flex-wrap items-center gap-x-2.5 gap-y-0.5 rounded-control bg-carbon-surface2 px-3 py-1.5 md:flex-nowrap${
        hot ? " glim-net-entry-hot" : ""
      }`}
    >
      <span className="shrink-0 text-carbon-textMuted [&>svg]:h-4.5 [&>svg]:w-4.5">{glyph}</span>
      <span className="min-w-0 text-sm font-semibold text-carbon-text wrap-anywhere md:truncate">{name}</span>
      {mark && <span className="shrink-0 text-caption font-semibold uppercase tracking-widest text-carbon-textMuted">{mark}</span>}
      {badge && <span className="ms-auto flex shrink-0 items-center gap-2">{badge}</span>}
    </li>
  );
}

function MemberRow({ m, hot, t }: { m: GroupMember; hot: boolean; t: T }) {
  const phone = m.kind === "android";
  return (
    <Row
      netKey={m.id}
      hot={hot}
      glyph={
        phone ? (
          <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
            <path fillRule="evenodd" d={PHONE_PATH} />
          </svg>
        ) : (
          <IconFleet />
        )
      }
      name={m.name || m.id}
      mark={phone ? t("fleet.kindAndroid") : undefined}
      badge={
        <>
          <Badge tone="ok">{t("pairing.paired")}</Badge>
          <span className="text-xs text-carbon-textSub">{m.direct ? t("pairing.direct") : t("pairing.viaRelay")}</span>
        </>
      }
    />
  );
}

function WaitRow({ text, seconds }: { text: string; seconds?: number }) {
  return (
    <div className="flex items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2.5 text-carbon-textSub">
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
  const { hot, handlers } = useNetHover();
  const [roles, setRoles] = useState<MemberRoles[]>([]);
  const [adding, setAdding] = useState(false);

  // The arrows follow who is in the group, so they are read again when a
  // member comes or goes.
  const present = group.members.map((m) => m.id).join(" ");
  useEffect(() => {
    listRoles()
      .then((res) => {
        if (res.ok) setRoles(res.members ?? []);
      })
      .catch(() => undefined);
  }, [present]);

  const nodes = useMemo(() => {
    const { instances } = buildInstances({
      members: group.members,
      peers: [],
      received: [],
      pulls: [],
      offers: [],
      roles: roles.filter((r) => group.members.some((m) => m.id === r.memberId)),
      destinations: null,
      groupReceivers: null,
      replicas: [],
    });
    return groupNodes(instances, zfs ? servers : [], t);
  }, [group.members, roles, servers, zfs, t]);

  const badge = stage !== "unpaired" && STAGE_BADGE[stage];
  const list = (
    <div className="flex min-w-0 flex-col gap-2">
      <ul className="flex flex-col gap-2" data-testid="members">
        <Row
          netKey="self"
          hot={hot === "self"}
          glyph={<IconFleet />}
          name={group.name}
          mark={t("instances.thisInstance")}
          badge={
            badge && (
              <span data-testid="pair-state">
                <Badge tone={badge.tone}>{t(badge.key)}</Badge>
              </span>
            )
          }
        />
        {group.members.map((m) => (
          <MemberRow key={m.id} m={m} hot={hot === m.id} t={t} />
        ))}
      </ul>
      {stage === "new" ? (
        <WaitRow text={t("pairing.waitNext")} />
      ) : stage !== "paired" && stage !== "unpaired" ? (
        <WaitRow text={t("pairing.searching")} seconds={stage === "searching" ? joinedAgo : undefined} />
      ) : null}
      {zfs && (
        <div data-testid="zfs-row" className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-control bg-carbon-surface2 text-accentText">
            <IconZFS />
          </span>
          <span className="flex min-w-0 flex-[1_1_12rem] flex-col gap-0.5">
            <span className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
              {t("pairing.group.zfsTitle")}
              <InfoBubble tip={t("pairing.group.zfsTip")} />
            </span>
            <span className="text-xs text-carbon-textMuted wrap-anywhere">
              {servers.length > 0 ? servers.map((s) => s.name).join(", ") : t("pairing.group.zfsNone")}
            </span>
          </span>
          <Button
            label={t("pairing.group.zfsConnect")}
            labelKey="pairing.group.zfsConnect"
            glyph={<IconLink />}
            tone="neutral"
            onClick={() => setAdding(true)}
          />
        </div>
      )}
    </div>
  );

  return (
    <Card title={t("instances.title")} hint={t("pairing.group.hint")} hueIndex={hueIndex}>
      {nodes.length > 0 ? (
        <div className="grid grid-cols-1 items-center gap-x-7 gap-y-3 min-[1400px]:grid-cols-[minmax(0,1fr)_minmax(0,24rem)] [--net-ground:var(--carbon-surface)]" {...handlers}>
          {list}
          <NetPicture selfName={group.name} nodes={nodes} hot={hot} />
        </div>
      ) : (
        list
      )}
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
