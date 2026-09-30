import { useCallback, useEffect, useRef, useState } from "react";
import { Badge, type BadgeSize } from "../../components/Badge";
import { Button } from "../../components/Button";
import { ConfirmPrompt } from "../../components/ConfirmPrompt";
import { IconDisclosure } from "../../components/IconDisclosure";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { ReadmeButton } from "../../components/ReadmeButton";
import { Toggle } from "../../components/Toggle";
import {
  MCP_CERTIFICATE_URL,
  addMcpCertificateName,
  createMcpKey,
  listMcpKeys,
  purgeMcpKey,
  revokeMcpKey,
  rotateMcpKey,
  updateMcpKey,
  type McpKeySecretResponse,
  type McpKeyView,
  type McpKeysResponse,
  type McpOAuthView,
  type OkEnvelope,
} from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT, type TranslationKey } from "../../lib/i18n";
import { CLOUD_CLIENTS, KIND_LABEL, LOCAL_CLIENTS, OTHER_CLIENT, clientById, type McpClient } from "../../lib/mcpClients";
import { mcpUrl, type McpSnippetInput } from "../../lib/mcpSnippets";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { McpClientDialog, type FreshKey } from "./McpClientDialog";
import { ClientMark } from "./McpClientMark";
import { keyLogId, McpKeyLog } from "./McpKeyLog";
import { McpOAuthSettings } from "./McpOAuthSettings";
import { Card, LOGIN_PASSWORD_FIELD } from "./shared";

// McpServerCard is where an MCP key comes from, and the only place it is ever
// visible: the server keeps a hash, so a key that is not copied out of the
// panel below is lost and has to be replaced.
//
// Without a key the endpoint answers 404, which makes this card the switch for
// the whole feature. It therefore also carries the two conditions under which
// a key would be handed to the wrong party: a web interface without a login
// password, and a page opened under a public-looking host name, where the
// server refuses to mint one at all.

/** The refusals of mcp_keys.go that the card answers with a sentence of its
 *  own rather than with the server's English one. */
const CODE_MESSAGE: Record<string, TranslationKey> = {
  "mcp-key-not-found": "mcp.notFound",
  "mcp-key-limit": "mcp.limitReached",
  "mcp-key-label-invalid": "mcp.labelInvalid",
  "mcp-key-label-taken": "mcp.labelTaken",
  "mcp-key-needs-password": "mcp.needsPasswordForHost",
  "cert-not-own": "mcp.certNotOwn",
  "cert-name-invalid": "mcp.certNameInvalid",
  "cert-name-limit": "mcp.certNameLimit",
  "cert-write-failed": "mcp.certWriteFailed",
};

/** Why a revoked key or grant is in the list, where it is not the operator's
 *  own Revoke. */
const REVOKED_REASON: Partial<Record<McpKeyView["revokedReason"], TranslationKey>> = {
  "config-restore": "mcp.revokedByRestore",
  replaced: "mcp.revokedReplaced",
  expired: "mcp.revokedExpired",
  "refresh-reuse": "mcp.revokedReuse",
  "code-replay": "mcp.revokedReuse",
  client: "mcp.revokedByClient",
  "oauth-off": "mcp.revokedOAuthOff",
  "oauth-moved": "mcp.revokedOAuthMoved",
};

type Pending =
  | { kind: "rotate" | "revoke" | "purge"; item: McpKeyView }
  | { kind: "certificate" };

// Each key is a tile laid out like an off-site target in OffsiteTargetsSection,
// with its chips at the size those use. The log opens inside the tile the way a
// fleet peer opens its details.
const TILE_BADGE_SIZE: BadgeSize = "medium";

function LogButton({
  keyId,
  open,
  onClick,
  t,
}: {
  keyId: string;
  open: boolean;
  onClick: () => void;
  t: (key: TranslationKey) => string;
}) {
  return (
    <Button
      label={t("mcp.log")}
      labelKey="mcp.log"
      tone="neutral"
      onClick={onClick}
      glyph={<IconDisclosure open={open} />}
      ariaExpanded={open}
      ariaControls={open ? keyLogId(keyId) : undefined}
    />
  );
}

/** How often the card asks for the key list while a setup dialog waits for
 *  the client's first call. */
