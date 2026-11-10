// SharedRepoCard is the shared repository this instance holds for its group,
// beside its receiving server: setting it up, how it stands, the asks to join
// and the members that copy into it. Only this instance prunes it, so a
// member can add snapshots there and delete none.

import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import {
  checkSharedRepo,
  copyIntoOwnSharedRepo,
  decideSharedJoin,
  forgetSharedFormer,
  getReceiverServer,
  removeSharedMember,
  setUpSharedRepo,
  type SharedFormer,
  type SharedJoinAsk,
  type SharedMember,
  type SharedServer,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { FolderBrowser } from "../../components/FolderBrowser";
import { IconConfirm, IconLink } from "../../components/glyphs";
import { IconTabStorage } from "../../components/navGlyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { Rows, SettingRow } from "../../components/storage/rows";
import { Card, CardFoot } from "../settings/shared";
import { InstanceWindow } from "./InstanceWindow";
import { checkBadge } from "./ReceiverServerWindow";
import type { SharedRepoState } from "./useSharedRepo";

type T = ReturnType<typeof useT>["t"];

/** While a server's backups are being forgotten, the card asks again this often. */
const FORGETTING_POLL_MS = 5000;

const INPUT = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

function memberNote(m: SharedMember, t: T): string {
  const keys = m.keys?.length ?? 0;
  return [
    m.host,
    keys > 0 && t("shared.keys", keys),
    m.lastSnapshotAt ? t("shared.lastBackup").replace("{time}", () => relativeTime(t, m.lastSnapshotAt!)) : t("shared.noBackup"),
  ]
    .filter(Boolean)
    .join(" · ");
}

function formerNote(f: SharedFormer, t: T): string {
  return [
    f.host,
    f.snapshots > 0 && t("shared.former.backups", f.snapshots),
    t("shared.former.left").replace("{time}", () => relativeTime(t, f.leftAt)),
  ]
    .filter(Boolean)
    .join(" · ");
}

export function SharedRepoCard({ shared, hueIndex }: { shared: SharedRepoState; hueIndex: number }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [formOpen, setFormOpen] = useState(false);
  const [checking, setChecking] = useState(false);
  const [busy, setBusy] = useState("");
  const navigate = useNavigate();
  const ownLogin = useOwnLogin();
  const { server, error, reload } = shared;

  const forgetting = server?.former.some((f) => f.forgetting) ?? false;
  useEffect(() => {
    if (!forgetting) return;
    const id = setInterval(() => void reload(), FORGETTING_POLL_MS);
    return () => clearInterval(id);
  }, [forgetting, reload]);

  async function runCheck() {
    setChecking(true);
    try {
      const r = await checkSharedRepo();
      if (r.ok && r.server) shared.setServer(r.server);
      else push(r.error ?? t("shared.loadError"), "fail");
    } catch (e) {
      push(e instanceof Error ? e.message : t("shared.loadError"), "fail");
    } finally {
      setChecking(false);
    }
  }

  // An answer that did not reach the member says so, since the member keeps
  // waiting until it asks again.
  async function change(id: string, call: () => ReturnType<typeof removeSharedMember>, done: string, name: string) {
    setBusy(id);
    try {
      const r = await call();
      if (!r.ok || !r.server) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      shared.setServer(r.server);
      push(done, "success");
      if (r.told === false) push(t("shared.notTold").replace("{name}", () => name), "warn");
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    } finally {
      setBusy("");
    }
  }

  const named = (key: TranslationKey, name: string) => t(key).replaceAll("{name}", () => name);

  const decide = (ask: SharedJoinAsk, decision: "allow" | "refuse") =>
    change(
      `${ask.memberId}:${decision}`,
      () => decideSharedJoin(ask.memberId, decision),
      named(decision === "allow" ? "shared.allowed" : "shared.refused", ask.name),
      ask.name,
    );

  async function copyIn() {
    setBusy("own");
    try {
      const r = await copyIntoOwnSharedRepo();
      if (!r.ok || !r.server) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      shared.setServer(r.server);
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    } finally {
      setBusy("");
    }
  }

  async function forget(id: string, f: SharedFormer) {
    const yes = await confirm(named("shared.former.ask", f.name), {
      confirmLabel: t("shared.former.forget"),
      confirmLabelKey: "shared.former.forget",
      confirmTone: "neutral",
      requireText: f.name,
      requirePrompt: named("offsiteRemoval.typeName", f.name),
    });
    if (!yes) return;
    setBusy(`${f.host}:forget`);
    try {
      const r = await forgetSharedFormer(id, f.host, f.name);
      if (ownLogin.handled(r)) return;
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      push(named("shared.former.started", f.name), "success");
      await reload();
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    } finally {
      setBusy("");
    }
  }

  async function remove(m: SharedMember) {
    if (!(await confirm(named("shared.removeAsk", m.name), { confirmKey: "shared.remove" }))) return;
    await change(`${m.memberId}:remove`, () => removeSharedMember(m.memberId), named("shared.removed", m.name), m.name);
  }

  return (
    <Card title={t("shared.title")} hint={t("shared.hint")} hueIndex={hueIndex} newId="shared-repo">
      {error !== null && <p className="text-sm text-statusFail wrap-break-word">{error || t("shared.loadError")}</p>}

      {shared.loaded && !server && (
        <>
          <p className="text-sm text-carbon-textSub">{t("shared.intro")}</p>
          <CardFoot>
            <Button label={t("shared.setUp")} labelKey="shared.setUp" tone="accent" onClick={() => setFormOpen(true)} />
          </CardFoot>
        </>
      )}

      {server && (
        <>
          <div className="flex flex-wrap items-center gap-2">
            {checkBadge(server, t)}
            {!server.present && (
              <Badge tone="fail">
                {t("receiver.server.gone")}
                <InfoBubble tip={t("shared.goneTip")} onAccent />
              </Badge>
            )}
            {server.checkedAt > 0 && (
              <span className="text-xs text-carbon-textMuted">
                {t("receiver.lastChecked").replace("{time}", () => relativeTime(t, server.checkedAt))}
              </span>
            )}
            <Button
              label={t("receiver.server.checkAgain")}
              labelKey="receiver.server.checkAgain"
              tone="neutral"
              onClick={() => void runCheck()}
              busy={checking}
              disabled={checking}
              className="ms-auto"
            />
          </div>
          {server.check !== "protected" && server.checkDetail && (
            <p className="text-xs text-carbon-textSub wrap-break-word">{server.checkDetail}</p>
          )}
          <Rows>
            <SettingRow label={t("receiver.server.address")}>
              <bdi dir="ltr" className="font-mono text-xs text-carbon-text break-all text-start">
                {server.url || t(server.needsHost ? "shared.needsHost" : "receiver.server.noAddress")}
              </bdi>
            </SettingRow>
            <SettingRow label={t("shared.folder")}>
              <bdi dir="ltr" className="font-mono text-xs text-carbon-text break-all text-start">{server.hostPath}</bdi>
            </SettingRow>
          </Rows>
          <p className="text-xs text-carbon-textMuted">
            {t("shared.prunes")} {t("shared.readable")}
          </p>

          {server.joins.map((ask) => (
            <div
              key={ask.memberId}
              data-join={ask.memberId}
              className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-carbon-background p-3"
            >
              <span className="grid h-9 w-9 shrink-0 place-items-center rounded-control bg-carbon-surface3 text-accentText">
                <IconLink />
              </span>
              <div className="flex min-w-0 flex-[1_1_14rem] flex-col gap-0.5">
                <span className="flex flex-wrap items-center gap-2 text-sm font-semibold text-carbon-text">
                  {ask.name}
                  {ask.state === "asked" && <Badge tone="warn">{t("instances.request")}</Badge>}
                </span>
                <span className="text-xs text-carbon-textMuted wrap-anywhere">
                  {named(ask.state === "asked" ? "shared.joinAsk" : "shared.joinAllowed", ask.name)}
                  {" · "}
                  {relativeTime(t, ask.askedAt)}
                </span>
                {ask.expiresAt ? (
                  <span className="text-xs text-carbon-textMuted wrap-anywhere">
                    {named("shared.joinExpires", ask.name).replace("{time}", () => formatTs(ask.expiresAt!))}
                  </span>
                ) : null}
              </div>
              <div className="ms-auto flex flex-wrap items-center justify-end gap-2">
                {ask.state === "asked" ? (
                  <>
                    <Button
                      label={t("zfs.receive.decline")}
                      labelKey="zfs.receive.decline"
                      tone="neutral"
                      onClick={() => void decide(ask, "refuse")}
                      disabled={busy !== ""}
                      busy={busy === `${ask.memberId}:refuse`}
                    />
                    <Button
                      label={t("zfs.receive.allow")}
                      labelKey="zfs.receive.allow"
                      glyph={<IconConfirm />}
                      tone="accent"
                      onClick={() => void decide(ask, "allow")}
                      disabled={busy !== ""}
                      busy={busy === `${ask.memberId}:allow`}
                    />
                  </>
                ) : (
                  <Button
                    label={t("shared.tellAgain")}
                    labelKey="shared.tellAgain"
                    tone="neutral"
                    onClick={() => void decide(ask, "allow")}
                    disabled={busy !== ""}
                    busy={busy === `${ask.memberId}:allow`}
                  />
                )}
              </div>
            </div>
          ))}

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("shared.members")}
              <InfoBubble tip={t("shared.membersTip")} />
            </span>
            {server.members.length === 0 && <p className="text-xs text-carbon-textMuted">{t("shared.noMembers")}</p>}
            {server.members.length > 0 && (
              <Rows as="ul">
                {server.members.map((m) => (
                  <SettingRow
                    key={m.memberId}
                    as="li"
                    label={m.name}
                    note={<bdi dir="ltr">{memberNote(m, t)}</bdi>}
                  >
                    {m.self ? (
                      <span className="text-xs text-carbon-textMuted">{t("shared.own.self")}</span>
                    ) : (
                      <Button
                        label={t("shared.remove")}
                        labelKey="shared.remove"
                        tone="neutral"
                        onClick={() => void remove(m)}
                        disabled={busy !== ""}
                        busy={busy === `${m.memberId}:remove`}
                      />
                    )}
                  </SettingRow>
                ))}
              </Rows>
            )}
          </div>

          {server.former.length > 0 && (
            <div className="flex flex-col gap-1.5">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("shared.former")}
                <InfoBubble tip={t("shared.formerTip")} />
              </span>
              <Rows as="ul">
                {server.former.map((f) => (
                  <SettingRow
                    key={f.host}
                    as="li"
                    label={f.name}
                    note={
                      <>
                        <bdi dir="ltr">{formerNote(f, t)}</bdi>
                        {f.forgetError && (
                          <span className="text-statusFail">{t("shared.former.failed").replace("{error}", () => f.forgetError ?? "")}</span>
                        )}
                      </>
                    }
                  >
                    <Button
                      label={t("shared.former.forget")}
                      labelKey="shared.former.forget"
                      tone="neutral"
                      onClick={() => void forget(server.id, f)}
                      disabled={busy !== "" || f.forgetting}
                      busy={f.forgetting || busy === `${f.host}:forget`}
                    />
                  </SettingRow>
                ))}
              </Rows>
            </div>
          )}

          <div data-new="shared-repo-own" className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-carbon-background p-3">
            <p className="min-w-0 flex-[1_1_16rem] text-sm text-carbon-text">
              {t(server.copiesIn ? "shared.own.on" : "shared.own.offer")}
            </p>
            {server.copiesIn ? (
              <Button
                label={t("shared.member.openStorage")}
                labelKey="shared.member.openStorage"
                glyph={<IconTabStorage />}
                tone="neutral"
                onClick={() => navigate("/storage")}
              />
            ) : (
              <Button
                label={t("shared.own.start")}
                labelKey="shared.own.start"
                hint={t("shared.own.tip")}
                tone="accent"
                onClick={() => void copyIn()}
                disabled={busy !== ""}
                busy={busy === "own"}
              />
            )}
          </div>
        </>
      )}

      {confirmDialog}
      {ownLogin.dialog}
      {formOpen && (
        <SharedRepoDialog
          t={t}
          defaultPort={shared.defaultPort}
          onClose={() => setFormOpen(false)}
          onDone={(s) => {
            shared.setServer(s);
            setFormOpen(false);
          }}
        />
      )}
    </Card>
  );
}

