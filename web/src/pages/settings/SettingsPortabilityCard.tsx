
import { Button } from "../../components/Button";
import { CheckDraw } from "../../components/CheckDraw";
import { InfoBubble } from "../../components/InfoBubble";
import { IconExport, IconImport } from "../../components/glyphs";
import {
  excludeFromTarget,
  exportSettings,
  importSettingsPreview,
  type ImportSettingsResponse,
  type ImportSettingsSummary,
  type NewTargetExclusion,
} from "../../lib/api";
import { type TranslationKey, useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Rows, SettingRow } from "../../components/storage/rows";
import { Card, SubCard, ToggleRow } from "./shared";
import { NewTargetPreviewLines } from "../../components/placement/NewTargetQuestion";
import { domainLabel, formatList } from "../../lib/placement";
import { importProblemText } from "../../lib/importProblems";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useRef, useState, type ReactNode } from "react";

const IMPORT_GROUP_KEYS: Record<string, TranslationKey> = {
  domains: "settings.domains",
  schedules: "settingsIO.group.schedules",
  everything: "settingsIO.group.everything",
  retention: "settingsIO.group.retention",
  offsite: "settingsIO.group.offsite",
  drills: "settingsIO.group.drills",
  digest: "settingsIO.group.digest",
  monitoring: "settingsIO.group.monitoring",
  language: "settingsIO.group.language",
  exportEncryption: "settingsIO.group.exportEncryption",
  anomalies: "settingsIO.group.anomalies",
  streaming: "settingsIO.group.streaming",
  idle: "settingsIO.group.idle",
  homeAssistant: "ha.title",
  network: "mdns.title",
};

const VALUE = "text-end text-sm text-carbon-text wrap-break-word";
const VALUE_MONO = `${VALUE} font-mono`;
// SettingsPortabilityCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