const FIRST_CALL_POLL_MS = 3000;

export function McpServerCard({ hueIndex, passwordSet }: { hueIndex?: number; passwordSet?: boolean }) {
  const { t } = useT();
  const { push } = useToast();
  const [data, setData] = useState<McpKeysResponse | null>(null);
  const [lists, setLists] = useState(0);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  // The phone's confirm sheet stays tappable while a call runs, and busy
  // reaches the card's buttons only with the next render, so a second tap
  // would run the call twice.
  const inFlight = useRef(false);
  const [fresh, setFresh] = useState<FreshKey | null>(null);
  const [keyVisible, setKeyVisible] = useState(true);
  const [dialog, setDialog] = useState<McpClient | null>(null);
  const [renaming, setRenaming] = useState("");
  const [draft, setDraft] = useState("");
  const [revokedOpen, setRevokedOpen] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});
  const [logOpen, setLogOpen] = useState<ReadonlySet<string>>(new Set());

  const bumpShake = (id: string) => setShake((s) => ({ ...s, [id]: (s[id] ?? 0) + 1 }));
  const toggleLog = (id: string) =>
    setLogOpen((open) => {
      const next = new Set(open);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const reload = useCallback(async () => {
    try {
      const res = await listMcpKeys();
      if (!res.ok) {
        setFailed(true);
        return;
      }
      setData(res);
      setLists((n) => n + 1);
      setFailed(false);
      // A handed-out key stays until it is dismissed, unless the list shows
      // that it stopped working: revoked, or replaced from another tab.
      setFresh((f) => (f && res.keys.some((k) => k.id === f.id && k.hint === f.hint) ? f : null));
    } catch {
      setFailed(true);
    }
  }, []);

  // Switching sign-in through OAuth off, or moving its address, revokes every
  // grant, so their tiles are read again.
  const oauthChanged = (oauth: McpOAuthView) => {
    const ended = !oauth.enabled || oauth.issuer !== data?.oauth.issuer;
    setData((prev) => (prev ? { ...prev, oauth } : prev));
    if (ended && data?.keys.some((k) => k.viaOAuth)) void reload();
  };

  // Setting or clearing the login password further down the tab changes what
  // this card may offer, so it asks the server again.
  useEffect(() => {
    void reload();
  }, [reload, passwordSet]);

  // An open setup dialog waits for the client's first call, which shows as a
  // change in the key's last use. It asks at once too, so the dialog learns
  // each key's last use as of its opening.
  useEffect(() => {
    if (dialog === null) return;
    void reload();
    const timer = window.setInterval(() => void reload(), FIRST_CALL_POLL_MS);
    return () => window.clearInterval(timer);
  }, [dialog, reload]);

  const origin = window.location.origin;
  // An IPv6 hostname keeps its brackets, the certificate names do not.
  const host = window.location.hostname.replace(/^\[(.*)\]$/, "$1");
  const secure = origin.toLowerCase().startsWith("https:");

  const keys = data?.keys ?? [];
  // A key APP_KEY no longer matches is listed so it can be replaced, but it
  // lets no assistant in.
  const working = keys.filter((k) => !k.unusable).length;
  // Grants have a limit of their own, so only the keys count against this one.
  const staticKeys = keys.filter((k) => !k.viaOAuth).length;
  const oauthActive = data?.oauth.active === true;
  const revoked = data?.revoked ?? [];
  const limit = data?.limit ?? 0;
  const certificate = data?.certificate ?? null;
  const certCovers =
    certificate !== null && certificate.names.some((n) => n.toLowerCase() === host.toLowerCase());
  // The client trusts BombVault's own certificate only through the file it is
  // pointed at, so the snippets take their mcp-remote form as soon as this
  // address is served with it.
  const ownCertificate = secure && certificate !== null && certificate.selfIssued && certCovers;

  /** The sentence for a refused call: the card's own for a known code, the
   *  server's otherwise. */
  function refusal(res: OkEnvelope): string {
    const key = res.code ? CODE_MESSAGE[res.code] : undefined;
    if (key === "mcp.limitReached") return t(key, limit);
    if (key === "mcp.needsPasswordForHost") return t(key).replace("{host}", host);
    if (key) return t(key);
    return res.error ?? t("common.actionFailed");
  }

  /** Runs one key or certificate call, reports its outcome and refreshes the
   *  list. It answers with the payload so a caller can take the fresh key out
   *  of it. */
  async function act<T extends OkEnvelope>(
    id: string,
    call: () => Promise<T>,
    okMessage: string
  ): Promise<T | null> {
    if (inFlight.current) return null;
    inFlight.current = true;
    setBusy(true);
    try {
      const res = await call();
      if (!res.ok) {
        push(refusal(res), "fail");
        bumpShake(id);
        if (res.code === "mcp-key-not-found" || res.code === "mcp-key-needs-password") await reload();
        return null;
      }
      if (okMessage) push(okMessage, "success");
      await reload();
      return res;
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      bumpShake(id);
      return null;
    } finally {
      inFlight.current = false;
      setBusy(false);
      setPending(null);
    }
  }

  function showFresh(res: McpKeySecretResponse) {
    if (res.key && res.item) setFresh({ key: res.key, id: res.item.id, hint: res.item.hint });
  }

  // The card takes the key the moment the server answers, so it survives a
  // dialog closed while the call ran.
  async function create(client: McpClient, label: string, canStart: boolean): Promise<FreshKey | null> {
    const id = client.id === OTHER_CLIENT.id ? "" : client.id;
    const res = await act("create", () => createMcpKey(label, canStart, id), t("mcp.created"));
    if (!res?.key || !res.item) return null;
    const made = { key: res.key, id: res.item.id, hint: res.item.hint };
    setKeyVisible(true);
    setFresh(made);
    return made;
  }

  async function rotate(item: McpKeyView) {
    const res = await act(`rotate:${item.id}`, () => rotateMcpKey(item.id), t("mcp.rotated"));
    if (res) showFresh(res);
  }

  async function rename(item: McpKeyView) {
    const next = draft.trim();
    if (next === "" || next === item.label) {
      setRenaming("");
      return;
    }
    // A refused name stays in the field, so the operator can correct it.
    const res = await act(`rename:${item.id}`, () => updateMcpKey(item.id, { label: next }), "");
    if (res) setRenaming("");
  }

  async function setCanStart(item: McpKeyView, on: boolean) {
    // The row follows the switch at once and the answer decides whether it
    // stays there, so what the operator sees is the state the server holds.
    const show = (v: boolean) =>
      setData((prev) =>
        prev
          ? { ...prev, keys: prev.keys.map((k) => (k.id === item.id ? { ...k, canStartBackups: v } : k)) }
          : prev
      );
    setBusy(true);
    show(on);
    try {
      const res = await updateMcpKey(item.id, { canStartBackups: on });
      if (res.ok) {
        push(t("mcp.permissionSaved"), "success");
        return;
      }
    } catch {
      // A refused save and an unreachable server leave the same wrong switch
      // on screen, so both take the way back below.
    } finally {
      setBusy(false);
    }
    show(!on);
    bumpShake(`start:${item.id}`);
    push(t("common.saveFailed"), "fail");
  }

  function downloadCertificate() {
    const a = document.createElement("a");
    a.href = MCP_CERTIFICATE_URL;
    a.download = "";
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  async function copy(text: string) {
    const ok = await copyText(text);
    push(ok ? t("common.copied") : t("vm.ssh.copyFailed"), ok ? "success" : "fail");
  }

  const snippetBase: Omit<McpSnippetInput, "key"> = {
    origin,
    endpointPath: data?.endpointPath ?? "/mcp",
    selfSigned: ownCertificate,
  };
  const endpoint = mcpUrl(snippetBase);

  function clientButton(c: McpClient) {
    const name = c === OTHER_CLIENT ? t("mcp.otherClient") : c.name;
    return (
      <ReadmeButton
        key={c.id}
        tile={c.tile}
        className={c.lift ? "glim-mark-lift" : undefined}
        parts={[{ name, sub: c.kind ? t(KIND_LABEL[c.kind]) : undefined, onClick: () => setDialog(c) }]}
        mark={<ClientMark client={c} />}
      />
    );
  }

  // The newest restore-revoked key still matters as long as no key was created
  // after it: once one was, the operator has clearly seen the notice.
  const restoreRevoked = revoked
    .filter((k) => k.revokedReason === "config-restore")
    .reduce((newest, k) => Math.max(newest, k.revokedAt), 0);
  const showRestoreNotice =
    restoreRevoked > 0 && !keys.some((k) => Math.max(k.createdAt, k.rotatedAt) > restoreRevoked);

  const canMint = data?.hostAllowsKeys !== false;

  return (
    <Card title={t("mcp.title")} hint={t("mcp.hint")} hueIndex={hueIndex}>
      {failed && <p className="text-sm text-statusWarn">{t("mcp.loadFailed")}</p>}

      {!failed && (
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${working > 0 ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {data === null
              ? t("folder.loading")
              : working > 0
                ? t("mcp.statusOn", working)
                : oauthActive
                  ? t("mcp.statusOAuthOnly")
                  : keys.length > 0
                    ? t("mcp.statusNoneWorks")
                    : t("mcp.statusOff")}
          </span>
        </div>
      )}

      {data !== null && !failed && (
        <>
          {data.authEnabled === false && (
            <div className="flex flex-col items-start gap-2 rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              <p>{t("mcp.noPasswordWarning")}</p>
              <Button
                label={t("mcp.setPassword")}
                labelKey="mcp.setPassword"
                tone="subtle"
                onClick={() => {
                  const field = document.getElementById(LOGIN_PASSWORD_FIELD);
                  field?.scrollIntoView?.({ behavior: "smooth", block: "center" });
                  field?.focus();
                }}
                hueIndex={hueIndex}
              />
            </div>
          )}

          {!canMint && (
            <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              {t("mcp.needsPasswordForHost").replace("{host}", host)}
            </p>
          )}

          {secure && certificate !== null && !certCovers && (
            <div className="flex flex-col items-start gap-2 rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              <p>
                {(certificate.selfIssued
                  ? t("mcp.certNotForThisAddress")
                  : t("mcp.certOwnNotForThisAddress")
                ).replace("{host}", host)}
              </p>
              {certificate.selfIssued && canMint && (
                <Button
                  label={t("mcp.certAddAddress")}
                  labelKey="mcp.certAddAddress"
                  tone="subtle"
                  onClick={() => setPending({ kind: "certificate" })}
                  disabled={busy}
                  className={`glim-btn-wrap${shake.certificate ? " glim-shake" : ""}`}
                  hueIndex={hueIndex}
                />
              )}
            </div>
          )}

          {showRestoreNotice && (
            <p className="text-sm text-statusWarn">{t("mcp.restoreRevokedNotice")}</p>
          )}

          {keys.some((k) => k.unusable === "app-key-changed") && (
            <p className="text-sm text-statusWarn">{t("mcp.appKeyChanged")}</p>
          )}

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("mcp.endpointLabel")}
              <InfoBubble tip={t("mcp.endpointHint")} />
            </span>
            <div className="flex flex-wrap items-center gap-2">
              <input
                readOnly
                value={endpoint}
                aria-label={t("mcp.endpointLabel")}
                dir="ltr"
                className="min-w-0 max-w-96 flex-[1_1_18rem] rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-sm text-carbon-text glim-field-focus"
              />
              <Button
                label={t("common.copy")}
                labelKey="common.copy"
                tone="subtle"
                onClick={() => void copy(endpoint)}
                hueIndex={hueIndex}
              />
              {ownCertificate && (
                <Button
                  label={t("mcp.certDownload")}
                  labelKey="mcp.certDownload"
                  tone="subtle"
                  onClick={downloadCertificate}
                  hueIndex={hueIndex}
                />
              )}
            </div>
          </div>

          <McpOAuthSettings
            oauth={data.oauth}
            authEnabled={data.authEnabled}
            onChange={oauthChanged}
            idPrefix="bv-mcp-card"
            t={t}
          />
        </>
      )}

      {/* Outside the load gate: the server keeps no copy of this key, so a
          failed reload must not hide it. The open setup dialog shows it itself. */}
      {fresh !== null && dialog === null && (
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2.5">
          <p className="flex items-center gap-1.5 text-sm font-medium text-carbon-text">
            {t("mcp.newKeyTitle")}
            <InfoBubble tip={t("mcp.showOnce")} />
          </p>
          <RevealInput
            visible={keyVisible}
            onToggleVisible={() => setKeyVisible((v) => !v)}
            showLabel={t("common.showValue")}
            hideLabel={t("common.hideValue")}
            value={fresh.key}
            readOnly
            aria-label={t("mcp.newKeyTitle")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface px-3 py-1.5 font-mono text-sm text-carbon-text glim-field-focus"
          />
          <div className="flex items-center gap-3">
            <Button
              label={t("mcp.copyKey")}
              labelKey="common.copy"
              tone="subtle"
              onClick={() => void copy(fresh.key)}
              hueIndex={hueIndex}
            />
            <Button
              label={t("mcp.dismissKey")}
              labelKey="mcp.dismissKey"
              tone="accent"
              onClick={() => setFresh(null)}
              hueIndex={hueIndex}
            />
          </div>
        </div>
      )}

      {data !== null && !failed && (
        <>
          <div className="flex flex-col gap-2.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("mcp.snippetsLabel")}
              <InfoBubble tip={t("mcp.connectHint")} />
            </span>
            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <span className="text-[length:var(--text-caption)] font-medium uppercase tracking-[0.06em] text-carbon-textMuted">
                  {t("mcp.groupLocal")}
                </span>
                <div className="glim-readme-btn-rows">{LOCAL_CLIENTS.map(clientButton)}</div>
              </div>
              <div className="flex flex-col gap-2">
                <span className="flex items-center gap-1.5 text-[length:var(--text-caption)] font-medium uppercase tracking-[0.06em] text-carbon-textMuted">
                  {t("mcp.groupCloud")}
                  <InfoBubble tip={t("mcp.cloudWarning")} />
                </span>
                <div className="glim-readme-btn-rows">{CLOUD_CLIENTS.map(clientButton)}</div>
              </div>
              <div className="glim-readme-btn-rows">{clientButton(OTHER_CLIENT)}</div>
            </div>
          </div>

          {keys.length > 0 && (
            <ul className="flex flex-col gap-3">
              {keys.map((k) => (
                <li key={k.id} className="glim-tile flex flex-col gap-3 rounded-card p-3">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="flex min-w-0 flex-col gap-1">
                      <div className="flex min-w-0 items-center gap-2 max-md:flex-wrap">
                        {renaming === k.id ? (
                          <input
                            autoFocus
                            value={draft}
                            maxLength={64}
                            aria-label={t("mcp.labelLabel")}
                            onChange={(e) => setDraft(e.target.value)}
                            onBlur={() => {
                              if (!busy) void rename(k);
                            }}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") void rename(k);
                              if (e.key === "Escape") setRenaming("");
                            }}
                            className={`w-64 rounded-control bg-carbon-surface3 px-3 py-1 text-sm text-carbon-text glim-field-focus-well${
                              shake[`rename:${k.id}`] ? " glim-shake" : ""
                            }`}
                          />
                        ) : (
                          <>
                            <span className="glim-client-mark" aria-hidden="true">
                              <ClientMark client={clientById(k.client)} forKey />
                            </span>
                            <span className="truncate text-sm text-carbon-text max-md:min-w-24 max-md:flex-1 max-md:whitespace-normal max-md:wrap-anywhere">
                              {k.label}
                            </span>
                            <Button
                              label={t("common.edit")}
                              labelKey="common.edit"
                              variant="icon"
                              tone="neutral"
                              onClick={() => {
                                setDraft(k.label);
                                setRenaming(k.id);
                              }}
                              hueIndex={hueIndex}
                            />
                          </>
                        )}
                      </div>
                      {k.unusable === "app-key-changed" ? (
                        <span className="text-xs text-statusWarn">{t("mcp.keyUnusable")}</span>
                      ) : (
                        <>
                          <span className="text-xs text-carbon-textMuted">
                            {k.viaOAuth ? (
                              t("mcp.grantCreated").replace("{date}", formatTs(k.createdAt))
                            ) : (
                              <>
                                {t("mcp.keyHint").replace("{hint}", k.hint)}
                                {" · "}
                                {k.rotatedAt > 0
                                  ? t("mcp.keyRotated").replace("{date}", formatTs(k.rotatedAt))
                                  : t("mcp.keyCreated").replace("{date}", formatTs(k.createdAt))}
                              </>
                            )}
                          </span>
                          {/* An IPv6 address has no break opportunity and runs
                              past a phone's tile. */}
                          <span className="text-xs text-carbon-textMuted max-md:wrap-anywhere">
                            {k.lastUsedAt === 0
                              ? t("mcp.keyNeverUsed")
                              : k.lastUsedFrom === ""
                                ? t("mcp.keyLastUsedNoAddr").replace("{when}", relativeTime(t, k.lastUsedAt))
                                : t("mcp.keyLastUsed")
                                    .replace("{when}", relativeTime(t, k.lastUsedAt))
                                    .replace("{addr}", k.lastUsedFrom)}
                          </span>
                        </>
                      )}
                      <span className="flex flex-wrap gap-2">
                        <Badge tone="neutral" size={TILE_BADGE_SIZE} wrap className="glim-tile-raise">
                          {k.canStartBackups ? t("mcp.canStart") : t("mcp.readOnly")}
                        </Badge>
                        <Badge tone="neutral" size={TILE_BADGE_SIZE} wrap className="glim-tile-raise">
                          <span className="glim-num">{t("mcp.callsToday", k.callsToday)}</span>
                        </Badge>
                      </span>
                    </div>
                    <div className="flex min-w-0 flex-wrap items-start gap-2">
                      <LogButton keyId={k.id} open={logOpen.has(k.id)} onClick={() => toggleLog(k.id)} t={t} />
                      <Button
                        label={t("mcp.revoke")}
                        labelKey="mcp.revoke"
                        tone="neutral"
                        onClick={() => setPending({ kind: "revoke", item: k })}
                        disabled={busy}
                        className={shake[`revoke:${k.id}`] ? "glim-shake" : ""}
                        hueIndex={hueIndex}
                      />
                      {canMint && !k.viaOAuth && (
                        <Button
                          label={t("mcp.rotate")}
                          labelKey="mcp.rotate"
                          tone="neutral"
                          onClick={() => setPending({ kind: "rotate", item: k })}
                          disabled={busy}
                          className={`glim-btn-wrap${shake[`rotate:${k.id}`] ? " glim-shake" : ""}`}
                          hueIndex={hueIndex}
                        />
                      )}
                    </div>
                  </div>

                  <Toggle
                    key={shake[`start:${k.id}`] ?? 0}
                    label={t("mcp.allowStart")}
                    checked={k.canStartBackups}
                    onChange={(v) => void setCanStart(k, v)}
                    disabled={busy}
                    className={shake[`start:${k.id}`] ? "glim-shake" : ""}
                  />

                  {logOpen.has(k.id) && <McpKeyLog keyId={k.id} used={k.lastUsedAt > 0} t={t} />}
                </li>
              ))}
            </ul>
          )}

          {revoked.length > 0 && (
            <div className="flex flex-col gap-2">
              <button
                type="button"
                onClick={() => setRevokedOpen((v) => !v)}
                aria-expanded={revokedOpen}
                className="flex items-center gap-1.5 self-start text-xs text-carbon-textSub hover:text-carbon-text pointer-coarse:min-h-(--btn-h)"
              >
                <IconDisclosure open={revokedOpen} />
                {t("mcp.revokedList", revoked.length)}
              </button>
              {revokedOpen && (
                <ul className="flex flex-col gap-3">
                  {revoked.map((k) => (
                    <li key={k.id} className="glim-tile flex flex-col gap-3 rounded-card p-3">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div className="flex min-w-0 flex-col gap-1">
                          <span className="truncate text-sm text-carbon-text">{k.label}</span>
                          <span className="text-xs text-carbon-textMuted">
                            {t(REVOKED_REASON[k.revokedReason] ?? "mcp.revokedAt").replace(
                              "{date}",
                              formatTs(k.revokedAt)
                            )}
                          </span>
                          <span className="text-xs text-carbon-textMuted">
                            {t("mcp.keyCreated").replace("{date}", formatTs(k.createdAt))}
                            {" · "}
                            {k.lastUsedAt === 0
                              ? t("mcp.keyNeverUsed")
                              : t("mcp.keyLastUsedNoAddr").replace("{when}", relativeTime(t, k.lastUsedAt))}
                          </span>
                        </div>
                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                          <LogButton keyId={k.id} open={logOpen.has(k.id)} onClick={() => toggleLog(k.id)} t={t} />
                          <Button
                            label={t("common.delete")}
                            labelKey="common.delete"
                            tone="neutral"
                            onClick={() => setPending({ kind: "purge", item: k })}
                            disabled={busy || k.inUse}
                            hint={k.inUse ? t("mcp.inUseTip") : undefined}
                            className={shake[`purge:${k.id}`] ? "glim-shake" : ""}
                            hueIndex={hueIndex}
                          />
                        </div>
                      </div>
                      {logOpen.has(k.id) && <McpKeyLog keyId={k.id} used={k.lastUsedAt > 0} t={t} />}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </>
      )}

      {dialog !== null && data !== null && (
        <McpClientDialog
          key={dialog.id}
          client={dialog}
          keys={keys}
          lists={lists}
          snippetBase={snippetBase}
          mintRefused={canMint ? undefined : t("mcp.needsPasswordForHost").replace("{host}", host)}
          limitNote={staticKeys >= limit ? t("mcp.limitReached", limit) : undefined}
          oauth={data.oauth}
          authEnabled={data.authEnabled}
          onOAuthChange={oauthChanged}
          allowStartHint={t("mcp.allowStartHint")
            .replace("{n}", String(data.startsPerHour))
            .replace("{minutes}", String(data.cooldownMinutes))
            .replace("{perDay}", String(data.itemStartsPerDay))}
          onCreate={(label, canStart) => create(dialog, label, canStart)}
          onCopy={(text) => void copy(text)}
          onDownloadCertificate={downloadCertificate}
          onClose={(used) => {
            setDialog(null);
            if (used) setFresh((f) => (f?.id === used ? null : f));
          }}
          t={t}
        />
      )}

      {pending?.kind === "certificate" && (
        <ConfirmPrompt
          title={t("mcp.certAddTitle").replace("{host}", host)}
          message={t("mcp.certAddConfirm").replace("{host}", host)}
          confirmLabel={t("mcp.certAddAddress")}
          confirmLabelKey="mcp.certAddAddress"
          cancelLabel={t("common.cancel")}
          onConfirm={() =>
            void act("certificate", () => addMcpCertificateName(host), t("mcp.certAdded"))
          }
          onCancel={() => setPending(null)}
        />
      )}

      {pending?.kind === "rotate" && (
        <ConfirmPrompt
          title={t("mcp.rotateTitle")}
          message={t("mcp.rotateConfirm").replace("{name}", pending.item.label)}
          confirmLabel={t("mcp.rotate")}
          confirmLabelKey="mcp.rotate"
          cancelLabel={t("common.cancel")}
          onConfirm={() => void rotate(pending.item)}
          onCancel={() => setPending(null)}
        />
      )}

      {pending?.kind === "revoke" && (
        <ConfirmPrompt
          title={t("mcp.revokeTitle")}
          message={t("mcp.revokeConfirm").replace("{name}", pending.item.label)}
          confirmLabel={t("mcp.revoke")}
          confirmLabelKey="mcp.revoke"
          cancelLabel={t("common.cancel")}
          onConfirm={() =>
            void act(`revoke:${pending.item.id}`, () => revokeMcpKey(pending.item.id), t("mcp.revoked"))
          }
          onCancel={() => setPending(null)}
        />
      )}

      {pending?.kind === "purge" && (
        <ConfirmPrompt
          title={t("mcp.purgeTitle")}
          message={t("mcp.purgeConfirm").replace("{name}", pending.item.label)}
          confirmLabel={t("common.delete")}
          confirmLabelKey="common.delete"
          cancelLabel={t("common.cancel")}
          onConfirm={() =>
            void act(`purge:${pending.item.id}`, () => purgeMcpKey(pending.item.id), t("mcp.purged"))
          }
          onCancel={() => setPending(null)}
        />
      )}
    </Card>
  );
}
