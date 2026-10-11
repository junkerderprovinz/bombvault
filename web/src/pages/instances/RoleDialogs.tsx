// Switching a role on an instance's card or in its window. Off asks: a
// dialog collects what the request covers and sends it, and the tile then
// waits for the other side. On withdraws, after a question that says what
// stops.
import { useState, type ReactNode } from "react";

import { Button } from "../../components/Button";
import { KindGlyph } from "../../components/backups/kinds";
import { IconPlay, IconSave, IconUpload } from "../../components/glyphs";
import { Selector } from "../../components/Selector";
import { SettingRow } from "../../components/storage/rows";
import { useReplicaServers } from "../../components/zfs/replica/replicaStore";
import { askRole, patchZFSReplica, withdrawKeeper, withdrawRole } from "../../lib/api";
import type { RequestedRole, RoleSection, RoleStore, ZFSReplicaTarget } from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { depositSentence } from "../../lib/depositCodes";
import { useConfirm } from "../../lib/useConfirm";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { ToggleRow } from "../settings/shared";
import { KeeperDialog } from "./KeeperDialogs";
import { domainLabelKey } from "./PeerScorecard";
import { RoleDialog } from "./RoleDialog";
import { ROLES } from "./RoleTile";
import { roleStates, type Instance, type ItemReplica, type RoleKind, type RoleState } from "./instancesModel";
import type { InstancesState } from "./useInstances";

const SECTION_KIND: Record<RoleSection, EntryKind> = {
  containers: "container",
  vms: "vm",
  files: "files",
  flash: "flash",
  zfs: "zfs",
  config: "config",
};

/** RoleAskDialog asks an instance to be this server's Receiver or Fetcher,
 *  or changes the sections of a request already sent. An instance too old to
 *  answer takes the role at once, and the dialog says so. */
