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

// formatSnapshotWhen renders a snapshot's RFC3339 timestamp for the exclusion
// assistant's source line. The date is load-bearing there: it is what turns
// "sizes as of the last backup" from a misleading number into an honest one.
// An unparseable value falls back to the raw string rather than "Invalid Date".
function formatSnapshotWhen(rfc3339: string): string {
  const d = new Date(rfc3339);
  return Number.isNaN(d.getTime()) ? rfc3339 : d.toLocaleString();
}

// SNAPSHOT_STALE_MS — past this age the assistant says out loud that its list
// describes the past. The date alone is not enough: a user reading "sizes come
// from the backup of <date>" still has to notice the date is not today, and the
// case that matters (a cache that exploded since the last backup, a junk folder
// created yesterday) is invisible in a snapshot list — no row, no warning. One
// day is the threshold because the schedule most installs run is nightly, so
// anything older than that means the backup the panel is quoting is not even
// the one the user thinks they made last night.
const SNAPSHOT_STALE_MS = 24 * 60 * 60 * 1000;

// snapshotIsStale reports whether a snapshot timestamp is old enough to warrant
// the "check the folders as they are now" nudge. An unparseable value is never
// stale — it would be a warning about a date nobody can read.
function snapshotIsStale(rfc3339: string): boolean {
  const d = new Date(rfc3339);
  if (Number.isNaN(d.getTime())) return false;
  return Date.now() - d.getTime() > SNAPSHOT_STALE_MS;
}

// liveSourceKey picks the sentence that says WHY the sizes came from a folder
// scan. One string used to serve all three cases and claimed "this container has
// no backup yet" on every one of them, including the folder scan offered after
// an index read failed — a container that demonstrably HAS a backup, which is
// the only reason its index was read at all.
// An advisory id from the server mapped to its sentence. Unknown ids answer
// null and render nothing: a newer server may know caveats this interface does
// not, and a raw "immich-db-separate" on screen would be worse than silence.
function advisoryKey(id: string): TranslationKey | null {
  if (id === "immich-db-separate") return "excludes.advisoryImmichDb";
  if (id === "nextcloud-db-separate") return "excludes.advisoryNextcloudDb";
  return null;
}

function liveSourceKey(reason: "no-snapshot" | "requested" | "not-in-snapshot"): TranslationKey {
  if (reason === "requested") return "excludes.assistSourceLiveRequested";
  if (reason === "not-in-snapshot") return "excludes.assistSourceLiveNotInSnapshot";
  return "excludes.assistSourceLive";
}

// ExcludesEditor edits this container's restic exclude patterns, one per line,
// and shows a debounced live preview of how each line resolves against the
// container's live mounts: a container path is translated to the anchored host
// path restic stored (shown muted), a bare name passes through, and a line that
// would exclude nothing is warned. Clones StopContainersEditor + a preview pane.
export type ExcludePreviewRow = { raw: string; resolved: string; status: string; matches: boolean };

// How a caller resolves candidate lines. Each domain has its own endpoint and
// its own answer shape, so the adapter rather than the raw call is the prop.
export type ExcludePreview = (name: string, lines: string[]) => Promise<ExcludePreviewRow[]>;

const containerExcludePreview: ExcludePreview = async (name, lines) => {
  const r = await previewContainerExcludes(name, lines);
  return r.ok ? r.preview : [];
};

