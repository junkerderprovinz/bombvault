// ReceiverServerWindow sets up the append-only rest-server this box runs for
// its pairing group, and shows how it stands once it runs: where members copy
// to, whether it still refuses deletes, which members have a login of their
// own, and the Unraid template that keeps it editable in the Docker tab.

import { useEffect, useState } from "react";
import {
  RECEIVER_TEMPLATE_URL,
  checkReceiverServer,
  forgetReceiverServer,
  getReceiverServer,
  revokeReceiverLogin,
  setUpReceiverServer,
  type ReceiverLogin,
  type ReceiverServer,
} from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { CopyBlock } from "../../components/CopyBlock";
import { FolderBrowser } from "../../components/FolderBrowser";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { Rows, SettingRow } from "../../components/storage/rows";
import { InstanceWindow } from "./InstanceWindow";

type T = ReturnType<typeof useT>["t"];

const INPUT = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** What setting up answered, kept only until the window is closed: the
 *  password is not shown again. */
interface Fresh {
  password: string;
  template: "written" | "download" | "none";
}

function checkBadge(server: ReceiverServer, t: T) {
  switch (server.check) {
    case "protected":
      return <Badge tone="ok">{t("receiver.server.protected")}</Badge>;
    case "unprotected":
      return <Badge tone="fail">{t("receiver.server.unprotected")}</Badge>;
    case "inconclusive":
      return <Badge tone="warn">{t("receiver.server.inconclusive")}</Badge>;
    default:
      return <Badge tone="neutral">{t("receiver.server.unchecked")}</Badge>;
  }
}

export function ReceiverServerWindow({
  t,
  lead,
  onClose,
}: {
  t: T;
  /** Why the window opened by itself. With it, setting up starts at once on
   *  a box that runs no receiving server yet. */
  lead?: string;
  /** Says whether a receiving server runs when the window closes. */
  onClose: (running: boolean) => void;
}) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [server, setServer] = useState<ReceiverServer | null>(null);
  const [defaults, setDefaults] = useState({ port: 8000, hostMountRoot: "/host/user" });
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [fresh, setFresh] = useState<Fresh | null>(null);
  const [checking, setChecking] = useState(false);

  useEffect(() => {
    let alive = true;
    getReceiverServer()
      .then((r) => {
        if (!alive) return;
        if (!r.ok) {
          setError(r.error ?? t("receiver.server.loadError"));
          return;
        }
        setServer(r.server ?? null);
        setDefaults({
          port: r.defaultPort ?? 8000,
          hostMountRoot: r.hostMountRoot ?? "/host/user",
        });
        if (lead && !r.server) setFormOpen(true);
      })
      .catch((e: unknown) => alive && setError(e instanceof Error ? e.message : t("receiver.server.loadError")))
      .finally(() => alive && setLoaded(true));
    return () => {
      alive = false;
    };
  }, [t, lead]);

  const close = () => onClose(server !== null);
  const offerSetUp = loaded && !server && !error;

  async function runCheck() {
    setChecking(true);
    try {
      const r = await checkReceiverServer();
      if (r.ok && r.server) setServer(r.server);
      else push(r.error ?? t("receiver.server.loadError"), "fail");
    } catch (e) {
      push(e instanceof Error ? e.message : t("receiver.server.loadError"), "fail");
    } finally {
      setChecking(false);
    }
  }

  async function forget() {
    if (!(await confirm(t("receiver.server.forgetAsk"), { confirmKey: "receiver.server.forget" }))) return;
    try {
      const r = await forgetReceiverServer();
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      setServer(null);
      setFresh(null);
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    }
  }

  async function revoke(login: ReceiverLogin) {
    const ask = t("receiver.server.revokeAsk").replace("{name}", () => login.name);
    if (!(await confirm(ask, { confirmKey: "receiver.server.revoke" }))) return;
    try {
      const r = await revokeReceiverLogin(login.memberId);
      if (r.ok && r.server) setServer(r.server);
      else push(r.error ?? t("settings.error"), "fail");
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    }
  }

  return (
    <InstanceWindow
      title={t("receiver.server.title")}
      hint={t("receiver.server.tip")}
      onClose={close}
      footer={
        <>
          {server && (
            <>
              <a href={RECEIVER_TEMPLATE_URL} download className="me-auto text-xs text-carbon-textSub underline">
                {t("receiver.server.downloadTemplate")}
              </a>
              <Button
                label={t("receiver.server.forget")}
                labelKey="receiver.server.forget"
                tone="neutral"
                hint={t("receiver.server.forgetTip")}
                onClick={() => void forget()}
              />
            </>
          )}
          <Button label={t("common.done")} labelKey="common.done" tone={offerSetUp ? "neutral" : "accent"} onClick={close} />
          {offerSetUp && (
            <Button
              label={t("receiver.server.setUp")}
              labelKey="receiver.server.setUp"
              tone="accent"
              onClick={() => setFormOpen(true)}
            />
          )}
        </>
      }
    >
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {offerSetUp && <p className="text-sm text-carbon-textSub">{t("receiver.server.intro")}</p>}

      {server && (
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            {checkBadge(server, t)}
            {!server.present && (
              <Badge tone="fail">
                {t("receiver.server.gone")}
                <InfoBubble tip={t("receiver.server.goneTip")} onAccent />
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
                {server.url || t(server.needsHost ? "receiver.server.needsHost" : "receiver.server.noAddress")}
              </bdi>
            </SettingRow>
            <SettingRow label={t("receiver.server.outsideUser")}>
              <bdi dir="ltr" className="font-mono text-xs text-carbon-text text-start">{server.user}</bdi>
            </SettingRow>
            <SettingRow label={t("receiver.server.folder")}>
              <bdi dir="ltr" className="font-mono text-xs text-carbon-text break-all text-start">{server.hostPath}</bdi>
            </SettingRow>
          </Rows>
          <p className="text-xs text-carbon-textMuted">{t("receiver.server.offered")}</p>

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("receiver.server.logins")}
              <InfoBubble tip={t("receiver.server.loginsTip")} />
            </span>
            {server.logins.length === 0 && <p className="text-xs text-carbon-textMuted">{t("receiver.server.noLogins")}</p>}
            {server.logins.length > 0 && (
              <Rows as="ul">
                {server.logins.map((l) => (
                  <SettingRow
                    key={l.memberId}
                    as="li"
                    label={l.name}
                    note={
                      <span>
                        <bdi dir="ltr" className="font-mono">{l.user}</bdi> · {new Date(l.createdAt * 1000).toLocaleDateString()}
                      </span>
                    }
                  >
                    <Button
                      label={t("receiver.server.revoke")}
                      labelKey="receiver.server.revoke"
                      tone="neutral"
                      onClick={() => void revoke(l)}
                    />
                  </SettingRow>
                ))}
              </Rows>
            )}
          </div>

          {fresh && (
            <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("receiver.server.password")}
                <InfoBubble tip={t("receiver.server.passwordTip")} />
              </span>
              <CopyBlock text={fresh.password} />
              {fresh.template === "written" && (
                <p className="text-xs text-statusOk">{t("receiver.server.templateWritten")}</p>
              )}
              {fresh.template === "download" && (
                <p className="text-xs text-statusWarn">{t("receiver.server.templateDownload")}</p>
              )}
            </div>
          )}
        </div>
      )}

      {confirmDialog}
      {formOpen && (
        <ReceiverServerDialog
          t={t}
          lead={lead}
          defaults={defaults}
          onClose={() => setFormOpen(false)}
          onDone={(s, f) => {
            setServer(s);
            setFresh(f);
            setFormOpen(false);
          }}
        />
      )}
    </InstanceWindow>
  );
}

