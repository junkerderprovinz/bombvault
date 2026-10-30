import { useCallback, useEffect, useRef, useState } from "react";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { ConfirmPrompt } from "../../components/ConfirmPrompt";
import { RevealInput } from "../../components/RevealInput";
import { Toggle } from "../../components/Toggle";
import { IconLog } from "../../components/glyphs";
import { IconAdd, IconCopy } from "../../components/navGlyphs";
import { Rows, SettingRow } from "../../components/storage/rows";
import { InstanceWindow } from "../instances/InstanceWindow";
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
  explainedError,
} from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT, type TranslationKey } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { McpKeyLog, type KeyLogWording } from "./McpKeyLog";
import { Card, SubCard, useOpenPasswordField } from "./shared";

// ApiTokensCard hands out the tokens scripts and home automation use for the
// API under /api/v1. A token is stored like an MCP key, so its row and its log
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
  const ownLogin = useOwnLogin();
  const openPasswordField = useOpenPasswordField();
  const [data, setData] = useState<ApiTokensResponse | null>(null);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const [fresh, setFresh] = useState<Fresh | null>(null);
  const [keyVisible, setKeyVisible] = useState(true);
  const [label, setLabel] = useState("");
  const [allowStart, setAllowStart] = useState(false);
  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState("");
  const [draft, setDraft] = useState("");
  const [pending, setPending] = useState<Pending | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});
  const [logOf, setLogOf] = useState<McpKeyView | null>(null);

  const bumpShake = (id: string) => setShake((s) => ({ ...s, [id]: (s[id] ?? 0) + 1 }));

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
        if (!ownLogin.handled(res)) push(refusal(res), "fail");
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
      setCreating(false);
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
    let refused: string | undefined;
    try {
      const res = await updateApiToken(item.id, { canStartBackups: on });
      if (res.ok) {
        push(t("api.permissionSaved"), "success");
        return;
      }
    } catch (err) {
      // A refused save and an unreachable server leave the same wrong switch
      // on screen, so both take the way back below.
      refused = explainedError(err);
    } finally {
      setBusy(false);
    }
    show(!on);
    bumpShake(`start:${item.id}`);
    push(refused ?? t("common.saveFailed"), "fail");
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
    <Card
      title={t("api.title")}
      hint={tLtr(t, "api.hint")}
      hueIndex={hueIndex}
      foot={
        data !== null &&
        !failed &&
        canMint && (
          <Button
            label={t("api.createToken")}
            labelKey="api.createToken"
            glyph={<IconAdd />}
            onClick={() => setCreating(true)}
            disabled={busy || tokens.length >= limit}
            hint={tokens.length >= limit ? t("api.limitReached", limit) : undefined}
          />
        )
      }
    >
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
                onClick={openPasswordField}
                className="glim-btn-wrap"
                hueIndex={hueIndex}
              />
            </div>
          )}

          {!canMint && (
            <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              {t("api.needsPasswordForHost").replace("{host}", host)}
            </p>
          )}

          <SettingRow label={t("api.addressLabel")} hint={tLtr(t, "api.addressHint")}>
            <input
              readOnly
              value={address}
              aria-label={t("api.addressLabel")}
              dir="ltr"
              className="w-80 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-xs text-carbon-text glim-field-focus"
            />
            <Button label={t("common.copy")} labelKey="common.copy" glyph={<IconCopy />} onClick={() => void copy(address)} />
          </SettingRow>
        </>
      )}

      {/* Outside the load gate: the server keeps no copy of this token, so a
          failed reload must not hide it. */}
      {fresh !== null && (
        <SettingRow label={t("api.newTokenTitle")} hint={t("mcp.showOnce")}>
          <RevealInput
            visible={keyVisible}
            onToggleVisible={() => setKeyVisible((v) => !v)}
            showLabel={t("common.showValue")}
            hideLabel={t("common.hideValue")}
            value={fresh.key}
            readOnly
            aria-label={t("api.newTokenTitle")}
            wrapperClassName="w-80 max-w-full"
            className="rounded-control bg-carbon-surface2 px-3 py-1.5 font-mono text-sm text-carbon-text glim-field-focus"
          />
          <Button label={t("api.copyToken")} labelKey="common.copy" tone="subtle" onClick={() => void copy(fresh.key)} hueIndex={hueIndex} />
          <Button label={t("mcp.dismissKey")} labelKey="mcp.dismissKey" tone="accent" onClick={() => setFresh(null)} hueIndex={hueIndex} />
        </SettingRow>
      )}

      {data !== null && !failed && tokens.length > 0 && (
        <Rows as="ul">
          {tokens.map((k) => (
            <SettingRow
              key={k.id}
              as="li"
              label={k.label}
              name={
                renaming === k.id ? (
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
                    className={`w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1 text-sm text-carbon-text glim-field-focus${
                      shake[`rename:${k.id}`] ? " glim-shake" : ""
                    }`}
                  />
                ) : (
                  <>
                    <span className="min-w-0 wrap-anywhere">{k.label}</span>
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
                )
              }
              note={
                <>
                  {k.unusable === "app-key-changed" ? (
                    <span className="text-statusWarn">{t("mcp.keyUnusable")}</span>
                  ) : (
                    <>
                      <span>
                        {t("mcp.keyHint").replace("{hint}", k.hint)}
                        {" · "}
                        {k.rotatedAt > 0
                          ? t("mcp.keyRotated").replace("{date}", formatTs(k.rotatedAt))
                          : t("mcp.keyCreated").replace("{date}", formatTs(k.createdAt))}
                      </span>
                      <span className="max-md:wrap-anywhere">
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
                    <Badge tone="neutral" size="medium" wrap>
                      {k.canStartBackups ? t("mcp.canStart") : t("mcp.readOnly")}
                    </Badge>
                    <Badge tone="neutral" size="medium" wrap>
                      <span className="glim-num">{t("mcp.callsToday", k.callsToday)}</span>
                    </Badge>
                  </span>
                  <Toggle
                    key={shake[`start:${k.id}`] ?? 0}
                    label={t("mcp.allowStart")}
                    checked={k.canStartBackups}
                    onChange={(v) => void setCanStart(k, v)}
                    disabled={busy}
                    className={shake[`start:${k.id}`] ? "glim-shake" : ""}
                  />
                </>
              }
            >
              <Button label={t("mcp.log")} labelKey="mcp.log" tone="neutral" glyph={<IconLog />} onClick={() => setLogOf(k)} />
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
            </SettingRow>
          ))}
        </Rows>
      )}

      {data !== null && !failed && (
        <SettingRow label={t("api.description")} hint={tLtr(t, "api.descriptionHint")}>
          <Button
            label={t("api.viewDescription")}
            labelKey="api.viewDescription"
            onClick={() => window.open(data.openapiPath, "_blank", "noopener")}
          />
        </SettingRow>
      )}

      {creating && (
        <InstanceWindow
          title={t("api.createToken")}
          hint={t("api.hint")}
          onClose={() => setCreating(false)}
          footer={
            <>
              <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={() => setCreating(false)} />
              <Button
                key={shake.create ?? 0}
                label={t("api.createToken")}
                labelKey="api.createToken"
                tone="accent"
                onClick={() => void create()}
                disabled={busy}
                busy={busy}
                className={shake.create ? "glim-shake" : ""}
              />
            </>
          }
        >
          <Rows>
            <SettingRow label={t("api.labelLabel")}>
              <input
                id="bv-api-token-label"
                value={label}
                maxLength={64}
                autoFocus
                aria-label={t("api.labelLabel")}
                placeholder={t("api.labelPlaceholder")}
                onChange={(e) => setLabel(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void create();
                }}
                className="w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
              />
            </SettingRow>
            <SettingRow label={t("mcp.allowStart")} hint={allowStartHint}>
              <Toggle hideLabel label={t("mcp.allowStart")} checked={allowStart} onChange={setAllowStart} />
            </SettingRow>
          </Rows>
        </InstanceWindow>
      )}

      {data !== null && !failed && revoked.length > 0 && (
        <SubCard title={t("api.revokedList", revoked.length)}>
          <Rows as="ul">
            {revoked.map((k) => (
              <SettingRow
                key={k.id}
                as="li"
                label={k.label}
                note={t(k.revokedReason === "config-restore" ? "mcp.revokedByRestore" : "mcp.revokedAt").replace(
                  "{date}",
                  formatTs(k.revokedAt)
                )}
              >
                <Button label={t("mcp.log")} labelKey="mcp.log" tone="neutral" glyph={<IconLog />} onClick={() => setLogOf(k)} />
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
              </SettingRow>
            ))}
          </Rows>
        </SubCard>
      )}

      {logOf !== null && (
        <InstanceWindow
          title={logOf.label}
          onClose={() => setLogOf(null)}
          footer={<Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={() => setLogOf(null)} />}
        >
          <McpKeyLog keyId={logOf.id} used={logOf.lastUsedAt > 0} t={t} wording={TOKEN_WORDING} />
        </InstanceWindow>
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
      {ownLogin.dialog}
    </Card>
  );
}
