import { useCallback, useEffect, useRef, useState } from "react";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { ConfirmPrompt } from "../../components/ConfirmPrompt";
import { IconDisclosure } from "../../components/IconDisclosure";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { Toggle } from "../../components/Toggle";
import {
  createApiToken,
  getApiTokenActivity,
  listApiTokens,
  purgeApiToken,
  revokeApiToken,
  rotateApiToken,
  updateApiToken,
  type ApiTokensResponse,
  type McpKeyView,
  type OkEnvelope,
} from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT, type TranslationKey } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { keyLogId, McpKeyLog, type KeyLogWording } from "./McpKeyLog";
import { Card, LOGIN_PASSWORD_FIELD } from "./shared";

// ApiTokensCard hands out the tokens scripts and home automation use for the
// API under /api/v1. A token is stored like an MCP key, so its tile and its log
// are the MCP card's, told with the word token.

const CODE_MESSAGE: Record<string, TranslationKey> = {
  "mcp-key-not-found": "api.notFound",
  "mcp-key-limit": "api.limitReached",
  "mcp-key-label-invalid": "api.labelInvalid",
  "mcp-key-label-taken": "api.labelTaken",
  "mcp-key-needs-password": "api.needsPasswordForHost",
};

/** The route each tool answers, so a token's log names what the script called. */
const ROUTE_OF: Record<string, string> = {
  get_health: "GET /api/v1/health",
  get_status: "GET /api/v1/status",
  get_activity: "GET /api/v1/activity",
  list_items: "GET /api/v1/items",
  list_runs: "GET /api/v1/runs",
  list_anomalies: "GET /api/v1/anomalies",
  get_anomaly: "GET /api/v1/anomalies/{id}",
  get_storage_stats: "GET /api/v1/storage/{domain}",
  start_backup: "POST /api/v1/backups",
  start_domain_backup: "POST /api/v1/backups",
  start_backup_everything: "POST /api/v1/backups/everything",
  cancel_backup: "POST /api/v1/runs/{id}/cancel",
};

const TOKEN_WORDING: KeyLogWording = {
  load: getApiTokenActivity,
  failed: "api.logFailed",
  empty: "api.logEmpty",
  keptHint: "api.logKeptHint",
  notPermitted: "api.outcomeNotPermitted",
  startLimit: "api.outcomeStartLimit",
  callName: (tool) => ROUTE_OF[tool] ?? tool,
};

type Pending = { kind: "rotate" | "revoke" | "purge"; item: McpKeyView };

type Fresh = { key: string; id: string; hint: string };

