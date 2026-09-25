import { useCallback, useEffect, useState } from "react";
import { Button } from "../../components/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import { IconDisclosure } from "../../components/IconDisclosure";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { HUE_OFFSET, Selector } from "../../components/Selector";
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
  type OkEnvelope,
} from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT, type TranslationKey } from "../../lib/i18n";
import {
  KEY_PLACEHOLDER,
  claudeCodeSnippet,
  claudeDesktopSnippet,
  genericSnippet,
  mcpUrl,
  type McpSnippetInput,
} from "../../lib/mcpSnippets";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { Card, LOGIN_PASSWORD_FIELD, ToggleRow } from "./shared";

// McpServerCard is where an MCP key comes from, and the only place it is ever
// visible: the server keeps a hash, so a key that is not copied out of the
// panel below is lost and has to be replaced.
//
// Without a key the endpoint answers 404, which makes this card the switch for
// the whole feature. It therefore also carries the two conditions under which
// a key would be handed to the wrong party: a web interface without a login
// password, and a page opened under a public-looking host name, where the
// server refuses to mint one at all.

/** The clients the snippets are written for. "other" is any client that speaks
 *  Streamable HTTP and sends a header. */
type ClientId = "code" | "desktop" | "other";

const SNIPPET_HINT: Record<ClientId, TranslationKey> = {
  code: "mcp.snippetClaudeCodeHint",
  desktop: "mcp.snippetDesktopHint",
  other: "mcp.snippetOtherHint",
};

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

type Pending =
  | { kind: "rotate" | "revoke" | "purge"; item: McpKeyView }
  | { kind: "certificate" };

/** A key handed out by create or rotate, with the row it belongs to. */
interface FreshKey {
  key: string;
  id: string;
  hint: string;
}

