import { useEffect, useState } from "react";
import { setContainerExcludes, previewContainerExcludes, suggestContainerExcludes } from "../../lib/api";
import type { ExcludePreset, ExcludeSuggestion } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { InfoBubble } from "../InfoBubble";
import { Button } from "../Button";
import { tLtr, withLtrFragments, withLtrPlaceholder, EXCLUDES_HINT_LTR_FRAGMENTS } from "../../lib/ltrFragments";
import { useToast } from "../../lib/toast";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { IconSearch } from "../glyphs";
import { ExcludePresetPanel } from "../ExcludePresetPanel";

type T = ReturnType<typeof useT>["t"];

// formatSnapshotWhen renders a snapshot's RFC3339 timestamp for the assistant's
// source line. An unparseable value is shown as it came, not as "Invalid Date".
function formatSnapshotWhen(rfc3339: string): string {
  const d = new Date(rfc3339);
  return Number.isNaN(d.getTime()) ? rfc3339 : d.toLocaleString();
}

// Past this age the assistant says that its list describes the past: a folder
// created since the last backup has no row in a snapshot list. One day,
// because most installs back up nightly.
const SNAPSHOT_STALE_MS = 24 * 60 * 60 * 1000;

// An unparseable timestamp is never stale: that would warn about a date nobody
// can read.
function snapshotIsStale(rfc3339: string): boolean {
  const d = new Date(rfc3339);
  if (Number.isNaN(d.getTime())) return false;
  return Date.now() - d.getTime() > SNAPSHOT_STALE_MS;
}

// advisoryKey maps an advisory id from the server to its sentence. An unknown
// id renders nothing: a newer server may know caveats this interface does not,
// and a raw id on screen would be worse than silence.
function advisoryKey(id: string): TranslationKey | null {
  if (id === "immich-db-separate") return "excludes.advisoryImmichDb";
  if (id === "nextcloud-db-separate") return "excludes.advisoryNextcloudDb";
  return null;
}

// liveSourceKey picks the sentence that says why the sizes came from a folder
// scan. "No backup yet" is true for only one of the three reasons.
function liveSourceKey(reason: "no-snapshot" | "requested" | "not-in-snapshot"): TranslationKey {
  if (reason === "requested") return "excludes.assistSourceLiveRequested";
  if (reason === "not-in-snapshot") return "excludes.assistSourceLiveNotInSnapshot";
  return "excludes.assistSourceLive";
}

export type ExcludePreviewRow = { raw: string; resolved: string; status: string; matches: boolean };

// How a caller resolves candidate lines. Each domain has its own endpoint and
// its own answer shape, so the adapter rather than the raw call is the prop.
export type ExcludePreview = (name: string, lines: string[]) => Promise<ExcludePreviewRow[]>;

const containerExcludePreview: ExcludePreview = async (name, lines) => {
  const r = await previewContainerExcludes(name, lines);
  return r.ok ? r.preview : [];
};