export function SettingsPortabilityCard({
  t,
  hueIndex,
  applyImport,
  children,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  // Supplied by SettingsPage rather than called from here: an apply replaces
  // the whole configuration, so the page that holds the save baseline has to be
  // the one that performs it and adopts the result. See applyImportedSettings.
  applyImport: (fileText: string) => Promise<ImportSettingsResponse>;
  /** What else belongs to an export, between it and the import. */
  children?: ReactNode;
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
  const { lang } = useT();
  const [exclusions, setExclusions] = useState<Record<string, NewTargetExclusion>>({});

  function resetImport() {
    setPreview(null);
    setPendingText(null);
    setImportDone(false);
    setExclusions({});
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
    setExclusions({});
    setImportBusy("reading");
    try {
      const text = await file.text();
      const res = await importSettingsPreview(text);
      if (res.ok && res.summary) {
        setPendingText(text);
        setPreview(res.summary);
      } else {
        push(importProblemText(t, lang, res) ?? placementErrorText(t, lang, res, "settingsIO.importFailed"), "fail");
        bumpShake("chooseFile");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      bumpShake("chooseFile");
    } finally {
      setImportBusy("idle");
    }
  }

  // The file's targets exist only once the import is applied, so what their
  // question left out is written afterwards, under the ids the import keeps.
  async function excludeChosen(applied: ImportSettingsSummary) {
    let wrote = applied.placementDefaults !== null || applied.copyRules !== null;
    for (const target of applied.newTargets) {
      const ex = exclusions[target.id];
      if (!ex || (ex.identities.length === 0 && !ex.default)) continue;
      const r = await excludeFromTarget({ domain: target.domain, targetId: target.id, ...ex });
      if (r.ok) wrote = true;
      else push(placementErrorText(t, lang, r, "settings.error"), "fail");
    }
    if (wrote) placementChanged();
  }

  async function handleConfirmImport() {
    if (!pendingText || !preview) return;
    const applied = preview;
    setImportBusy("applying");
    try {
      const res = await applyImport(pendingText);
      if (res.ok) {
        setImportDone(true);
        setPreview(null);
        setPendingText(null);
        if (fileInputRef.current) fileInputRef.current.value = "";
        await excludeChosen(applied);
      } else {
        push(importProblemText(t, lang, res) ?? placementErrorText(t, lang, res, "settingsIO.importFailed"), "fail");
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

  // The card's four buttons take its hue, as every control in a hued card does.
  const hueOn = hueIndex !== undefined;

  return (
    <Card title={t("settingsIO.title")} hint={t("settingsIO.desc")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.exportHeading")}
        </h3>
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
          // The download box is the import button's upload box with the
          // arrow reversed, so the pair reads as one glyph without a
          // mirroring transform.
          tone="accent"
          glyph={<IconExport />}
          onClick={() => void handleExport()}
          disabled={busy}
          busy={exporting}
          title={exporting ? t("settingsIO.exporting") : undefined}
          className={`glim-btn-wrap self-end ${shake.export ? "glim-shake" : ""}`}
        />
      </div>

      {children}

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
          glyph={<IconImport />}
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          hueIndex={hueIndex}
          busy={importBusy === "reading"}
          title={importBusy === "reading" ? t("settingsIO.reading") : undefined}
          className={`glim-btn-wrap self-end ${shake.chooseFile ? "glim-shake" : ""}`}
        />

        {/* Preview + confirmation before anything is written. */}
        {preview && (
          <div className="flex flex-col gap-3">
            <SubCard title={t("settingsIO.previewTitle")}>
              <Rows>
                <SettingRow label={t("settingsIO.previewExportedAt")}>
                  <span className={VALUE_MONO}>{preview.exportedAt || "-"}</span>
                </SettingRow>
                <SettingRow label={t("settingsIO.previewAppVersion")}>
                  <span className={VALUE_MONO}>{preview.appVersion || "-"}</span>
                </SettingRow>
                <SettingRow label={t("settingsIO.previewOffsiteTargets")}>
                  <span dir="ltr" className={VALUE_MONO}>{preview.offsiteTargets}</span>
                </SettingRow>
                <SettingRow label={t("settingsIO.previewNamedRepos")}>
                  <span dir="ltr" className={VALUE_MONO}>{preview.namedRepos ?? 0}</span>
                </SettingRow>
                <SettingRow label={t("dest.title")}>
                  <span className={VALUE}>{preview.destinations ?? t("settingsIO.previewNotInFile")}</span>
                </SettingRow>
                <SettingRow label={t("placementDefaults.title")}>
                  <span className={VALUE}>{preview.placementDefaults ?? t("settingsIO.previewNotInFile")}</span>
                </SettingRow>
                <SettingRow label={t("settingsIO.previewCopyRules")}>
                  <span className={VALUE}>{preview.copyRules ?? t("settingsIO.previewNotInFile")}</span>
                </SettingRow>
                <SettingRow label={t("settingsIO.previewCredentials")}>
                  <span className={VALUE}>
                    {preview.credentials.present
                      ? t("settingsIO.previewCredsIncluded")
                      : t("settingsIO.previewCredsNotIncluded")}
                  </span>
                </SettingRow>
                <SettingRow label={t("settingsIO.previewSettingsAreas")}>
                  <span className={VALUE}>
                    {preview.settingsGroups.length > 0
                      ? preview.settingsGroups
                          .map((g) => (IMPORT_GROUP_KEYS[g] ? t(IMPORT_GROUP_KEYS[g]) : g))
                          .join(", ")
                      : t("settingsIO.previewNone")}
                  </span>
                </SettingRow>
              </Rows>
            </SubCard>
            {preview.leftOut && preview.leftOut.length > 0 && (
              <p className="text-sm text-carbon-textMuted">
                {t("settingsIO.previewLeftOut").replace("{sections}", () =>
                  formatList(lang, [...new Set(preview.leftOut!.map((l) => l.domain))].map((d) => domainLabel(t, d)))
                )}
              </p>
            )}
            {preview.newTargets.map((target) => (
              <div key={target.id} className="flex flex-col gap-1">
                <span className="text-xs font-semibold text-carbon-textSub">
                  {t("newTarget.importHeading")
                    .replace("{target}", () => target.name)
                    .replace("{domain}", () => domainLabel(t, target.domain))}
                </span>
                <NewTargetPreviewLines
                  domain={target.domain}
                  target={target.name}
                  preview={target.preview}
                  exclusion={exclusions[target.id]}
                  onExclusion={(next) => setExclusions((prev) => ({ ...prev, [target.id]: next }))}
                />
              </div>
            ))}
            <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
              {t("settingsIO.confirmWarning")}
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              <Button
                label={t("settingsIO.cancel")}
                labelKey="settingsIO.cancel"
                tone="neutral"
                onClick={resetImport}
                disabled={busy}
                className={`rounded-pill px-4 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50 ${hueOn ? "glim-hue" : ""}`}
                hueIndex={hueIndex}
              />
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