function SharedRepoDialog({
  t,
  defaultPort,
  onClose,
  onDone,
}: {
  t: T;
  defaultPort: number;
  onClose: () => void;
  onDone: (server: SharedServer) => void;
}) {
  const { push } = useToast();
  const [folder, setFolder] = useState("");
  const [port, setPort] = useState(defaultPort);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [saving, setSaving] = useState(false);

  // The folder picker starts where the receiving server's did.
  useEffect(() => {
    let alive = true;
    getReceiverServer()
      .then((r) => alive && r.ok && r.hostMountRoot && setHostMountRoot(r.hostMountRoot))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const canSave = folder.trim() !== "" && Number.isFinite(port) && !saving;

  async function submit() {
    setSaving(true);
    try {
      const r = await setUpSharedRepo({ folder: folder.trim(), port });
      if (!r.ok || !r.server) {
        push(r.code === "own-data-folder" ? t("runReason.ownDataFolder") : (r.error ?? t("shared.setUpFailed")), "fail");
        return;
      }
      onDone(r.server);
    } catch (e) {
      push(e instanceof Error ? e.message : t("shared.setUpFailed"), "fail");
    } finally {
      setSaving(false);
    }
  }

  return (
    <InstanceWindow
      title={t("shared.setUp")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={saving} />
          <Button
            label={t("receiver.server.start")}
            labelKey="receiver.server.start"
            tone="accent"
            onClick={() => void submit()}
            disabled={!canSave}
            busy={saving}
          />
        </>
      }
    >
      <p className="text-xs text-carbon-textSub">{t("shared.formIntro")}</p>
      <FolderBrowser
        label={t("shared.folder")}
        hint={t("shared.folderTip")}
        value={folder}
        hostMountRoot={hostMountRoot}
        onChange={setFolder}
        placeholder="user/restic-shared"
        inDialog
      />
      <Rows>
        <SettingRow label={t("receiver.server.port")} hint={t("shared.portTip")} htmlFor="bv-shared-repo-port">
          <NumberField
            id="bv-shared-repo-port"
            wrapperClassName="w-28"
            min={1}
            max={65535}
            value={port}
            onChange={(e) => setPort(parseInt(e.target.value, 10))}
            className={`${INPUT} w-full`}
          />
        </SettingRow>
      </Rows>
    </InstanceWindow>
  );
}