// ExcludesEditor edits this container's restic exclude patterns, one per line,
// and previews how each line resolves against the container's mounts. The
// caller controls `open`.
export function ExcludesEditor({
  name,
  initial,
  open,
  t,
  preview: resolvePreview = containerExcludePreview,
}: {
  name: string;
  initial: string[];
  open: boolean;
  t: T;
  preview?: ExcludePreview;
}) {
  const [text, setText] = useState(initial.join("\n"));
  const [state, setState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  const [preview, setPreview] = useState<ExcludePreviewRow[]>([]);
  const { debouncedSave, cancel: cancelPendingSave } = useDebouncedSave();

  useEffect(() => {
    if (!open) return;
    const lines = text.split("\n").map((s) => s.trim()).filter(Boolean);
    if (lines.length === 0) {
      setPreview([]);
      return;
    }
    let cancelled = false;
    const id = setTimeout(() => {
      resolvePreview(name, lines)
        .then((rows) => {
          if (!cancelled) setPreview(rows);
        })
        .catch(() => {
          if (!cancelled) setPreview([]);
        });
    }, 400);
    return () => {
      cancelled = true;
      clearTimeout(id);
    };
  }, [text, name, open, resolvePreview]);

  // The lines as the textarea holds them, unsaved edits included. The
  // assistant's one-click actions build on these, so they cannot diverge from
  // what is typed.
  const currentLines = text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);

  async function saveLines(list: string[]) {
    setState("saving");
    try {
      const r = await setContainerExcludes(name, list);
      if (r.ok) {
        setText(list.join("\n"));
        push(t("excludes.saved"), "success");
      } else {
        push(r.error ?? t("excludes.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("excludes.error"), "fail");
    } finally {
      setState("idle");
    }
  }

  const [assistOpen, setAssistOpen] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [suggestions, setSuggestions] = useState<ExcludeSuggestion[] | null>(null); // null = not scanned yet
  const [truncated, setTruncated] = useState(false);
  // Where the sizes came from. A snapshot's sizes are exact but as of
  // snapshotTime, which is why the timestamp is shown next to the list. A live
  // scan is current but can stop early, inside stoppedAt.
  const [source, setSource] = useState<"snapshot" | "live">("live");
  // Why the live walk ran; liveSourceKey has a sentence for each reason.
  const [liveReason, setLiveReason] = useState<"no-snapshot" | "requested" | "not-in-snapshot">(
    "no-snapshot"
  );
  const [snapshotTime, setSnapshotTime] = useState("");
  const [stoppedAt, setStoppedAt] = useState("");
  // Backup folders the walk never opened (its budget went on an earlier one) and
  // ones it could not read at all. Neither can be expressed by a per-row flag:
  // a folder that produced no rows has nothing to flag, and with several roots
  // an unmentioned one reads as a finished scan of all of them (#175).
  const [unexamined, setUnexamined] = useState<string[]>([]);
  const [unreadable, setUnreadable] = useState<string[]>([]);
  // The folders are configured but none is reachable (an unmounted array or
  // share) and there was no backup to read instead, so "nothing left to
  // exclude" would be a false finding.
  const [pathsUnavailable, setPathsUnavailable] = useState(false);
  // Caveats about the container itself, which the folder scan cannot see. Kept
  // apart from the suggestion list: they hold whether or not an exclusion is
  // offered, and the most important one holds when the list is empty.
  const [advisories, setAdvisories] = useState<string[]>([]);
  const [preset, setPreset] = useState<ExcludePreset | null>(null);
  // The backup index could not be read. Not a failed scan: the panel stays up
  // and offers the folder scan as an explicit second request.
  const [indexFailed, setIndexFailed] = useState(false);
  // Tells "the scan failed" from "scanned, found nothing" for the hint below.
  const [scanFailed, setScanFailed] = useState(false);
  const [shakeScan, setShakeScan] = useState(0);

  async function scan(live = false) {
    setScanning(true);
    setScanFailed(false);
    setIndexFailed(false);
    // Each "why is this list short" fact describes the scan that produced the
    // list on screen. A failed rescan leaves no list, so the previous run's
    // facts are cleared up front.
    setTruncated(false);
    setStoppedAt("");
    setUnexamined([]);
    setUnreadable([]);
    setPathsUnavailable(false);
    setAdvisories([]);
    setPreset(null);
    try {
      const r = await suggestContainerExcludes(name, live ? "live" : undefined);
      if (r.ok) {
        setSuggestions(r.suggestions);
        setTruncated(r.truncated);
        setSource(r.source ?? "live");
        // A live scan can only be asked for when a backup was there to read,
        // so its fallback is "requested" and never the "no backup yet" reason.
        setLiveReason(r.liveReason ?? (live ? "requested" : "no-snapshot"));
        setSnapshotTime(r.snapshotTime ?? "");
        setStoppedAt(r.stoppedAt ?? "");
        setUnexamined(r.unexaminedRoots ?? []);
        setUnreadable(r.unreadableRoots ?? []);
        setPathsUnavailable(r.pathsUnavailable === true);
        setIndexFailed(r.indexFailed === true);
        setAdvisories(r.advisories ?? []);
        setPreset(r.preset ?? null);
      } else {
        setSuggestions([]);
        setScanFailed(true);
        push(r.error ?? t("excludes.assistScanFailed"), "fail");
        setShakeScan((n) => n + 1);
      }
    } catch (err) {
      setSuggestions([]);
      setScanFailed(true);
      push(err instanceof Error ? err.message : t("excludes.assistScanFailed"), "fail");
      setShakeScan((n) => n + 1);
    }
    setScanning(false);
  }

  function toggleAssistant() {
    const opening = !assistOpen;
    setAssistOpen(opening);
    if (opening && suggestions === null) void scan();
  }

  // The chip actions save at once, so they cancel a pending textarea save
  // first. currentLines already carries what was typed, and the dropped timer
  // would only have written an older copy of it.
  async function addExclude(line: string) {
    if (currentLines.includes(line)) return;
    cancelPendingSave();
    await saveLines([...currentLines, line]);
  }

  async function addExcludes(lines: string[]) {
    const fresh = lines.filter((l) => !currentLines.includes(l));
    if (fresh.length === 0) return;
    cancelPendingSave();
    await saveLines([...currentLines, ...fresh]);
  }

  async function removeExclude(line: string) {
    cancelPendingSave();
    await saveLines(currentLines.filter((l) => l !== line));
  }

  // A suggestion whose line is already stored disappears from the list (it shows
  // up in the current-exclusions chips instead).
  const openSuggestions = (suggestions ?? []).filter((sg) => !currentLines.includes(sg.line));

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus";

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">
        {withLtrFragments(t("excludes.hint"), EXCLUDES_HINT_LTR_FRAGMENTS)}
      </p>
      <textarea
        value={text}
        onChange={(e) => {
          const nextText = e.target.value;
          setText(nextText);
          // Parsed from nextText here: `text` in this closure still holds the
          // previous value when the timer fires.
          const nextLines = nextText.split("\n").map((s) => s.trim()).filter(Boolean);
          debouncedSave(() => void saveLines(nextLines));
        }}
        spellCheck={false}
        rows={3}
        placeholder={tLtr(t, "excludes.placeholder")}
        dir="ltr"
        className={`${inputCls} text-start`}
      />
      {preview.length > 0 && (
        <div className="flex flex-col gap-1">
          {preview.map((row, i) => {
            // A plain confirmation and not the internal restic path, whose
            // rebased host-mount form looks like an invalid path (#38). The
            // exact pattern is in the title.
            const good = row.matches;
            const msg = good
              ? row.status === "basename"
                ? t("excludes.matchesAnywhere")
                : t("excludes.willExclude")
              : row.status === "passthrough"
                ? t("excludes.noMatch")
                : t("excludes.excludesNothing");
            return (
              <div
                key={i}
                className="text-xs wrap-break-word leading-snug flex items-baseline gap-1.5"
                title={row.status === "translated" ? row.resolved : undefined}
              >
                <span dir="ltr" className="font-mono text-carbon-textSub text-start">{row.raw}</span>
                <span className={good ? "text-statusOk" : "text-statusFail"}>
                  {good ? "✓" : "⚠"} {msg}
                </span>
              </div>
            );
          })}
        </div>
      )}

      <div className="mt-1 flex flex-col gap-2">
        <Button
          label={t("excludes.assistTitle")}
          labelKey="excludes.assistTitle"
          tone="neutral"
          onClick={toggleAssistant}
          glyph={
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${assistOpen ? "rotate-90" : "rtl:rotate-180"}`}>
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
          }
        />
        {assistOpen && (
          <div className="flex flex-col gap-2">
            <p className="text-xs text-carbon-textMuted">{t("excludes.assistHint")}</p>
            <div className="flex flex-wrap items-center gap-3">
              <Button
                key={shakeScan}
                label={suggestions === null
                    ? t("excludes.assistScan")
                    : t("excludes.assistRescan")}
                labelKey={suggestions === null ? "excludes.assistScan" : "excludes.assistRescan"}
                glyph={<IconSearch />}
                tone="accent"
                onClick={() => void scan()}
                disabled={scanning}
                busy={scanning}
                title={scanning ? t("excludes.assistScanning") : undefined}
                className={shakeScan ? "glim-shake" : ""}
              />
              {/* A snapshot cannot say what is on disk at the moment: a junk
                  folder created since the last backup has no row in it. So a
                  live scan is offered whenever the list came from a backup,
                  except while indexFailed, whose branch offers the same action
                  under its own label. */}
              {!scanning && suggestions !== null && !scanFailed && !indexFailed && source === "snapshot" && (
                <Button
                  label={t("excludes.assistScanCurrent")}
                  labelKey="excludes.assistScanCurrent"
                  tone="neutral"
                  onClick={() => void scan(true)}
                />
              )}
              {truncated && !scanning && stoppedAt && (
                // A separate line from the per-row size flags: a folder the
                // walk never reached has no row to flag. It names where the
                // list ends, and the path is pinned LTR so its leading slash
                // stays in front in right-to-left languages.
                <span className="text-xs text-statusWarn">
                  {withLtrPlaceholder(t("excludes.assistTruncated"), "{path}", stoppedAt)}
                </span>
              )}
            </div>
            {!scanning &&
              advisories.map((id) => {
                const key = advisoryKey(id);
                return key ? (
                  <p key={id} className="text-xs text-statusWarn">
                    {t(key)}
                  </p>
                ) : null;
              })}
            {!scanning && preset && (
              <ExcludePresetPanel
                key={preset.app}
                preset={preset}
                currentLines={currentLines}
                saving={state === "saving"}
                onApply={(lines) => void addExcludes(lines)}
                t={t}
              />
            )}
            {!scanning && unexamined.length > 0 && (
              <p className="text-xs text-statusWarn">
                {withLtrPlaceholder(t("excludes.assistUnexamined"), "{paths}", unexamined.join(", "))}
              </p>
            )}
            {!scanning && unreadable.length > 0 && (
              <p className="text-xs text-statusWarn">
                {withLtrPlaceholder(t("excludes.assistUnreadable"), "{paths}", unreadable.join(", "))}
              </p>
            )}
            {!scanning && pathsUnavailable && (
              <p className="text-xs text-statusWarn">{t("excludes.assistPathsUnavailable")}</p>
            )}
            {!scanning && indexFailed && (
              <div className="flex items-center gap-3">
                <span className="text-xs text-statusWarn">{t("excludes.assistIndexFailed")}</span>
                <Button
                  label={t("excludes.assistScanLive")}
                  labelKey="excludes.assistScanLive"
                  tone="accent"
                  onClick={() => void scan(true)}
                />
              </div>
            )}
            {/* "Nothing left to exclude" is a finding, so it is said only
                when the scan could look. With pathsUnavailable it could not. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && !pathsUnavailable && openSuggestions.length === 0 && (
              <p className="text-xs text-carbon-textMuted">{t("excludes.assistNothingFound")}</p>
            )}
            {!scanning && openSuggestions.length > 0 && (
              <div className="flex flex-col gap-1">
                {openSuggestions.map((sg) => (
                  <div
                    key={sg.line}
                    title={sg.line}
                    className="flex items-center gap-2 rounded-control bg-carbon-surface2 px-2 py-1.5"
                  >
                    <span dir="ltr" className="min-w-0 flex-1 truncate font-mono text-xs text-carbon-text text-start">{sg.path}</span>
                    <span
                      // "Cache" is a category, not an outcome, so it takes the
                      // neutral chip. "Large" is a warning.
                      className={`inline-flex items-center rounded-pill px-2 py-0.5 text-xs font-medium ${
                        sg.reason === "large" ? "bg-statusWarnBgStrong text-statusWarn" : "bg-statusNeutralBg text-statusNeutral"
                      }`}
                    >
                      {sg.reason === "large" ? t("excludes.assistReasonLarge") : t("excludes.assistReasonCache")}
                    </span>
                    {/* A size the scan could not finish measuring is a minimum
                        and says so (#175). As a plain number a 55 GB folder
                        would read as 5.7 GB. */}
                    {sg.complete ? (
                      <span className="text-xs text-carbon-textSub whitespace-nowrap">{humanBytes(sg.sizeBytes)}</span>
                    ) : (
                      // Same tone as the exact figure, so the caveat is not
                      // the least legible text in the row. The explanation is
                      // an InfoBubble because a title is invisible on touch.
                      <span className="inline-flex items-center gap-1 whitespace-nowrap">
                        <span className="text-xs text-carbon-textSub">
                          {t("excludes.assistSizeAtLeast").replace("{size}", humanBytes(sg.sizeBytes))}
                        </span>
                        <InfoBubble tip={t("excludes.assistSizeMinimumTip")} />
                      </span>
                    )}
                    <Button
                      label={t("excludes.assistExclude")}
                      labelKey="excludes.assistExclude"
                      tone="accent"
                      onClick={() => void addExclude(sg.line)}
                      disabled={state === "saving"}
                    />
                  </div>
                ))}
              </div>
            )}
            {/* A snapshot size is exact but as of that backup, and a cache may
                have grown since, so the line states the date. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && (
              <p className="text-xs text-carbon-textMuted">
                {source === "snapshot"
                  ? t("excludes.assistSourceSnapshot").replace("{when}", formatSnapshotWhen(snapshotTime))
                  : t(liveSourceKey(liveReason))}
              </p>
            )}
            {/* A snapshot list cannot show a folder that did not exist when
                the backup ran, so past a certain age the panel says so and
                does not leave the user to compare timestamps. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && source === "snapshot" && snapshotIsStale(snapshotTime) && (
              <p className="text-xs text-statusWarn">{t("excludes.assistSnapshotStale")}</p>
            )}
            <p className="text-xs text-carbon-textSub">{t("excludes.assistCurrent")}</p>
            {currentLines.length === 0 ? (
              <p className="text-xs text-carbon-textMuted">{t("excludes.assistNoneYet")}</p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {currentLines.map((line) => (
                  <span
                    key={line}
                    className="inline-flex items-center gap-1.5 rounded-pill bg-carbon-surface2 px-2 py-0.5 text-xs font-mono text-carbon-textSub"
                  >
                    {line}
                    <Button
                      label={t("excludes.assistRemoveLine").replace("{line}", line)}
                      labelKey="excludes.assistRemoveLine"
                      variant="chip"
                      onClick={() => void removeExclude(line)}
                      disabled={state === "saving"}
                      title={t("excludes.assistRemove")}
                    />
                  </span>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
