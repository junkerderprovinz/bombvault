import { useState } from "react";
import { Button } from "../../components/Button";
import { IconSave } from "../../components/glyphs";
import { SettingRow } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import { setMcpOAuth, type McpOAuthView, explainedError } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import type { TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { CopyField } from "./CopyField";

type T = (key: TranslationKey, count?: number) => string;

/** The address a cloud assistant is given: the MCP endpoint under the public
 *  address, or "" while sign-in through OAuth is not offered. */
export function connectorUrl(oauth: McpOAuthView): string {
  return oauth.active ? oauth.issuer + oauth.connectorPath : "";
}

/**
 * McpOAuthSettings is the switch for sign-in through OAuth and the public
 * address it runs under. Switching on without a stored address opens the field
 * first, because the server refuses the switch without one. The card shows it
 * under the endpoint, and the setup dialog of a cloud client shows the same
 * block as its first step.
 */
export function McpOAuthSettings({
  oauth,
  authEnabled,
  onChange,
  idPrefix,
  t,
}: {
  oauth: McpOAuthView;
  authEnabled: boolean;
  onChange: (next: McpOAuthView) => void;
  /** Keeps the field ids apart when the card and a dialog both render this. */
  idPrefix: string;
  t: T;
}) {
  const { push } = useToast();
  const [draft, setDraft] = useState(oauth.issuer);
  const [opening, setOpening] = useState(false);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const on = oauth.enabled || opening;
  const addressId = `${idPrefix}-oauth-address`;
  const connector = connectorUrl(oauth);

  async function save(enabled: boolean, issuer: string) {
    setBusy(true);
    try {
      const res = await setMcpOAuth(enabled, issuer);
      if (res.ok && res.oauth) {
        onChange(res.oauth);
        setOpening(false);
        setDraft(res.oauth.issuer);
        push(t("mcp.oauthSaved"), "success");
        return;
      }
      const reason =
        res.code === "mcp-oauth-issuer-invalid"
          ? t("mcp.oauthAddressInvalid")
          : res.code === "mcp-oauth-needs-password"
            ? t("mcp.oauthNeedsPassword")
            : (res.error ?? t("common.saveFailed"));
      push(reason, "fail");
      setShake((n) => n + 1);
    } catch (err) {
      push(explainedError(err) ?? t("common.saveFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  function toggle(next: boolean) {
    if (!next) {
      setOpening(false);
      if (oauth.enabled) void save(false, oauth.issuer);
      return;
    }
    if (oauth.issuer !== "") void save(true, oauth.issuer);
    else setOpening(true);
  }

  async function copy(text: string) {
    const ok = await copyText(text);
    push(ok ? t("common.copied") : t("vm.ssh.copyFailed"), ok ? "success" : "fail");
  }

  return (
    <div className="flex flex-col divide-y divide-carbon-border/60">
      <SettingRow
        label={t("mcp.oauthToggle")}
        hint={t("mcp.oauthHint")}
        note={authEnabled ? undefined : t("mcp.oauthNeedsPassword")}
      >
        <Toggle
          key={shake}
          label={t("mcp.oauthToggle")}
          hideLabel
          checked={on}
          onChange={toggle}
          disabled={busy || (!authEnabled && !oauth.enabled)}
          className={shake ? "glim-shake" : ""}
        />
      </SettingRow>

      {on && (
        <SettingRow label={t("mcp.oauthAddressLabel")} hint={t("mcp.oauthAddressHint")}>
          <input
            id={addressId}
            aria-label={t("mcp.oauthAddressLabel")}
            value={draft}
            dir="ltr"
            inputMode="url"
            autoComplete="url"
            spellCheck={false}
            placeholder="https://backup.example.com"
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void save(true, draft);
            }}
            className="w-72 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-xs text-carbon-text glim-field-focus"
          />
          <Button
            label={t("mcp.oauthSave")}
            labelKey="mcp.oauthSave"
            glyph={<IconSave />}
            onClick={() => void save(true, draft)}
            disabled={busy || draft.trim() === "" || (draft.trim() === oauth.issuer && oauth.enabled)}
            busy={busy}
          />
        </SettingRow>
      )}

      {connector !== "" && (
        <SettingRow label={t("mcp.oauthConnectorLabel")} hint={t("mcp.oauthConnectorHint")}>
          <CopyField value={connector} label={t("mcp.oauthConnectorLabel")} onCopy={() => void copy(connector)} t={t} />
        </SettingRow>
      )}
    </div>
  );
}