export function RoleAskDialog({
  instance,
  role,
  sections,
  onClose,
  onSent,
}: {
  instance: Instance;
  role: RequestedRole;
  /** The sections this server backs up. */
  sections: RoleSection[];
  onClose: () => void;
  onSent: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const sent = instance.roles?.byMember[role];
  const editing = sent?.state === "asked" || sent?.state === "allowed";
  const answers = instance.roles?.answers ?? true;
  // A receiver takes ordinary backups. ZFS data reaches another instance as a
  // replica, so it starts unpicked.
  const [picked, setPicked] = useState<RoleSection[]>(() =>
    editing ? sent.sections : role === "receiver" ? sections.filter((s) => s !== "zfs") : sections.slice(0, 1),
  );
  const [store, setStore] = useState<RoleStore>(sent?.store === "share" ? "share" : "rest");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const named = (key: TranslationKey) => t(key).replaceAll("{peer}", () => instance.name);
  const title = named("instances.role.asTitle").replace("{role}", t(ROLES[role].label));
  // An instance too old to answer has its receiving server and nothing else.
  const picksStore = role === "receiver" && answers;

  async function send() {
    if (picked.length === 0) return setError(t("instances.role.needSection"));
    setError(null);
    setBusy(true);
    try {
      const res = await askRole(instance.memberId, role, picksStore ? { sections: picked, store } : { sections: picked });
      if (!res.ok) {
        setError(res.error ?? t("instances.role.saveError"));
        return;
      }
      const done = editing
        ? t("folders.saved")
        : res.request?.state === "allowed"
          ? named("instances.role.activeNow").replace("{role}", t(ROLES[role].label))
          : named("instances.role.asked");
      push(done, "success");
      onSent();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("instances.role.saveError"));
    } finally {
      setBusy(false);
    }
  }

  const sendButton: { key: TranslationKey; glyph: ReactNode } = editing
    ? { key: "settings.save", glyph: <IconSave /> }
    : answers
      ? { key: "instances.role.send", glyph: <IconUpload /> }
      : { key: "instances.role.switchOn", glyph: <IconPlay /> };
  return (
    <RoleDialog
      title={title}
      hint={t(ROLES[role].tip)}
      lead={named(answers ? "instances.role.askLead" : "instances.role.oldLead")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={busy} />
          <Button
            label={t(sendButton.key)}
            labelKey={sendButton.key}
            glyph={sendButton.glyph}
            tone="accent"
            onClick={() => void send()}
            disabled={busy}
            busy={busy}
          />
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <span className="text-subline font-semibold text-carbon-textSub">
          {t(role === "receiver" ? "instances.role.sections" : "instances.role.sectionsFetch")}
        </span>
        {sections.map((s, i) => (
          <div key={s} className="flex items-center gap-2.5">
            <span className="shrink-0 text-accentText">
              <KindGlyph kind={SECTION_KIND[s]} />
            </span>
            <div className="min-w-0 flex-1">
              <ToggleRow
                label={t(domainLabelKey(s))}
                checked={picked.includes(s)}
                onChange={(on) => setPicked((prev) => (on ? [...prev, s] : prev.filter((x) => x !== s)))}
                hueIndex={i}
              />
            </div>
          </div>
        ))}
      </div>
      {picksStore && (
        <SettingRow
          label={named("instances.role.where")}
          hint={t("instances.role.whereTip")}
          note={store === "rest" ? t("instances.role.storeRestNote") : undefined}
        >
          <Selector
            items={[
              { id: "rest", label: named("instances.role.storeRest") },
              { id: "share", label: t("rcloneRemote.share") },
            ]}
            label={named("instances.role.where")}
            select="one"
            activation="manual"
            inline
            pairsOnPhone
            active={store}
            onChange={(id) => setStore(id as RoleStore)}
            // What was allowed was allowed for the place it named.
            disabled={sent?.state === "allowed"}
          />
        </SettingRow>
      )}
      {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}
    </RoleDialog>
  );
}

/** ZFSRoleDialog picks which of this server's ZFS entries replicate to an
 *  instance. Each entry asks on its own, so the dialog only points entries
 *  at the instance or away from it. */
export function ZFSRoleDialog({
  instance,
  replicas,
  peers,
  onClose,
  onSent,
}: {
  instance: Instance;
  /** Every ZFS entry of this server. */
  replicas: ItemReplica[];
  /** The instances another entry may already replicate to, for their names. */
  peers: Instance[];
  onClose: () => void;
  onSent: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { servers } = useReplicaServers();
  const here = (target: ZFSReplicaTarget) => target.kind === "peer" && target.id === instance.memberId;
  const [picked, setPicked] = useState<string[]>(() => {
    const mine = replicas.filter((r) => here(r.replica.target));
    return (mine.length > 0 ? mine : replicas).map((r) => r.itemId);
  });
  const [busy, setBusy] = useState(false);
  const answers = instance.roles?.answers ?? true;
  const named = (key: TranslationKey) => t(key).replaceAll("{peer}", () => instance.name);

  function elsewhere(target: ZFSReplicaTarget): string | undefined {
    if (target.kind === "none" || here(target)) return undefined;
    const name =
      target.kind === "server" ? servers.find((s) => s.id === target.id)?.name : peers.find((p) => p.memberId === target.id)?.name;
    return name && t("instances.role.zfsElsewhere").replace("{target}", () => name);
  }

  async function send() {
    setBusy(true);
    try {
      const changes = replicas.flatMap((r) => {
        const wanted = picked.includes(r.itemId);
        if (wanted === here(r.replica.target)) return [];
        const target: ZFSReplicaTarget = wanted ? { kind: "peer", id: instance.memberId } : { kind: "none", id: "" };
        return [patchZFSReplica(r.itemId, { target })];
      });
      const refused = (await Promise.all(changes)).find((res) => !res.ok);
      if (refused) push(refused.code ? zfsCodeSentence(t, refused.code) : (refused.error ?? t("instances.role.saveError")), "fail");
      else if (picked.length > 0) push(named("instances.role.asked"), "success");
      onSent();
    } catch (err) {
      push(err instanceof Error ? err.message : t("instances.role.saveError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  const usable = answers && replicas.length > 0;
  return (
    <RoleDialog
      title={named("instances.role.asTitle").replace("{role}", t(ROLES.zfs.label))}
      hint={t(ROLES.zfs.tip)}
      lead={!answers ? named("instances.role.oldZfs") : replicas.length === 0 ? t("instances.role.zfsNone") : named("instances.role.askLeadZfs")}
      onClose={onClose}
      footer={
        usable ? (
          <>
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={busy} />
            <Button
              label={t("instances.role.send")}
              labelKey="instances.role.send"
              tone="accent"
              onClick={() => void send()}
              disabled={busy}
              busy={busy}
            />
          </>
        ) : (
          <Button label={t("common.close")} labelKey="common.close" tone="accent" onClick={onClose} />
        )
      }
    >
      {usable && (
        <div className="flex flex-col gap-3">
          <span className="text-subline font-semibold text-carbon-textSub">{t("instances.role.zfsEntries")}</span>
          {replicas.map((r, i) => (
            <div key={r.itemId} className="flex flex-col gap-0.5">
              <ToggleRow
                label={r.dataset}
                checked={picked.includes(r.itemId)}
                onChange={(on) => setPicked((prev) => (on ? [...prev, r.itemId] : prev.filter((x) => x !== r.itemId)))}
                hueIndex={i}
              />
              {elsewhere(r.replica.target) && <span className="text-xs text-carbon-textMuted">{elsewhere(r.replica.target)}</span>}
            </div>
          ))}
        </div>
      )}
    </RoleDialog>
  );
}

const OFF_TEXT: Record<RoleKind, TranslationKey> = {
  receiver: "instances.role.offReceiver",
  fetcher: "instances.role.offFetcher",
  zfs: "instances.role.offZfs",
  keeper: "instances.role.offKeeper",
};

/** useRoleSwitch is what the role controls of one instance do. The caller
 *  draws `dialogs` once and wires `toggle` and `change` to its tiles or
 *  switches. */
export function useRoleSwitch({
  instance,
  peers,
  sections,
  state,
}: {
  instance: Instance;
  peers: Instance[];
  sections: RoleSection[];
  state: InstancesState;
}): { states: Record<RoleKind, RoleState>; toggle: (role: RoleKind) => void; change: (role: RoleKind) => void; dialogs: ReactNode } {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [asking, setAsking] = useState<RoleKind | null>(null);
  const states = roleStates(instance);

  async function switchOff(role: RoleKind) {
    // The last keeper takes the recovery code with it.
    const lastKeeper = role === "keeper" && state.deposits.sent.every((d) => d.memberId === instance.memberId);
    const question = [
      t("instances.role.offQuestion").replace("{role}", t(ROLES[role].label)),
      t(OFF_TEXT[role]),
      lastKeeper && t("keeper.offLast"),
    ]
      .filter(Boolean)
      .join(" ");
    if (!(await confirm(question.replaceAll("{peer}", () => instance.name), { confirmKey: "instances.role.switchOff" }))) return;
    try {
      if (role === "keeper") {
        const res = await withdrawKeeper(instance.memberId);
        if (!res.ok) push(depositSentence(t, res), "fail");
        // A keeper that did not hear still holds the deposit.
        else if (!res.told) push(t("keeper.notTold").replaceAll("{peer}", () => instance.name), "warn");
        await state.reloadDeposits();
        return;
      }
      if (role === "zfs") {
        const answers = await Promise.all(instance.replicas.map((r) => patchZFSReplica(r.itemId, { target: { kind: "none", id: "" } })));
        const refused = answers.find((res) => !res.ok);
        if (refused) push(refused.code ? zfsCodeSentence(t, refused.code) : (refused.error ?? t("instances.role.saveError")), "fail");
        await state.reloadReplicas();
      } else {
        const res = await withdrawRole(instance.memberId, role);
        if (!res.ok) push(res.error ?? t("instances.role.saveError"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("instances.role.saveError"), "fail");
    }
    await state.reloadRoles();
  }

  function sent() {
    setAsking(null);
    void (asking === "keeper" ? state.reloadDeposits() : state.reloadRoles());
    if (asking === "zfs") void state.reloadReplicas();
  }

  const dialogs = (
    <>
      {confirmDialog}
      {asking === "zfs" && (
        <ZFSRoleDialog instance={instance} replicas={state.data.replicas} peers={peers} onClose={() => setAsking(null)} onSent={sent} />
      )}
      {asking === "keeper" && (
        <KeeperDialog instance={instance} hasCode={state.deposits.codeSince !== ""} onClose={() => setAsking(null)} onDone={sent} />
      )}
      {(asking === "receiver" || asking === "fetcher") && (
        <RoleAskDialog instance={instance} role={asking} sections={sections} onClose={() => setAsking(null)} onSent={sent} />
      )}
    </>
  );
  return {
    states,
    toggle: (role) => (states[role] === "off" ? setAsking(role) : void switchOff(role)),
    change: setAsking,
    dialogs,
  };
}
