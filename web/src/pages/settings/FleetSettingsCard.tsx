// FleetSettingsCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

// FleetSettingsCard manages this instance's own identity for the Fleet view:
// the display name reported to polling peers, and the peer status token (GET
// /api/fleet/status) that authorizes OTHER instances to poll THIS one. The
// token follows the exact same show-once secret contract as the widget token
// (generate/rotate/disable, never echoed back after the fact).
import { Button } from "../../components/Button";
import { RevealInput } from "../../components/RevealInput";
import { IconClose } from "../../components/Sidebar";
// From glyphs, where glyphFor gets it too: Sidebar re-exports a hand-picked
// subset of navGlyphs and IconRefresh is in neither ([412]).
import { IconRefresh } from "../../components/glyphs";
import { Settings, disableFleetToken, generateFleetToken } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { Card, type SaveState } from "./shared";
import { useRef, useState } from "react";

export function FleetSettingsCard({
  t,
  settings,
  setSettings,
  save,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  // Only the setters are needed post-conversion (see instanceName's own
  // onChange below) — save()'s toast already reports the outcome, same
  // "only setters needed" shape as SettingsPage's own setDomSaveState/
  // setDomSaveError.
  const [, setNameSaveState] = useState<SaveState>("idle");
  const [, setNameSaveError] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): the
  // toast below already fired on every failure, but nothing ever bumped a
  // shake nonce, so the Generate/Regenerate/Disable buttons never played
  // `.glim-shake` — same gap, same fix, as the sibling DashboardWidgetCard
  // above (see its own comment for the "mutually-exclusive buttons share one
  // key" reasoning, identical here).
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  const reveal = useReveal();
  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): instanceName used to batch into its own bottom SaveBar.
  // This Card is a standalone component with no access to SettingsPage's own
  // shared `debouncedSave` (only the generic `save` prop crosses that
  // boundary), so it gets its own local debounce timer — the exact same
  // local mechanism FlashZipExportCard already established for its own
  // path/keep-count fields, for the identical reason (a self-contained Card
  // that can't reach the page's own debounce helper).
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // GlimStone follow-up pass (v8.0.0): the persistent "✗ {error}" banner
  // (never auto-cleared; only reset by the next generate/disable attempt) is
  // now a toast — a generate/disable outcome is the same one-shot completion
  // notice handleSetPassword's own migration already established the pattern
  // for, above.
  async function handleGenerate() {
    setBusy(true);
    try {
      const r = await generateFleetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("generate");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("generate");
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    try {
      const r = await disableFleetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("disable");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("disable");
    } finally {
      setBusy(false);
    }
  }

  // copyText falls back to execCommand in non-secure contexts (#112). Mirrors
  // VMSSHCard's own handleCopy migration: the local button-label swap is now
  // a routine (quiet-mode-suppressible) toast — see lib/toast.tsx.
  async function handleCopy() {
    if (!token) return;
    if (await copyText(token)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  // Button-size/colour-engine sweep (jdp, live review — see VMSSHCard's own
  // identical comment for the full reasoning): Generate/Regenerate/Disable/
  // copy were already at this page's dominant 32px control height but had no
  // tie to this Card's own hueIndex.
  const hueOn = hueIndex !== undefined;

  return (
    <Card title={t("settings.fleet")} hint={t("settings.fleetHint")} hueIndex={hueIndex}>

      <div className="flex flex-col gap-1.5">
        <label className="text-xs text-carbon-textSub">{t("settings.instanceName")}</label>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={settings.instanceName}
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => (prev ? { ...prev, instanceName: v } : prev));
              debounced("instanceName", () => void save({ instanceName: v }, setNameSaveState, setNameSaveError));
            }}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower"
            className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
          />
        </div>
      </div>

      <ul className="list-disc ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.fleetHow")}</li>
        <li>{t("settings.fleetAccess")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetToken")}</span>
          {/* Actions on their own line below the field, not beside it —
              same reveal-eye layout rule as DashboardWidgetCard above. */}
          <RevealInput
            {...reveal}
            readOnly
            value={token ?? ""}
            placeholder={token ? "" : t("cloud.secretSet")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
          />
          <div className="flex items-center gap-2">
            <Button
              label={t("settings.fleetRegenerate")}
              labelKey="settings.fleetRegenerate"
              // A circular arrow. Was IconSync ([322]); IconRefresh since
              // jdp read it as too big next to the ✕ beside it ([412]).
              //
              // The sizing system was not at fault and scaling would have been
              // the wrong fix. Measured on the live card: both buttons are
              // 48x32 with identical 6px margins, and every glyph on the page
              // fills ~100% of its viewBox, which is exactly what the contact
              // sheet at /glyphs checks for (it flags anything under 90%).
              //
              // What differs is MASS, which that check cannot see. Painted
              // area at equal size: IconSync 41.2%, IconClose 24.5% — the ✕ is
              // a thin diagonal, the sync ring a dense full-height vertical
              // form, so side by side one outweighs the other by 1.7x while
              // both measure "100% of the box".
              //
              // IconRefresh says the same thing (two arrows, a loop, do it
              // again), lies horizontally rather than filling the button's
              // height, and is lighter at 38.5%. It was otherwise reachable
              // only through glyphFor's refresh|reload rule.
              glyph={<IconRefresh />}
              // accent, not neutral ([515]). jdp: "Die beiden buttons sind
              // auch nicht farbig". Regenerating the token is the action this
              // row is for, and the accent is what this app spends on exactly
              // that. It never competes with "Token erzeugen": that button is
              // for when there is no token, this pair for when there is.
              tone="accent"
              onClick={() => void handleGenerate()}
              disabled={busy}
              // text-carbon-text dropped with the tone change ([515]): accent brings
              // its own ink, and restating the neutral one here would paint the
              // wrong colour on the accent fill.
              className={`shrink-0 rounded-control px-3 py-2 text-xs disabled:opacity-50${
                shake.generate ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              hueIndex={hueIndex}
            />
            <Button
              label={t("settings.fleetDisable")}
              labelKey="settings.fleetDisable"
              // danger ([515]). Not decoration: the card's own sentence says
              // "Deaktivieren widerruft ihn sofort", and a control that revokes
              // access without warning should not look like the one beside it
              // that hands out a new key.
              tone="danger"
              // The same X as every other dismissal ([323]) — the hand-drawn
              // cross from [287], not a second one.
              glyph={<IconClose />}
              onClick={() => void handleDisable()}
              disabled={busy}
              // text-carbon-text dropped with the tone change ([515]): danger
              // brings its own ink, and a call site restating the neutral one
              // would paint dark text on the fail fill.
              className={`shrink-0 rounded-control px-3 py-2 text-xs disabled:opacity-50${
                shake.disable ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
              hueIndex={hueIndex}
            />
          </div>
        </div>
      ) : (
        <Button
          label={t("settings.fleetGenerate")}
          labelKey="settings.fleetGenerate"
          tone="accent"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className={`self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake.generate ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
          hueIndex={hueIndex}
        />
      )}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.fleetTokenOnce")}</p>
      )}
      {token && (
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetTokenPasteHint")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {token}
            </code>
            <Button
              label={t("vm.ssh.copy")}
              labelKey="vm.ssh.copy"
              tone="accent"
              onClick={() => void handleCopy()}
              className={`shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast${hueOn ? " glim-hue" : ""}`}
              hueIndex={hueIndex}
            />
          </div>
          <p className="text-caption text-carbon-textMuted">
            {t("settings.fleetUrlHint").replace("{url}", window.location.origin)}
          </p>
        </div>
      )}
    </Card>
  );
}

// RcloneCard manages the off-site rclone config (paste rclone.conf). It is
// stored encrypted; only the remote NAMES are read back for display. Backup
// paths can then be set to "rclone:<remote>:<bucket>" in Backup Paths.
//
// GENUINE EXCEPTION to the full-page Speichern-Button sweep (jdp, live
// review, emphatic: "Die Speicher-Buttons sollen in allen Tabs weg. Überall
// soll es automatisch speichern. Nur dort sollen Speicher-Buttons bleiben,
// wo es unbedingt sein muss."). Every other manual Save on this page got
// converted; this one stays, for a reason specific to this field's shape,
// not "it's a text field" (debounced auto-save already handles those fine
// everywhere else): the textarea below is a WRITE-ONLY one-shot paste — it
// never round-trips the actual stored rclone.conf (`conf` starts blank on
// every load and is blanked again after a successful save; `remotes` above
// is a SEPARATE read-only summary fetched fresh from the server), so there
// is no live "current value" here to keep in sync the way autoSaveField's
// contract assumes. setRclone() also REPLACES the whole config wholesale,
// not a partial PATCH — auto-saving mid-paste/mid-edit would push a
// momentarily incomplete or invalid TOML blob live, and a scheduled off-site
// job that happens to run in that exact window would see a broken config
// instead of the working one it had a moment ago. This matches the "a
// multi-step DRAFT of something not meant to take effect until deliberately
// applied" exception named in the sweep's own criteria — it's the one
// genuine case of that shape actually present in this file.