function ReceiverServerDialog({
  t,
  lead,
  defaults,
  onClose,
  onDone,
}: {
  t: T;
  lead?: string;
  defaults: { port: number; hostMountRoot: string };
  onClose: () => void;
  onDone: (server: ReceiverServer, fresh: Fresh) => void;
}) {
  const { push } = useToast();
  const [folder, setFolder] = useState("");
  const [port, setPort] = useState(defaults.port);
  const [host, setHost] = useState("");
  const [saving, setSaving] = useState(false);

  const canSave = folder.trim() !== "" && Number.isFinite(port) && !saving;

  async function submit() {
    setSaving(true);
    try {
      const r = await setUpReceiverServer({ folder: folder.trim(), port, host: host.trim() });
      if (!r.ok || !r.server) {
        push(r.code === "own-data-folder" ? t("runReason.ownDataFolder") : (r.error ?? t("receiver.server.setUpFailed")), "fail");
        return;
      }
      onDone(r.server, { password: r.password ?? "", template: r.template ?? "none" });
    } catch (e) {
      push(e instanceof Error ? e.message : t("receiver.server.setUpFailed"), "fail");
    } finally {
      setSaving(false);
    }
  }

  return (
    <InstanceWindow
      title={t("receiver.server.setUp")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={saving} />
          <Button
            label={t(lead ? "receiver.server.startContinue" : "receiver.server.start")}
            labelKey={lead ? "receiver.server.startContinue" : "receiver.server.start"}
            tone="accent"
            onClick={() => void submit()}
            disabled={!canSave}
            busy={saving}
          />
        </>
      }
    >
      {lead && <p className="text-sm text-carbon-text">{lead}</p>}
      <p className="text-xs text-carbon-textSub">{t("receiver.server.formIntro")}</p>
      <FolderBrowser
        label={t("receiver.server.folder")}
        hint={t("receiver.server.folderTip")}
        value={folder}
        hostMountRoot={defaults.hostMountRoot}
        onChange={setFolder}
        placeholder="user/restic"
        inDialog
      />
      <Rows>
        <SettingRow label={t("receiver.server.port")} hint={t("receiver.server.portTip")} htmlFor="bv-receiver-server-port">
          <NumberField
            id="bv-receiver-server-port"
            wrapperClassName="w-28"
            min={1}
            max={65535}
            value={port}
            onChange={(e) => setPort(parseInt(e.target.value, 10))}
            className={`${INPUT} w-full`}
          />
        </SettingRow>
        <SettingRow label={t("receiver.server.host")} hint={t("receiver.server.hostTip")} htmlFor="bv-receiver-server-host">
          <input
            id="bv-receiver-server-host"
            type="text"
            value={host}
            onChange={(e) => setHost(e.target.value)}
            placeholder={t("receiver.server.hostPlaceholder")}
            spellCheck={false}
            autoComplete="off"
            dir="ltr"
            className={`${INPUT} w-72 max-w-full font-mono text-start`}
          />
        </SettingRow>
      </Rows>
    </InstanceWindow>
  );
}