export function McpServerCard({ hueIndex, passwordSet }: { hueIndex?: number; passwordSet?: boolean }) {
  const { t } = useT();
  const { push } = useToast();
  const [data, setData] = useState<McpKeysResponse | null>(null);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [fresh, setFresh] = useState<FreshKey | null>(null);
  const [keyVisible, setKeyVisible] = useState(true);
  const [adding, setAdding] = useState(false);
  const [label, setLabel] = useState("");
  const [allowStart, setAllowStart] = useState(true);
  const [client, setClient] = useState<ClientId>("code");
  const [renaming, setRenaming] = useState("");
  const [draft, setDraft] = useState("");
  const [revokedOpen, setRevokedOpen] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});

  const bumpShake = (id: string) => setShake((s) => ({ ...s, [id]: (s[id] ?? 0) + 1 }));

  const reload = useCallback(async () => {
    try {
      const res = await listMcpKeys();
      if (!res.ok) {
        setFailed(true);
        return;
      }
      setData(res);
      setFailed(false);
      // A handed-out key stays until it is dismissed, unless the list shows
      // that it stopped working: revoked, or replaced from another tab.
      setFresh((f) => (f && res.keys.some((k) => k.id === f.id && k.hint === f.hint) ? f : null));
    } catch {
      setFailed(true);
    }
  }, []);

  // Setting or clearing the login password further down the tab changes what
  // this card may offer, so it asks the server again.
  useEffect(() => {
    void reload();
  }, [reload, passwordSet]);

  const origin = window.location.origin;
  // An IPv6 hostname keeps its brackets, the certificate names do not.
  const host = window.location.hostname.replace(/^\[(.*)\]$/, "$1");
  const secure = origin.toLowerCase().startsWith("https:");

  const keys = data?.keys ?? [];
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
      setBusy(false);
      setPending(null);
    }
  }

  function showFresh(res: McpKeySecretResponse) {
    if (res.key && res.item) setFresh({ key: res.key, id: res.item.id, hint: res.item.hint });
  }

  async function create() {
    const res = await act("create", () => createMcpKey(label.trim(), allowStart), t("mcp.created"));
    if (!res) return;
    showFresh(res);
    setAdding(false);
    setLabel("");
    setAllowStart(true);
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

  async function copy(text: string) {
    const ok = await copyText(text);
    push(ok ? t("common.copied") : t("vm.ssh.copyFailed"), ok ? "success" : "fail");
  }

  const snippetInput: McpSnippetInput = {
    origin,
    endpointPath: data?.endpointPath ?? "/mcp",
    key: fresh?.key ?? KEY_PLACEHOLDER,
    selfSigned: ownCertificate,
  };
  const snippet =
    client === "code"
      ? claudeCodeSnippet(snippetInput)
      : client === "desktop"
        ? claudeDesktopSnippet(snippetInput)
        : genericSnippet(snippetInput);

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
            className={`inline-block h-2 w-2 rounded-full ${keys.length > 0 ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {data === null
              ? t("folder.loading")
              : keys.length > 0
                ? t("mcp.statusOn", keys.length)
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
                  className={shake.certificate ? "glim-shake" : ""}
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

          {keys.length > 0 && (
            <div className="flex flex-col gap-1.5">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("mcp.endpointLabel")}
                <InfoBubble tip={t("mcp.endpointHint")} />
              </span>
              <div className="flex flex-wrap items-center gap-2">
                <code className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs break-all text-carbon-text">
                  {mcpUrl(snippetInput)}
                </code>
                <Button
                  label={t("common.copy")}
                  labelKey="common.copy"
                  variant="icon"
                  tone="subtle"
                  onClick={() => void copy(mcpUrl(snippetInput))}
                  hueIndex={hueIndex}
                />
                {ownCertificate && (
                  <a
                    href={MCP_CERTIFICATE_URL}
                    download
                    className="rounded-control bg-carbon-surface3 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hoverRaised glim-field-focus"
                  >
                    {t("mcp.certDownload")}
                  </a>
                )}
              </div>
            </div>
          )}
        </>
      )}

      {/* Outside the load gate: the server keeps no copy of this key, so a
          failed reload must not hide it. */}
      {fresh !== null && (
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2.5">
          <p className="text-sm font-medium text-carbon-text">{t("mcp.newKeyTitle")}</p>
          <p className="text-sm text-carbon-textSub">{t("mcp.showOnce")}</p>
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
          {keys.length > 0 && (
            <div className="flex flex-col gap-2">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("mcp.snippetsLabel")}
                <InfoBubble tip={t("mcp.tlsHint")} />
                <InfoBubble tip={t("mcp.privacyHint")} />
              </span>
              <div className="flex flex-wrap items-center gap-2">
                <Selector
                  items={[
                    { id: "code", label: "Claude Code" },
                    { id: "desktop", label: "Claude Desktop" },
                    { id: "other", label: t("mcp.snippetOther") },
                  ]}
                  label={t("mcp.snippetsLabel")}
                  active={client}
                  onChange={(id) => setClient(id as ClientId)}
                  hueOffset={HUE_OFFSET.mcpClient}
                />
                <InfoBubble tip={t(SNIPPET_HINT[client])} />
              </div>
              <pre className="overflow-x-auto rounded-control bg-carbon-surface2 px-3 py-2 text-xs text-carbon-text">
                <code>{snippet}</code>
              </pre>
              {client === "code" && (
                <p className="text-xs text-carbon-textSub">{t("mcp.snippetShellHistory")}</p>
              )}
              {client === "other" && ownCertificate && (
                <p className="text-xs text-carbon-textSub">{t("mcp.snippetOtherCert")}</p>
              )}
              <Button
                label={t("mcp.copySnippet")}
                labelKey="common.copy"
                tone="subtle"
                onClick={() => void copy(snippet)}
                className="self-start"
                hueIndex={hueIndex}
              />
            </div>
          )}

          {keys.length > 0 && (
            <ul className="flex flex-col gap-2">
              {keys.map((k) => (
                <li
                  key={k.id}
                  className="flex flex-col gap-2 rounded-control bg-carbon-surface2 px-3 py-2"
                >
                  <div className="flex items-center gap-2">
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
                        className={`w-64 rounded-control bg-carbon-surface px-3 py-1 text-sm text-carbon-text glim-field-focus${
                          shake[`rename:${k.id}`] ? " glim-shake" : ""
                        }`}
                      />
                    ) : (
                      <>
                        <span className="truncate text-sm text-carbon-text">{k.label}</span>
                        <Button
                          label={t("common.edit")}
                          labelKey="common.edit"
                          variant="icon"
                          tone="subtle"
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
                    <span className="text-xs text-carbon-textSub">
                      {t("mcp.keyHint").replace("{hint}", k.hint)}
                      {" · "}
                      {k.rotatedAt > 0
                        ? t("mcp.keyRotated").replace("{date}", formatTs(k.rotatedAt))
                        : t("mcp.keyCreated").replace("{date}", formatTs(k.createdAt))}
                      {" · "}
                      {k.lastUsedAt === 0
                        ? t("mcp.keyNeverUsed")
                        : k.lastUsedFrom === ""
                          ? t("mcp.keyLastUsedNoAddr").replace("{when}", relativeTime(t, k.lastUsedAt))
                          : t("mcp.keyLastUsed")
                              .replace("{when}", relativeTime(t, k.lastUsedAt))
                              .replace("{addr}", k.lastUsedFrom)}
                    </span>
                  )}

                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <Toggle
                      key={shake[`start:${k.id}`] ?? 0}
                      label={t("mcp.allowStart")}
                      checked={k.canStartBackups}
                      onChange={(v) => void setCanStart(k, v)}
                      disabled={busy}
                      className={shake[`start:${k.id}`] ? "glim-shake" : ""}
                    />
                    <div className="flex items-center gap-2">
                      <Button
                        label={t("mcp.revoke")}
                        labelKey="mcp.revoke"
                        tone="neutral"
                        onClick={() => setPending({ kind: "revoke", item: k })}
                        disabled={busy}
                        className={shake[`revoke:${k.id}`] ? "glim-shake" : ""}
                        hueIndex={hueIndex}
                      />
                      {canMint && (
                        <Button
                          label={t("mcp.rotate")}
                          labelKey="mcp.rotate"
                          tone="neutral"
                          onClick={() => setPending({ kind: "rotate", item: k })}
                          disabled={busy}
                          className={shake[`rotate:${k.id}`] ? "glim-shake" : ""}
                          hueIndex={hueIndex}
                        />
                      )}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}

          {canMint && keys.length >= limit && (
            <p className="text-sm text-carbon-textSub">{t("mcp.limitReached", limit)}</p>
          )}

          {canMint && keys.length < limit && !adding && (
            <Button
              label={t("mcp.newKey")}
              labelKey="mcp.newKey"
              tone="accent"
              onClick={() => setAdding(true)}
              disabled={busy}
              className="self-start"
              hueIndex={hueIndex}
            />
          )}

          {adding && (
            <div className="flex flex-col gap-3">
              <div className="flex flex-col gap-1.5">
                <label htmlFor="bv-mcp-label" className="text-xs text-carbon-textSub">
                  {t("mcp.labelLabel")}
                </label>
                <input
                  id="bv-mcp-label"
                  value={label}
                  maxLength={64}
                  onChange={(e) => setLabel(e.target.value)}
                  placeholder={t("mcp.labelPlaceholder")}
                  className={`w-64 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus${
                    shake.create ? " glim-shake" : ""
                  }`}
                />
              </div>
              <ToggleRow
                label={t("mcp.allowStart")}
                hint={t("mcp.allowStartHint")
                  .replace("{n}", String(data.startsPerHour))
                  .replace("{minutes}", String(data.cooldownMinutes))
                  .replace("{perDay}", String(data.itemStartsPerDay))}
                checked={allowStart}
                onChange={setAllowStart}
                hueIndex={hueIndex}
              />
              <div className="flex items-center gap-3">
                <Button
                  label={t("common.cancel")}
                  labelKey="common.cancel"
                  tone="neutral"
                  onClick={() => {
                    setAdding(false);
                    setLabel("");
                  }}
                  hueIndex={hueIndex}
                />
                <Button
                  label={t("mcp.createKey")}
                  labelKey="mcp.createKey"
                  tone="accent"
                  onClick={() => void create()}
                  disabled={busy}
                  busy={busy}
                  hueIndex={hueIndex}
                />
              </div>
            </div>
          )}

          {revoked.length > 0 && (
            <div className="flex flex-col gap-2">
              <button
                type="button"
                onClick={() => setRevokedOpen((v) => !v)}
                aria-expanded={revokedOpen}
                className="flex items-center gap-1.5 self-start text-xs text-carbon-textSub hover:text-carbon-text"
              >
                <IconDisclosure open={revokedOpen} />
                {t("mcp.revokedList", revoked.length)}
              </button>
              {revokedOpen && (
                <ul className="flex flex-col gap-2">
                  {revoked.map((k) => (
                    <li
                      key={k.id}
                      className="flex flex-wrap items-center justify-between gap-3 rounded-control bg-carbon-surface2 px-3 py-2"
                    >
                      <div className="flex min-w-0 flex-col">
                        <span className="truncate text-sm text-carbon-text">{k.label}</span>
                        <span className="text-xs text-carbon-textSub">
                          {(k.revokedReason === "config-restore"
                            ? t("mcp.revokedByRestore")
                            : t("mcp.revokedAt")
                          ).replace("{date}", formatTs(k.revokedAt))}
                        </span>
                      </div>
                      <Button
                        label={t("common.delete")}
                        labelKey="common.delete"
                        tone="neutral"
                        onClick={() => setPending({ kind: "purge", item: k })}
                        disabled={busy || k.inUse}
                        title={k.inUse ? t("mcp.inUseTip") : undefined}
                        className={shake[`purge:${k.id}`] ? "glim-shake" : ""}
                        hueIndex={hueIndex}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </>
      )}

      {pending?.kind === "certificate" && (
        <ConfirmDialog
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
        <ConfirmDialog
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
        <ConfirmDialog
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
        <ConfirmDialog
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
