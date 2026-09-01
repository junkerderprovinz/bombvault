// FlashZipExportCard, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import type { Settings } from "../../lib/api";
import { Card, ToggleRow } from "../settings/shared";
import { FolderBrowser } from "../../components/FolderBrowser";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { getSettings, putSettings } from "../../lib/api";
import { useEffect, useRef, useState } from "react";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

// ---------------------------------------------------------------------------
// Flash-ZIP-Export Card (#28) — MOVED here from Settings' Storage tab (jdp,
// two live-review messages in sequence, the second superseding/refining the
// first: "trenn bitte flash zip export und den rest wieder in zwei separate
// cards", then "soll die flash zip export toggle nicht einfach in den flash
// tab? macht doch mehr sinn" — implemented as the SECOND ask, not both: this
// setting now lives ONLY on the dedicated Flash page (pages/Flash.tsx), not
// as a Settings card at all any more). Exported from Settings.tsx and
// imported into Flash.tsx the same way Config.tsx already imports ToggleRow
// and Recovery.tsx already imports CloudCard/RcloneCard/ToggleRow from this
// same file — a real, pre-existing cross-page reuse pattern, not a new one
// invented for this move.
//
// Self-contained the SAME way VMSSHCard/RcloneCard/CloudCard below already
// are ("fetches its own data so the large SettingsPage doesn't need extra
// state") — necessarily so here, since this Card now renders OUTSIDE
// SettingsPage entirely and can't reach that component's own settings/
// savedSettings state or its save()/autoSaveField()/debouncedSave() helpers.
// Persists via the same "re-fetch the latest settings, merge only the
// fields THIS card owns, PUT" pattern Config.tsx's ConfigSettingsCard
// already established for exactly this situation (a card living on a
// domain's own page, editing a few fields of the shared Settings object) —
// see that component's own handleSave() for the reference implementation.
// Kept the ORIGINAL per-field auto-save behaviour (optimistic flip + revert-
// on-failure + shake for the two toggles, debounce for the path/keep-count
// fields) rather than switching to Config.tsx's single "Save" button: this
// feature already had, and jdp already approved, the no-Speichern-button
// auto-save UX (#142) before this move — relocating a control is not licence
// to silently redesign how it saves.
export function FlashZipExportCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [enabled, setEnabled] = useState(false);
  const [path, setPath] = useState("");
  const [keep, setKeep] = useState(0);
  // Remembers the last "keep N" the user picked so toggling history OFF
  // (which zeroes the persisted count) and back ON restores their count
  // instead of the shipped default — same behaviour, same variable name, as
  // the one this card used to keep inside SettingsPage's own state before
  // the move.
  const [rememberedKeep, setRememberedKeep] = useState(7);
  const [busyEnabled, setBusyEnabled] = useState(false);
  const [busyKeep, setBusyKeep] = useState(false);
  const [shakeEnabled, setShakeEnabled] = useState(0);
  const [shakeKeep, setShakeKeep] = useState(0);
  // Confirmation-pulse (GlimStone motion-engine animation 2) — this Card
  // owns its own local `persist()` rather than SettingsPage's shared save()
  // (see that function's own comment), so it needs its own pulse nonces
  // too, same per-toggle shape as shakeEnabled/shakeKeep above, bumped on
  // the OPPOSITE (`ok`, not `!ok`) branch of each toggle handler below.
  const [pulseEnabled, setPulseEnabled] = useState(0);
  const [pulseKeep, setPulseKeep] = useState(0);
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  const DEBOUNCE_MS = 800;

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (!res.ok) return;
        setEnabled(res.settings.flashZipExportEnabled);
        setPath(res.settings.flashZipExportPath);
        setKeep(res.settings.flashZipExportKeep);
        if (res.settings.flashZipExportKeep > 0) setRememberedKeep(res.settings.flashZipExportKeep);
        setHostMountRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
  }, []);

  // persist re-fetches the current server state and merges ONLY this card's
  // own patch onto it before PUTting — never a stale mount-time snapshot,
  // which could otherwise re-assert a field some OTHER open tab/page has
  // since changed. Mirrors Config.tsx's ConfigSettingsCard.handleSave()
  // exactly (see that component's own comment for the fuller rationale);
  // unlike that one, this fires per-field rather than from one batched
  // "Save" click, so it also mirrors SettingsPage's own save() helper's
  // return-a-boolean contract so callers can revert an optimistic flip.
  async function persist(patch: Partial<Settings>): Promise<boolean> {
    try {
      const latest = await getSettings();
      if (!latest.ok) {
        push(latest.error ?? t("settings.error"), "fail");
        return false;
      }
      const merged: Settings = { ...latest.settings, ...patch };
      const res = await putSettings(merged);
      if (res.ok) {
        push(t("settings.saved"), "success");
        return true;
      }
      push(res.error ?? t("settings.error"), "fail");
      return false;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  async function toggleEnabled(next: boolean) {
    const prev = enabled;
    setEnabled(next);
    setBusyEnabled(true);
    const ok = await persist({ flashZipExportEnabled: next });
    setBusyEnabled(false);
    if (!ok) {
      setEnabled(prev);
      setShakeEnabled((n) => n + 1);
    } else {
      setPulseEnabled((n) => n + 1);
    }
  }

  async function toggleKeepHistory(next: boolean) {
    const prev = keep;
    const nextKeep = next ? rememberedKeep : 0;
    // Retire the number field's pending debounce first. It arms 800ms with the
    // typed value and this toggle saves immediately, so a flip inside that
    // window lost the race by construction: the toggle wrote keep=0, the timer
    // then wrote the old count back over it, and both reported success. The UI
    // showed history off while the server kept it.
    cancelKeepDebounce();
    setKeep(nextKeep);
    setBusyKeep(true);
    const ok = await persist({ flashZipExportKeep: nextKeep });
    setBusyKeep(false);
    if (!ok) {
      setKeep(prev);
      setShakeKeep((n) => n + 1);
    } else {
      setPulseKeep((n) => n + 1);
    }
  }

  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, DEBOUNCE_MS);
  }

  // Lets an IMMEDIATE save beat a pending debounced one — see
  // toggleKeepHistory, which is the only caller and the reason this exists.
  function cancelKeepDebounce() {
    const existing = debounceTimers.current.flashZipExportKeep;
    if (existing) {
      clearTimeout(existing);
      delete debounceTimers.current.flashZipExportKeep;
    }
  }

  return (
    <Card
      title={t("flash.zipExport.title")}
      hint={`${t("flash.zipExport.hint")} ${t("flash.zipExport.enableHint")}`}
      hueIndex={hueIndex}
    >
      {/* No-empty-toggles audit (jdp): this row used to `hideLabel` on the
          reasoning that the Card's own title/hint above already carry the
          same explanation, verbatim — the exact pattern jdp has now ruled
          out categorically ("es soll nie leere Toggles geben"), the same
          reversal already applied to the merged Colors Card's master
          "Regenbogen-Modus" row and RestoreChecksSection's "Automatische
          Restore-Prüfungen" row. The row's own label is visible again. */}
      <ToggleRow
        label={t("flash.zipExport.enable")}
        checked={enabled}
        onChange={(v) => void toggleEnabled(v)}
        disabled={busyEnabled}
        shakeNonce={shakeEnabled}
        pulseNonce={pulseEnabled}
      />
      {enabled && (
        <>
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("flash.zipExport.plaintextWarn")}
          </div>
          <FolderBrowser
            label={t("flash.zipExport.path")}
            value={path}
            hostMountRoot={hostMountRoot}
            hint={t("flash.zipExport.pathHint")}
            onChange={(v) => {
              setPath(v);
              debounced("flashZipExportPath", () => void persist({ flashZipExportPath: v }));
            }}
          />
          {!path.trim() && (
            <p className="text-xs text-statusFail -mt-1">{t("flash.zipExport.pathRequired")}</p>
          )}
          <ToggleRow
            label={t("flash.zipExport.keepHistory")}
            hint={t("flash.zipExport.keepHistoryHint")}
            // History is "on" whenever we keep more than a single overwritten zip.
            checked={keep > 0}
            onChange={(v) => void toggleKeepHistory(v)}
            disabled={busyKeep}
            shakeNonce={shakeKeep}
            pulseNonce={pulseKeep}
          />
          {keep > 0 ? (
            <label className="flex flex-col gap-1 max-w-40">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t("flash.zipExport.keepN")}
                <InfoBubble tip={t("flash.zipExport.keepNHint")} />
              </span>
              <NumberField
                min={1}
                value={keep}
                onChange={(e) => {
                  const n = Math.max(1, parseInt(e.target.value, 10) || 1);
                  setRememberedKeep(n);
                  setKeep(n);
                  debounced("flashZipExportKeep", () => void persist({ flashZipExportKeep: n }));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
              />
            </label>
          ) : (
            <p className="text-xs text-carbon-textMuted">{t("flash.zipExport.latestNote")}</p>
          )}
        </>
      )}
    </Card>
  );
}