// `open` is controlled by the caller — see HooksEditor's own comment.
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
  // Live-save conversion (jdp, live review — see HooksEditor's own header
  // comment for the full "why" across all four editors): the manual Save
  // button + its own `shakeSave` nonce are GONE — the textarea below now
  // debounce-auto-saves itself the same way HooksEditor's two inputs do (see
  // that component's own comment for why no shake/revert applies to free
  // text). `state`/`saveLines` stay exactly as they were: the assistant's
  // one-click suggestion chips (addExclude/removeExclude below) already
  // saved immediately, no button involved, before this conversion — that
  // half of this editor was ALREADY live-save and needed no change.
  const { debouncedSave, cancel: cancelPendingSave } = useDebouncedSave();

  // Debounced live preview: whenever the editor is open and the textarea holds at
  // least one non-blank line, resolve the candidate lines against the container's
  // mounts (~400ms after the last keystroke). Depends only on `text`/`open`, so
  // it re-previews on real edits — never in a loop (setPreview doesn't touch text).
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

  // The current exclude lines as the editor holds them (unsaved edits included) —
  // the single source both the save button and the assistant's one-click actions
  // work from, so they can never diverge.
  const currentLines = text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);

  // saveLines persists an explicit line list and mirrors it back into the
  // textarea, sharing the editor's save state machine. The debounced textarea
  // auto-save below passes the freshly-typed lines; the assistant passes the
  // list ± one line.
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

  // --- Exclusion assistant: server-side scan for junk/large folders with
  // one-click exclude.
  const [assistOpen, setAssistOpen] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [suggestions, setSuggestions] = useState<ExcludeSuggestion[] | null>(null); // null = not scanned yet
  const [truncated, setTruncated] = useState(false);
  // Where the sizes came from, and the promise that goes with them (#175): the
  // snapshot source is exact but AS OF snapshotTime, which is why the timestamp
  // is rendered next to the list rather than treated as optional polish; the
  // live source is as of now but can stop early, inside stoppedAt.
  const [source, setSource] = useState<"snapshot" | "live">("live");
  // Why the live walk ran. Three different facts, three different sentences —
  // see liveSourceKey.
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
  // The folders are configured but none is reachable (unmounted array/share) and
  // there was no backup to read instead. "Nothing left to exclude" would be the
  // loudest lie this panel can tell.
  const [pathsUnavailable, setPathsUnavailable] = useState(false);
  // Caveats about the container itself, which the folder scan cannot see. Held
  // apart from the suggestion list on purpose: they are true whether or not a
  // single exclusion is offered, and the most important one is true precisely
  // when the list is empty.
  const [advisories, setAdvisories] = useState<string[]>([]);
  const [preset, setPreset] = useState<ExcludePreset | null>(null);
  // The backup index could not be read. Not a failed scan: the panel stays up
  // and offers the folder scan as an explicit second request.
  const [indexFailed, setIndexFailed] = useState(false);
  // Distinguishes "scanned, found nothing" from "the scan itself failed" for the
  // "nothing found" hint below — the failure TEXT itself no longer lives here
  // (see scan()'s own comment), just the fact of it.
  const [scanFailed, setScanFailed] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Scan/Rescan button alongside its existing toast on a failed scan.
  const [shakeScan, setShakeScan] = useState(0);

  // GlimStone follow-up pass (v8.0.0): the scan-failed inline error is now a
  // toast — a Scan/Rescan click is a one-shot action like every other migrated
  // save/test button here. `truncated` (rendered below) stays inline: it is a
  // persistent fact ABOUT the current suggestion list ("this list was cut
  // short"), not a completion notice of the scan action itself.
  async function scan(live = false) {
    setScanning(true);
    setScanFailed(false);
    setIndexFailed(false);
    // Every "why is this list short" fact below describes ONE scan: the one that
    // produced the list currently on screen. A rescan that fails leaves no list,
    // so keeping the previous run's truncation banner up would describe a scan
    // that no longer exists ("the scan hit its time limit inside /config/Media"
    // sitting above an empty panel and a failure toast).
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
        // A scan the user asked for is known to be one HERE, whatever the server
        // says: the fallback must never be the "no backup yet" sentence on a
        // request that only exists because a backup was there to read.
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

  // Both chip paths save IMMEDIATELY, so they retire any pending textarea
  // debounce first — see cancel()'s own comment. currentLines is derived from
  // the textarea's live value, so the list sent here already carries whatever
  // was typed in that same window; the dropped timer would only have written an
  // older copy of it.
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
          // Debounced auto-save (800ms after the last keystroke) — see this
          // component's own top-level comment. Lines are parsed from
          // `nextText` right here, not re-read from `text` when the timer
          // fires, matching Settings.tsx's own "compute the next value
          // locally, pass it straight into the debounced closure" shape.
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
            // Show a plain, reassuring confirmation — NOT the raw internal
            // restic path (BombVault's rebased host-mount view, e.g.
            // /host/user/user/appdata/…), which looked like an invalid path
            // and confused users (#38). The exact pattern is still available
            // on hover (title) for the curious.
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

      {/* Exclusion assistant */}
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
              {/* Colour-engine integration (same fix/reasoning as
                  FoldersEditor's "Hinzufügen" button above): was the one
                  plain grey `bg-carbon-surface2` button in this
                  assistant sub-panel, next to its own "Ausschließen"
                  suggestion-accept button below which was ALREADY
                  `bg-accent` — matches that sibling now, same
                  already-correct .glim-hue-cascade mechanism. */}
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
              {/* The standing "what is on disk RIGHT NOW" question. A snapshot
                  cannot answer it: a junk folder created since the last backup
                  is not in the index, so it has no row and no warning at all,
                  and "what can I stop backing up" is exactly the question a
                  cache that exploded yesterday answers. Offered whenever the
                  list came from a backup, not only after an index failure.
                  NOT while indexFailed: that branch already offers the same
                  action under its own label, and two differently-worded
                  buttons for one thing read as two different things. */}
              {!scanning && suggestions !== null && !scanFailed && !indexFailed && source === "snapshot" && (
                <Button
                  label={t("excludes.assistScanCurrent")}
                  labelKey="excludes.assistScanCurrent"
                  tone="neutral"
                  onClick={() => void scan(true)}
                />
              )}
              {truncated && !scanning && stoppedAt && (
                // Stays inline and stays a separate line from the per-row size
                // flags below: a folder the walk never reached has no row at
                // all, so "the rest was not examined" is a claim no per-row flag
                // can make. It names WHERE the list ends (#175), and the folder
                // is pinned LTR so the leading `/` does not migrate to the far
                // end of the path in ar/he/fa.
                <span className="text-xs text-statusWarn">
                  {withLtrPlaceholder(t("excludes.assistTruncated"), "{path}", stoppedAt)}
                </span>
              )}
            </div>
            {/* Whole backup folders that produced no rows, for two different
                reasons. Both are claims no per-row flag can make, and with
                several roots their absence read as a finished scan of all of
                them. */}
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
            {/* "Nothing left to exclude" is a POSITIVE finding and may only be
                said when the scan actually looked. pathsUnavailable means it
                could not look at all. */}
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
                      // "cache" was bg-statusInfoBg/text-statusInfo (the
                      // old fifth hue). This is a categorisation label — "this
                      // looks like a cache dir" — not activity and not a
                      // pass/fail/warn outcome, so it folds into --status-neutral-*
                      // (already documented in index.css as "skipped/neutral
                      // chip", the same broad "not a real state" bucket this
                      // chip belongs in, sitting next to its "large" sibling
                      // which keeps its own real warn meaning unchanged).
                      className={`inline-flex items-center rounded-pill px-2 py-0.5 text-xs font-medium ${
                        sg.reason === "large" ? "bg-statusWarnBgStrong text-statusWarn" : "bg-statusNeutralBg text-statusNeutral"
                      }`}
                    >
                      {sg.reason === "large" ? t("excludes.assistReasonLarge") : t("excludes.assistReasonCache")}
                    </span>
                    {/* #175: a size the scan could not finish measuring is a
                        MINIMUM, and says so. Rendering it as a plain number is
                        what showed a 55 GB folder as "5.7 GB". */}
                    {sg.complete ? (
                      <span className="text-xs text-carbon-textSub whitespace-nowrap">{humanBytes(sg.sizeBytes)}</span>
                    ) : (
                      // Same tone as the exact figure on purpose. It used to be
                      // text-carbon-textMuted, which made the least legible text
                      // in the row the one carrying the caveat, in both colour
                      // modes. The explanation moved out of a native title= —
                      // invisible on touch — into the house InfoBubble.
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
            {/* Where the numbers come from. Required, not decoration: a
                snapshot size is exact but AS OF that backup, so a cache that
                has grown tenfold since reads at its old size — stating the date
                is the whole reason that is honest rather than misleading. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && (
              <p className="text-xs text-carbon-textMuted">
                {source === "snapshot"
                  ? t("excludes.assistSourceSnapshot").replace("{when}", formatSnapshotWhen(snapshotTime))
                  : t(liveSourceKey(liveReason))}
              </p>
            )}
            {/* The date above is necessary and not sufficient. A snapshot list
                cannot show a folder that did not exist when the backup ran, so
                once the backup is old enough to matter the panel says so rather
                than leaving the user to compare a timestamp. */}
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
