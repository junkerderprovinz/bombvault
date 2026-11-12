// The members of the pairing on the Pairing page: the devices that wait to be
// let in, then this instance and every paired instance and phone as the cards
// of the Instances tab, and the ZFS servers without BombVault, which join
// over SSH.
import { useState } from "react";
import { listOf } from "@glimstone/core";
import { Card } from "../shared";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { InfoBubble } from "../../../components/InfoBubble";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { IconConfirm, IconLink, IconReject } from "../../../components/glyphs";
import { IconFleet, IconZFS } from "../../../components/navGlyphs";
import { AddReplicaServerDialog } from "../../../components/zfs/replica/AddReplicaServerDialog";
import { useReplicaServers } from "../../../components/zfs/replica/replicaStore";
import { approveMember, refuseMember, type GroupState, type WaitingMember } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { Facts, InstanceCard, PHONE_PATH } from "../../instances/InstanceCard";
import { RemoveInstanceButton } from "../../instances/RemoveInstanceButton";
import { STAGE_BADGE, clock, type PairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

function memberGlyph(kind: string | undefined) {
  if (kind !== "android") return <IconFleet />;
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
      <path fillRule="evenodd" d={PHONE_PATH} />
    </svg>
  );
}

/** WaitingCard lists the devices that entered the twelve words and wait for
 *  somebody here to let them in, each with its key to compare with what it
 *  shows, and says when this instance waits on another one. It stands only
 *  while there is something to decide or wait for. */
export function WaitingCard({
  group,
  onGroup,
  t,
  hueIndex,
}: {
  group: GroupState;
  onGroup: (g: GroupState) => void;
  t: T;
  hueIndex: number;
}) {
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
    <Card title={t("pairing.pendingTitle")} hint={t("pairing.pendingTip")} hueIndex={hueIndex}>
      <div data-testid="waiting" className="flex flex-col gap-3">
        {asks.length > 0 && (
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
        )}
        {there.length > 0 && <p className="text-sm text-carbon-textSub">{t("pairing.awaiting")}</p>}
        {ownKey && <SettingRow label={ownKey} hint={t("pairing.ownKeyTip")} />}
        {error && <p className="text-caption text-statusFail">{error}</p>}
      </div>
    </Card>
  );
}

function WaitRow({ text, seconds }: { text: string; seconds?: number }) {
  return (
    <div data-testid="wait-row" className="flex items-center gap-3 rounded-card bg-carbon-surface px-5 py-4 text-carbon-textSub">
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

/** MemberCards draws this instance and its pairing as the Instances tab does,
 *  one card each, with the devices that never followed into the fortress
 *  and the ones on an older version named under them. While nobody has come
 *  yet, a line says the instance waits for one with this phrase. */
export function MemberCards({
  group,
  stage,
  joinedAgo,
  waitFor,
  onRefresh,
  nextHue,
  t,
}: {
  group: GroupState;
  stage: PairStage;
  joinedAgo: number;
  /** Nobody else is in the fortress, so the wait for a first member is worth a line. */
  waitFor: boolean;
  /** Reads the group again, after a device left behind was removed. */
  onRefresh: () => void;
  nextHue: () => number;
  t: T;
}) {
  const { lang } = useT();
  const badge = stage !== "unpaired" && STAGE_BADGE[stage];
  const older = group.older ?? [];
  const behind = group.behind ?? [];
  const moving = group.oldPhrase ?? [];
  return (
    <section data-testid="members" aria-label={t("instances.title")} className="flex flex-col gap-4">
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,36rem),1fr))] gap-4">
        <InstanceCard
          cardKey="self"
          name={group.name}
          eyebrow={t("instances.thisInstance")}
          badges={
            badge && (
              <span data-testid="pair-state" className="flex">
                <Badge tone={badge.tone} size="large">
                  {t(badge.key)}
                </Badge>
              </span>
            )
          }
          index={nextHue()}
        />
        {group.members.map((m) => (
          <InstanceCard
            key={m.id}
            cardKey={m.id}
            name={m.name || m.id}
            eyebrow={m.kind === "android" ? t("fleet.kindAndroid") : undefined}
            badges={
              <Badge tone="ok" size="large">
                {t("instances.connected")}
              </Badge>
            }
            facts={<Facts items={[t(m.direct ? "pairing.direct" : "pairing.viaRelay")]} />}
            index={nextHue()}
          />
        ))}
        {group.away.map((m) => (
          <InstanceCard
            key={m.id}
            cardKey={m.id}
            name={m.name || m.id}
            eyebrow={m.kind === "android" ? t("fleet.kindAndroid") : undefined}
            badges={
              <span data-new="pairing-inactive" className="flex">
                <Badge tone="neutral" size="large">
                  {t(group.settling ? "instances.checking" : "instances.inactive")}
                </Badge>
              </span>
            }
            facts={
              <Facts
                items={[m.lastSeen > 0 && !group.settling && t("instances.lastSeen").replace("{time}", () => relativeTime(t, m.lastSeen))]}
              />
            }
            index={nextHue()}
          />
        ))}
        {behind.length > 0 && (
          <div data-testid="behind" className="contents">
            {behind.map((m) => (
              <InstanceCard
                key={m.id}
                cardKey={m.id}
                name={m.name || m.id}
                eyebrow={t(m.kind === "android" ? "fleet.kindAndroid" : "pairing.kindInstance")}
                badges={
                  <Badge tone="warn" size="large">
                    {t("pairing.behindTitle")}
                    <InfoBubble tip={t("pairing.behindTip")} onAccent />
                  </Badge>
                }
                actions={<RemoveInstanceButton instance={{ memberId: m.id, name: m.name || m.id }} onRemoved={onRefresh} />}
                index={nextHue()}
              />
            ))}
          </div>
        )}
      </div>
      {moving.length > 0 && (
        <p className="text-sm text-carbon-textSub">{t("pairing.oldPhrase").replace("{names}", () => listOf(lang, moving))}</p>
      )}
      {older.length > 0 && (
        <p data-testid="older" className="text-sm text-statusWarn">
          {t("pairing.older").replace("{names}", () => older.join(", "))}
        </p>
      )}
      {waitFor && stage === "new" && <WaitRow text={t("pairing.waitNext")} />}
      {waitFor && (stage === "searching" || stage === "alone" || stage === "gone") && (
        <WaitRow text={t("pairing.waitPhrase")} seconds={stage === "searching" ? joinedAgo : undefined} />
      )}
    </section>
  );
}

/** ZfsCard connects a ZFS server that runs no BombVault, over SSH. */
export function ZfsCard({ t, hueIndex }: { t: T; hueIndex: number }) {
  const { servers, reload } = useReplicaServers();
  const [adding, setAdding] = useState(false);
  return (
    <Card title={t("pairing.group.zfsTitle")} hint={t("pairing.group.zfsTip")} hueIndex={hueIndex}>
      <div data-testid="zfs-row">
        <SettingRow
          label={servers.length > 0 ? servers.map((s) => s.name).join(", ") : t("pairing.group.zfsNone")}
          glyph={
            <span className="text-accentText [&>svg]:h-4.5 [&>svg]:w-4.5">
              <IconZFS />
            </span>
          }
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
