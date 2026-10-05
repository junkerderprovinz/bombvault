// ReceiverServerCard sets up the append-only rest-server this box runs for its
// pairing group, and shows how it stands once it runs: where members copy to,
// whether it still refuses deletes, which members have a login of their own,
// and the Unraid template that keeps it editable in the Docker tab.

import { useEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import {
  ApiError,
  RECEIVER_TEMPLATE_URL,
  checkReceiverServer,
  forgetReceiverServer,
  getReceiverServer,
  revokeReceiverLogin,
  setUpReceiverServer,
  type ReceiverLogin,
  type ReceiverServer,
} from "../../lib/api";
import { hueVars } from "../../lib/appearance";
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

type T = ReturnType<typeof useT>["t"];

// The receiver's colour on the Modules card, so the card reads as the module's.
const HUE = 6;

const INPUT = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** What setting up answered, kept only until the page is left: the password
 *  is not shown again. */
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

export function ReceiverServerCard({ t }: { t: T }) {
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
      })
      .catch((e: unknown) => alive && setError(e instanceof Error ? e.message : t("receiver.server.loadError")))
      .finally(() => alive && setLoaded(true));
    return () => {
      alive = false;
    };
  }, [t]);

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

  if (!loaded) return null;

  return (
    <section
      className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-5 flex flex-col gap-3"
      style={hueVars(HUE) as CSSProperties}
      aria-label={t("receiver.server.title")}
    >
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={HUE}>
          {t("receiver.server.title")}
          <InfoBubble tip={t("receiver.server.tip")} onAccent />
        </Badge>
      </h2>
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {!server && !error && (
        <div className="flex flex-col gap-3">
          <p className="text-sm text-carbon-textSub">{t("receiver.server.intro")}</p>
          <Button
            label={t("receiver.server.setUp")}
            labelKey="receiver.server.setUp"
            tone="accent"
            onClick={() => setFormOpen(true)}
            className="self-start"
          />
        </div>
      )}

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
          </div>
          {server.check !== "protected" && server.checkDetail && (
            <p className="text-xs text-carbon-textSub wrap-break-word">{server.checkDetail}</p>
          )}
          <dl className="grid gap-x-4 gap-y-1 text-xs md:grid-cols-[auto_1fr]">
            <dt className="text-carbon-textMuted">{t("receiver.server.address")}</dt>
            <dd dir="ltr" className="font-mono text-carbon-text break-all text-start">
              {server.url || t("receiver.server.noAddress")}
            </dd>
            <dt className="text-carbon-textMuted">{t("receiver.server.outsideUser")}</dt>
            <dd dir="ltr" className="font-mono text-carbon-text text-start">{server.user}</dd>
            <dt className="text-carbon-textMuted">{t("receiver.server.folder")}</dt>
            <dd dir="ltr" className="font-mono text-carbon-text break-all text-start">{server.hostPath}</dd>
          </dl>
          <p className="text-xs text-carbon-textMuted">{t("receiver.server.offered")}</p>

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("receiver.server.logins")}
              <InfoBubble tip={t("receiver.server.loginsTip")} />
            </span>
            {server.logins.length === 0 && <p className="text-xs text-carbon-textMuted">{t("receiver.server.noLogins")}</p>}
            {server.logins.length > 0 && (
              <ul className="flex flex-col gap-1">
                {server.logins.map((l) => (
                  <li key={l.memberId} className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs">
                    <span className="font-semibold text-carbon-text">{l.name}</span>
                    <span dir="ltr" className="font-mono text-carbon-textSub">{l.user}</span>
                    <span className="text-carbon-textMuted">{new Date(l.createdAt * 1000).toLocaleDateString()}</span>
                    <Button
                      label={t("receiver.server.revoke")}
                      labelKey="receiver.server.revoke"
                      tone="neutral"
                      onClick={() => void revoke(l)}
                      className="ms-auto"
                    />
                  </li>
                ))}
              </ul>
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

          <div className="flex flex-wrap items-center gap-2">
            <Button
              label={t("receiver.server.checkAgain")}
              labelKey="receiver.server.checkAgain"
              tone="accent"
              onClick={() => void runCheck()}
              busy={checking}
              disabled={checking}
            />
            <a href={RECEIVER_TEMPLATE_URL} download className="text-xs text-carbon-textSub underline">
              {t("receiver.server.downloadTemplate")}
            </a>
            <Button
              label={t("receiver.server.forget")}
              labelKey="receiver.server.forget"
              tone="neutral"
              hint={t("receiver.server.forgetTip")}
              onClick={() => void forget()}
              className="ms-auto"
            />
          </div>
        </div>
      )}

      {confirmDialog}
      {formOpen && (
        <ReceiverServerDialog
          t={t}
          defaults={defaults}
          onClose={() => setFormOpen(false)}
          onDone={(s, f) => {
            setServer(s);
            setFresh(f);
            setFormOpen(false);
          }}
        />
      )}
    </section>
  );
}

function ReceiverServerDialog({
  t,
  defaults,
  onClose,
  onDone,
}: {
  t: T;
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
        push(r.error ?? t("receiver.server.setUpFailed"), "fail");
        return;
      }
      onDone(r.server, { password: r.password ?? "", template: r.template ?? "none" });
    } catch (e) {
      // The server refuses with 403 while no login password is set.
      if (e instanceof ApiError && e.status === 403) push(t("receiver.server.needsPassword"), "fail");
      else push(e instanceof Error ? e.message : t("receiver.server.setUpFailed"), "fail");
    } finally {
      setSaving(false);
    }
  }

  return createPortal(
    <div className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" onClick={onClose}>
      <div className="relative w-full max-w-lg">
        <h2 className="flex items-center px-5">
          <Badge tone="heading" size="heading" wrap>{t("receiver.server.setUp")}</Badge>
        </h2>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t("receiver.server.setUp")}
          onClick={(e) => e.stopPropagation()}
          className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
        >
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
          <label className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("receiver.server.port")}
              <InfoBubble tip={t("receiver.server.portTip")} />
            </span>
            <NumberField min={1} max={65535} value={port} onChange={(e) => setPort(parseInt(e.target.value, 10))} className={INPUT} />
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("receiver.server.host")}
              <InfoBubble tip={t("receiver.server.hostTip")} />
            </span>
            <input
              type="text"
              value={host}
              onChange={(e) => setHost(e.target.value)}
              placeholder={t("receiver.server.hostPlaceholder")}
              spellCheck={false}
              autoComplete="off"
              dir="ltr"
              className={`${INPUT} font-mono text-start`}
            />
          </label>
          <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={saving} />
            <Button
              label={t("receiver.server.start")}
              labelKey="receiver.server.start"
              tone="accent"
              onClick={() => void submit()}
              disabled={!canSave}
              busy={saving}
            />
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}