export function ApiTokensCard({ hueIndex, passwordSet }: { hueIndex?: number; passwordSet?: boolean }) {
  const { t } = useT();
  const { push } = useToast();
  const [data, setData] = useState<ApiTokensResponse | null>(null);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const [fresh, setFresh] = useState<Fresh | null>(null);
  const [keyVisible, setKeyVisible] = useState(true);
  const [label, setLabel] = useState("");
  const [allowStart, setAllowStart] = useState(false);
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
      const res = await listApiTokens();
      if (!res.ok) {
        setFailed(true);
        return;
      }
      setData(res);
      setFailed(false);
      setFresh((f) => (f && res.tokens.some((k) => k.id === f.id && k.hint === f.hint) ? f : null));
    } catch {
      setFailed(true);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload, passwordSet]);

  const host = window.location.hostname.replace(/^\[(.*)\]$/, "$1");
  const tokens = data?.tokens ?? [];
  const revoked = data?.revoked ?? [];
  const limit = data?.limit ?? 0;
  const canMint = data?.hostAllowsKeys !== false;
  const address = `${window.location.origin}${data?.basePath ?? "/api/v1/"}`;

  function refusal(res: OkEnvelope): string {
    const key = res.code ? CODE_MESSAGE[res.code] : undefined;
    if (key === "api.limitReached") return t(key, limit);
    if (key === "api.needsPasswordForHost") return t(key).replace("{host}", host);
    if (key) return t(key);
    return res.error ?? t("common.actionFailed");
  }

  async function act<T extends OkEnvelope>(id: string, call: () => Promise<T>, okMessage: string): Promise<T | null> {
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

  async function create() {
    const name = label.trim();
    if (name === "") {
      push(t("api.labelInvalid"), "fail");
      bumpShake("create");
      return;
    }
    const res = await act("create", () => createApiToken(name, allowStart), t("api.created"));
    if (res?.key && res.item) {
      setKeyVisible(true);
      setFresh({ key: res.key, id: res.item.id, hint: res.item.hint });
      setLabel("");
      setAllowStart(false);
    }
  }

  async function rotate(item: McpKeyView) {
    const res = await act(`rotate:${item.id}`, () => rotateApiToken(item.id), t("api.rotated"));
    if (res?.key && res.item) {
      setKeyVisible(true);
      setFresh({ key: res.key, id: res.item.id, hint: res.item.hint });
    }
  }

  async function rename(item: McpKeyView) {
    const next = draft.trim();
    if (next === "" || next === item.label) {
      setRenaming("");
      return;
    }
    const res = await act(`rename:${item.id}`, () => updateApiToken(item.id, { label: next }), "");
    if (res) setRenaming("");
  }

  async function setCanStart(item: McpKeyView, on: boolean) {
    const show = (v: boolean) =>
      setData((prev) =>
        prev ? { ...prev, tokens: prev.tokens.map((k) => (k.id === item.id ? { ...k, canStartBackups: v } : k)) } : prev
      );
    setBusy(true);
    show(on);
    try {
      const res = await updateApiToken(item.id, { canStartBackups: on });
      if (res.ok) {
        push(t("api.permissionSaved"), "success");
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

  const allowStartHint = data
    ? t("api.allowStartHint")
        .replace("{n}", String(data.startsPerHour))
        .replace("{minutes}", String(data.cooldownMinutes))
        .replace("{perDay}", String(data.itemStartsPerDay))
    : "";

  return (
    <Card title={t("api.title")} hint={tLtr(t, "api.hint")} hueIndex={hueIndex}>
      {failed && <p className="text-sm text-statusWarn">{t("api.loadFailed")}</p>}
      {!failed && data === null && <p className="text-sm text-carbon-textMuted">{t("folder.loading")}</p>}

      {data !== null && !failed && (
        <>
          {data.authEnabled === false && (
            <div className="flex flex-col items-start gap-2 rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              <p>{t("api.noPasswordWarning")}</p>
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
              {t("api.needsPasswordForHost").replace("{host}", host)}
            </p>
          )}

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("api.addressLabel")}
              <InfoBubble tip={tLtr(t, "api.addressHint")} />
            </span>
            <div className="flex flex-wrap items-center gap-2">
              <input
                readOnly
                value={address}
                aria-label={t("api.addressLabel")}
                dir="ltr"
                className="min-w-0 max-w-96 flex-[1_1_18rem] rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-sm text-carbon-text glim-field-focus"
              />
              <Button
                label={t("common.copy")}
                labelKey="common.copy"
                tone="subtle"
                onClick={() => void copy(address)}
                hueIndex={hueIndex}
              />
              <Button
                label={t("api.viewDescription")}
                labelKey="api.viewDescription"
                tone="subtle"
                onClick={() => window.open(data.openapiPath, "_blank", "noopener")}
                hueIndex={hueIndex}
              />
            </div>
          </div>

          {canMint && (
            <div className="flex flex-col gap-3">
              <div className="flex flex-col gap-1.5">
                <label htmlFor="bv-api-token-label" className="text-xs text-carbon-textSub">
                  {t("api.labelLabel")}
                </label>
                <input
                  key={shake.create ?? 0}
                  id="bv-api-token-label"
                  value={label}
                  maxLength={64}
                  placeholder={t("api.labelPlaceholder")}
                  onChange={(e) => setLabel(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void create();
                  }}
                  className={`w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus${
                    shake.create ? " glim-shake" : ""
                  }`}
                />
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <span className="flex items-center gap-1.5">
                  <Toggle label={t("mcp.allowStart")} checked={allowStart} onChange={setAllowStart} />
                  <InfoBubble tip={allowStartHint} />
                </span>
                <Button
                  label={t("api.createToken")}
                  labelKey="api.createToken"
                  tone="accent"
                  onClick={() => void create()}
                  disabled={busy || tokens.length >= limit}
                  busy={busy}
                  hint={tokens.length >= limit ? t("api.limitReached", limit) : undefined}
                  hueIndex={hueIndex}
                />
              </div>
            </div>
          )}
        </>
      )}

      {/* Outside the load gate: the server keeps no copy of this token, so a
          failed reload must not hide it. */}
      {fresh !== null && (
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2.5">
          <p className="flex items-center gap-1.5 text-sm font-medium text-carbon-text">
            {t("api.newTokenTitle")}
            <InfoBubble tip={t("mcp.showOnce")} />
          </p>
          <RevealInput
            visible={keyVisible}
            onToggleVisible={() => setKeyVisible((v) => !v)}
            showLabel={t("common.showValue")}
            hideLabel={t("common.hideValue")}
            value={fresh.key}
            readOnly
            aria-label={t("api.newTokenTitle")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface px-3 py-1.5 font-mono text-sm text-carbon-text glim-field-focus"
          />
          <div className="flex items-center gap-3">
            <Button
              label={t("api.copyToken")}
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

      {data !== null && !failed && tokens.length > 0 && (
        <ul className="flex flex-col gap-3">
          {tokens.map((k) => (
            <li key={k.id} className="glim-tile flex flex-col gap-3 rounded-card p-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="flex min-w-0 flex-col gap-1">
                  <div className="flex min-w-0 items-center gap-2 max-md:flex-wrap">
                    {renaming === k.id ? (
                      <input
                        autoFocus
                        value={draft}
                        maxLength={64}
                        aria-label={t("api.labelLabel")}
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
                        {t("mcp.keyHint").replace("{hint}", k.hint)}
                        {" · "}
                        {k.rotatedAt > 0
                          ? t("mcp.keyRotated").replace("{date}", formatTs(k.rotatedAt))
                          : t("mcp.keyCreated").replace("{date}", formatTs(k.createdAt))}
                      </span>
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
                    <Badge tone="neutral" size="medium" wrap className="glim-tile-raise">
                      {k.canStartBackups ? t("mcp.canStart") : t("mcp.readOnly")}
                    </Badge>
                    <Badge tone="neutral" size="medium" wrap className="glim-tile-raise">
                      <span className="glim-num">{t("mcp.callsToday", k.callsToday)}</span>
                    </Badge>
                  </span>
                </div>
                <div className="flex min-w-0 flex-wrap items-start gap-2">
                  <Button
                    label={t("mcp.log")}
                    labelKey="mcp.log"
                    tone="neutral"
                    onClick={() => toggleLog(k.id)}
                    glyph={<IconDisclosure open={logOpen.has(k.id)} />}
                    ariaExpanded={logOpen.has(k.id)}
                    ariaControls={logOpen.has(k.id) ? keyLogId(k.id) : undefined}
                  />
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
                      label={t("api.rotate")}
                      labelKey="api.rotate"
                      tone="neutral"
                      onClick={() => setPending({ kind: "rotate", item: k })}
                      disabled={busy}
                      className={shake[`rotate:${k.id}`] ? "glim-shake" : ""}
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

              {logOpen.has(k.id) && <McpKeyLog keyId={k.id} used={k.lastUsedAt > 0} t={t} wording={TOKEN_WORDING} />}
            </li>
          ))}
        </ul>
      )}

      {data !== null && !failed && revoked.length > 0 && (
        <div className="flex flex-col gap-2">
          <button
            type="button"
            onClick={() => setRevokedOpen((v) => !v)}
            aria-expanded={revokedOpen}
            className="flex items-center gap-1.5 self-start text-xs text-carbon-textSub hover:text-carbon-text pointer-coarse:min-h-(--btn-h)"
          >
            <IconDisclosure open={revokedOpen} />
            {t("api.revokedList", revoked.length)}
          </button>
          {revokedOpen && (
            <ul className="flex flex-col gap-3">
              {revoked.map((k) => (
                <li key={k.id} className="glim-tile flex flex-col gap-3 rounded-card p-3">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="flex min-w-0 flex-col gap-1">
                      <span className="truncate text-sm text-carbon-text">{k.label}</span>
                      <span className="text-xs text-carbon-textMuted">
                        {t(k.revokedReason === "config-restore" ? "mcp.revokedByRestore" : "mcp.revokedAt").replace(
                          "{date}",
                          formatTs(k.revokedAt)
                        )}
                      </span>
                    </div>
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <Button
                        label={t("mcp.log")}
                        labelKey="mcp.log"
                        tone="neutral"
                        onClick={() => toggleLog(k.id)}
                        glyph={<IconDisclosure open={logOpen.has(k.id)} />}
                        ariaExpanded={logOpen.has(k.id)}
                        ariaControls={logOpen.has(k.id) ? keyLogId(k.id) : undefined}
                      />
                      <Button
                        label={t("common.delete")}
                        labelKey="common.delete"
                        tone="neutral"
                        onClick={() => setPending({ kind: "purge", item: k })}
                        disabled={busy || k.inUse}
                        hint={k.inUse ? t("api.inUseTip") : undefined}
                        className={shake[`purge:${k.id}`] ? "glim-shake" : ""}
                        hueIndex={hueIndex}
                      />
                    </div>
                  </div>
                  {logOpen.has(k.id) && <McpKeyLog keyId={k.id} used={k.lastUsedAt > 0} t={t} wording={TOKEN_WORDING} />}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {pending?.kind === "rotate" && (
        <ConfirmPrompt
          title={t("api.rotateTitle")}
          message={t("api.rotateConfirm").replace("{name}", pending.item.label)}
          confirmLabel={t("api.rotate")}
          confirmLabelKey="api.rotate"
          cancelLabel={t("common.cancel")}
          onConfirm={() => void rotate(pending.item)}
          onCancel={() => setPending(null)}
        />
      )}

      {pending?.kind === "revoke" && (
        <ConfirmPrompt
          title={t("api.revokeTitle")}
          message={t("mcp.revokeConfirm").replace("{name}", pending.item.label)}
          confirmLabel={t("mcp.revoke")}
          confirmLabelKey="mcp.revoke"
          cancelLabel={t("common.cancel")}
          onConfirm={() =>
            void act(`revoke:${pending.item.id}`, () => revokeApiToken(pending.item.id), t("api.revoked"))
          }
          onCancel={() => setPending(null)}
        />
      )}

      {pending?.kind === "purge" && (
        <ConfirmPrompt
          title={t("api.purgeTitle")}
          message={t("mcp.purgeConfirm").replace("{name}", pending.item.label)}
          confirmLabel={t("common.delete")}
          confirmLabelKey="common.delete"
          cancelLabel={t("common.cancel")}
          onConfirm={() => void act(`purge:${pending.item.id}`, () => purgeApiToken(pending.item.id), t("api.purged"))}
          onCancel={() => setPending(null)}
        />
      )}
    </Card>
  );
}
