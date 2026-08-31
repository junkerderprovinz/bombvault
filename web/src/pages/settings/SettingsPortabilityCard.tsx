
import { Button } from "../../components/Button";
import { CheckDraw } from "../../components/CheckDraw";
import { InfoBubble } from "../../components/InfoBubble";
import { IconDownload } from "../../components/Sidebar";
import { IconUpload } from "../../components/glyphs";
import { exportSettings, importSettingsPreview, type ImportSettingsResponse, type ImportSettingsSummary } from "../../lib/api";
import { type TranslationKey, useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card, ToggleRow } from "./shared";
import { useRef, useState } from "react";

const IMPORT_GROUP_KEYS: Record<string, TranslationKey> = {
  domains: "settingsIO.group.domains",
  schedules: "settingsIO.group.schedules",
  everything: "settingsIO.group.everything",
  retention: "settingsIO.group.retention",
  offsite: "settingsIO.group.offsite",
  drills: "settingsIO.group.drills",
  digest: "settingsIO.group.digest",
  monitoring: "settingsIO.group.monitoring",
  language: "settingsIO.group.language",
  exportEncryption: "settingsIO.group.exportEncryption",
};
// SettingsPortabilityCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

export function SettingsPortabilityCard({
  t,
  hueIndex,
  applyImport,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  // Supplied by SettingsPage rather than called from here: an apply replaces
  // the whole configuration, so the page that holds the save baseline has to be
  // the one that performs it and adopts the result. See applyImportedSettings.
  applyImport: (fileText: string) => Promise<ImportSettingsResponse>;
}) {
  const { push } = useToast();
  const [includeCreds, setIncludeCreds] = useState(true);
  const [exporting, setExporting] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST, never as permanent page text, and
  // the button that triggered it replays `.glim-shake` once — the exact same
  // migration IntegrityCard/FleetSettingsCard/DashboardWidgetCard already
  // made elsewhere in this file (their own comments: "the persistent ✗
  // {error} banner... is now a toast"). Replaces the old permanent
  // exportError/importError paragraphs below each button. Export/Choose-
  // file/Import each get their own shake key so a failure in one never
  // shakes an unrelated button (same per-key nonce shape as IntegrityCard's
  // own `shake` state — see its `bumpShake` doc comment).
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  // Import is a two-step flow: pick a file → preview summary + confirm → apply.
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [importBusy, setImportBusy] = useState<"idle" | "reading" | "applying">("idle");
  const [importDone, setImportDone] = useState(false);
  // The parsed preview and the raw file text held for the confirmed apply.
  const [preview, setPreview] = useState<ImportSettingsSummary | null>(null);
  const [pendingText, setPendingText] = useState<string | null>(null);

  function resetImport() {
    setPreview(null);
    setPendingText(null);
    setImportDone(false);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  async function handleExport() {
    setExporting(true);
    // Backend-provided error text (if any) is shown verbatim BY DESIGN — the API
    // answers English and is not translated client-side.
    const err = await exportSettings(includeCreds);
    setExporting(false);
    if (err) {
      push(err, "fail");
      bumpShake("export");
    }
  }

  async function handleFilePicked(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setImportDone(false);
    setPreview(null);
    setImportBusy("reading");
    try {
      const text = await file.text();
      const res = await importSettingsPreview(text);
      if (res.ok && res.summary) {
        setPendingText(text);
        setPreview(res.summary);
      } else {
        push(res.error ?? t("settingsIO.importFailed"), "fail");
        bumpShake("chooseFile");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      bumpShake("chooseFile");
    } finally {
      setImportBusy("idle");
    }
  }

  async function handleConfirmImport() {
    if (!pendingText) return;
    setImportBusy("applying");
    try {
      const res = await applyImport(pendingText);
      if (res.ok) {
        setImportDone(true);
        setPreview(null);
        setPendingText(null);
        if (fileInputRef.current) fileInputRef.current.value = "";
      } else {
        push(res.error ?? t("settingsIO.importFailed"), "fail");
        bumpShake("import");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      bumpShake("import");
    } finally {
      setImportBusy("idle");
    }
  }

  const busy = importBusy !== "idle" || exporting;

  // Button-size/colour-engine sweep (jdp, live review — see VMSSHCard's own
  // identical comment for the full reasoning): Export/Choose file/Confirm/
  // Cancel were already at this page's dominant 32px control height, but
  // none of the four carried this Card's own `hueIndex` — flat regardless of
  // rainbow mode. Same `.glim-hue` + `hueVars(rainbowAt(hueIndex))` fix.
  const hueOn = hueIndex !== undefined;

  return (
    <Card title={t("settingsIO.title")} hint={t("settingsIO.desc")} hueIndex={hueIndex}>
      {/* EXPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.exportHeading")}
        </h3>
        {/* Was a raw hand-rolled <input type="checkbox"> + <label> — the last
            remaining one in this file (every earlier instance of the same
            anti-pattern was already converted to ToggleRow elsewhere on this
            page, e.g. NotifyCard's scheduledSummary/notifyOnUpdate/unraid
            rows above): a native checkbox never picks up the shape engine's
            --radius-control/--radius-pill tokens (stays the browser's native
            box shape in every round/soft/square mode) and never gets
            ToggleRow's shake/hue plumbing. Converted to the shared
            ToggleRow, matching every other toggle on this page. No
            `hueIndex` — this is a genuinely LONE toggle in this Card with no
            siblings of its own kind (ToggleRow's own hueIndex doc comment
            carves out exactly this case). */}
        <ToggleRow
          label={t("settingsIO.includeCreds")}
          checked={includeCreds}
          onChange={setIncludeCreds}
        />
        {includeCreds && (
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settingsIO.credsWarning")}
          </div>
        )}
        <Button
          key={shake.export || 0}
          label={t("settingsIO.exportButton")}
          labelKey="settingsIO.exportButton"
          // Accent, and the down-arrow half of the pair ([293]). Streamline
          // draws download-box-1 and upload-box-1 as one box with the arrow
          // reversed, which is exactly the "same glyph, opposite arrow" jdp
          // asked for — no mirroring transform, and no second silhouette to
          // keep in sync.
          tone="accent"
          glyph={<IconDownload />}
          onClick={() => void handleExport()}
          disabled={busy}
          busy={exporting}
          title={exporting ? t("settingsIO.exporting") : undefined}
          className={`self-start${shake.export ? " glim-shake" : ""}`}
        />
      </div>

      {/* IMPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.importHeading")}
          <InfoBubble tip={t("settingsIO.importHint")} />
        </h3>

        <input
          ref={fileInputRef}
          type="file"
          accept="application/json,.json"
          onChange={(e) => void handleFilePicked(e)}
          className="hidden"
        />
        <Button
          key={shake.chooseFile || 0}
          label={t("settingsIO.importButton")}
          labelKey="settingsIO.importButton"
          tone="accent"
          glyph={<IconUpload />}
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          hueIndex={hueIndex}
          busy={importBusy === "reading"}
          title={importBusy === "reading" ? t("settingsIO.reading") : undefined}
          className={`self-start${shake.chooseFile ? " glim-shake" : ""}`}
        />

        {/* Preview + confirmation before anything is written. */}
        {preview && (
          <div className="rounded-card bg-carbon-surface2 p-4 flex flex-col gap-3">
            <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("settingsIO.previewTitle")}
            </span>
            <dl className="flex flex-col gap-1.5 text-xs text-carbon-text">
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewExportedAt")}</dt>
                <dd className="font-mono text-end wrap-break-word">{preview.exportedAt || "-"}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewAppVersion")}</dt>
                <dd className="font-mono text-end wrap-break-word">{preview.appVersion || "-"}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewOffsiteTargets")}</dt>
                <dd dir="ltr" className="font-mono text-start">{preview.offsiteTargets}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewCredentials")}</dt>
                <dd className="text-end">
                  {preview.credentials.present
                    ? t("settingsIO.previewCredsIncluded")
                    : t("settingsIO.previewCredsNotIncluded")}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted shrink-0">{t("settingsIO.previewSettingsAreas")}</dt>
                <dd className="text-end wrap-break-word">
                  {preview.settingsGroups.length > 0
                    ? preview.settingsGroups
                        .map((g) => (IMPORT_GROUP_KEYS[g] ? t(IMPORT_GROUP_KEYS[g]) : g))
                        .join(", ")
                    : t("settingsIO.previewNone")}
                </dd>
              </div>
            </dl>
            <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
              {t("settingsIO.confirmWarning")}
            </div>
            <div className="flex items-center gap-3">
              <Button
                key={shake.import || 0}
                label={t("settingsIO.confirmButton")}
                labelKey="settingsIO.confirmButton"
                tone="accent"
                onClick={() => void handleConfirmImport()}
                disabled={busy}
                busy={importBusy === "applying"}
                title={importBusy === "applying" ? t("settingsIO.importing") : undefined}
                className={shake.import ? "glim-shake" : ""}
              />
              <Button
                label={t("settingsIO.cancel")}
          labelKey="settingsIO.cancel"
                tone="neutral"
                onClick={resetImport}
                disabled={busy}
                className={`rounded-control px-4 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50${hueOn ? " glim-hue" : ""}`}
                hueIndex={hueIndex}
              />
            </div>
          </div>
        )}

        {importDone && (
          <span className="inline-flex items-center gap-1 text-xs text-statusOk">
            <CheckDraw />
            {t("settingsIO.importSuccess")}
          </span>
        )}
      </div>
    </Card>
  );
}

// The companion dashboard-tile plugin's .plg URL + repo — shown for manual
// install when SSH is missing, and linked for transparency before installing.
// (Install itself uses a hard-coded server-side constant; these are display-only.)
