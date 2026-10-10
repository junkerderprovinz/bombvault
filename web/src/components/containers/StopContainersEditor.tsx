import { useEffect, useRef, useState } from "react";
import { setStopContainers } from "../../lib/api";
import type { Container } from "../../lib/api";
import { DropdownListbox } from "../DropdownListbox";
import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// StopContainersEditor edits the list of OTHER containers to stop during this
// container's backup (e.g. a database). Collapsible; `open` is controlled by
// the caller — see HooksEditor's own comment.
//
// REWORKED (jdp, live review: "Können wir da nicht eine Dropdownliste aller
// installierten Container machen? Also dass es automatisch alle installierten
// Container auflistet, die man dann auswählen kann."): this used to be a
// free-text `<textarea>`, one hand-typed container name per line — no
// validation against what is actually installed, a typo just silently never
// matched anything at backup time. Replaced with a proper multi-select: a
// dropdown/listbox populated from `installedContainers`, the SAME container
// list ContainerRow's own caller (Containers()) already fetches to render
// every row on this page — no second API call. Custom listbox, not a native
// `<select multiple>` (illegible checkbox-free multi-select UI, no per-row
// icon/status room) — same "escape hatch" precedent as Settings.tsx's
// LanguageCard dropdown (role="listbox", outside-click/Escape-to-close), here
// extended to `aria-multiselectable="true"` with a real checkbox per row
// instead of LanguageCard's single-select radio-like rows.
export function StopContainersEditor({
  name,
  initial,
  installedContainers,
  open,
  t,
}: {
  name: string;
  initial: string[];
  /** Every INSTALLED container on this BombVault instance, as already
   *  fetched once by Containers() for rendering the row list — threaded
   *  through ContainerRow rather than a second `listContainers()` call here. */
  installedContainers: Container[];
  open: boolean;
  t: T;
}) {
  const [selected, setSelected] = useState<Set<string>>(() => new Set(initial));
  const [pickerOpen, setPickerOpen] = useState(false);
  // The BUTTON itself, not its wrapper: DropdownListbox sizes the portalled
  // panel to whatever this ref measures, and the wrapper below is a flex
  // ITEM of this editor's `flex flex-col` box — `inline-block` gets
  // blockified and stretched to the card's full content width there, so
  // measuring the wrapper handed the panel a ~970px width instead of the
  // button's own 256px (caught by measuring it live, not by reading the
  // markup). It also keeps the outside-click exemption tight: only the
  // button is exempt, which is all that needs to be.
  const pickerRef = useRef<HTMLButtonElement>(null);
  const { push } = useToast();
  // Live-save conversion (jdp, live review — see HooksEditor's own header
  // comment for the full "why" across all four editors): each listbox row is
  // a discrete boolean pick (same shape as FoldersEditor's mount checkboxes),
  // so rowBusy/rowShake below are that identical per-key busy/shake map, keyed
  // by candidate container name instead of mount source.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});

  // Re-seed whenever the SAVED value changes underneath this editor (e.g. a
  // fresh `listContainers()` reload after Discover) — the same "derived from
  // props but independently editable until the next save" shape
  // UpdateAfterBackupRow's own `initial`-seeded toggle already uses.
  //
  // Keyed on the CONTENT, not the array's identity. The call site passes
  // `container.stopContainers ?? []`, and the API returns null for any
  // container that has no target row yet, so that fallback minted a BRAND NEW
  // array on every parent render — and this effect then reset the selection to
  // empty each time. What made it costly rather than merely annoying: the next
  // toggle saves the visible set, and the server replaces the stored list
  // wholesale, so a user who ticked three containers and then triggered any
  // parent re-render silently saved a list with only the fourth in it.
  const initialKey = JSON.stringify(initial);
  useEffect(() => {
    setSelected(new Set(JSON.parse(initialKey) as string[]));
  }, [initialKey]);

  // Candidates: every OTHER installed container — excludes this row's own
  // container (a container can't stop itself) and BombVault's own container
  // (the established "BombVault's own container never appears in
  // schedule-member lists" rule, Settings.tsx's ContainersSection).
  const candidates = installedContainers
    .filter((c) => c.name !== name && !c.self)
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));

  // A previously-saved name that no longer matches an installed container
  // (uninstalled since, renamed, or a leftover from the old free-text field)
  // still needs to stay visible and removable — silently dropping it on the
  // next save would be data loss the user never asked for. Marked inline
  // with the existing `containers.notInstalled` badge text rather than a
  // second bespoke "stale" label.
  const candidateNames = new Set(candidates.map((c) => c.name));
  const installedNames = new Set(installedContainers.map((c) => c.name));

  // Discrete boolean toggle — optimistic flip, immediate save, revert +
  // `.glim-shake` (keyed by container name) on failure. Same shape as
  // SettingsPage's toggleDomainEnabled/FoldersEditor's own mount-checkbox
  // `toggle` — see this component's own top-level comment.
  async function toggle(n: string) {
    const wasSelected = selected.has(n);
    const next = new Set(selected);
    if (wasSelected) next.delete(n);
    else next.add(n);
    setSelected(next);
    setRowBusy((b) => ({ ...b, [n]: true }));
    try {
      const r = await setStopContainers(name, [...next]);
      if (r.ok) {
        push(t("settings.saved"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
        setSelected((prev) => {
          const reverted = new Set(prev);
          if (wasSelected) reverted.add(n);
          else reverted.delete(n);
          return reverted;
        });
        setRowShake((s) => ({ ...s, [n]: (s[n] ?? 0) + 1 }));
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setSelected((prev) => {
        const reverted = new Set(prev);
        if (wasSelected) reverted.add(n);
        else reverted.delete(n);
        return reverted;
      });
      setRowShake((s) => ({ ...s, [n]: (s[n] ?? 0) + 1 }));
    } finally {
      setRowBusy((b) => ({ ...b, [n]: false }));
    }
  }

  // Outside-click / Escape / scroll dismissal is deliberately NOT wired up
  // here any more: it moved into DropdownListbox along with the panel itself
  // (see that component's header for the clipping bug that forced the panel
  // out of this card and into a portal). A second copy left behind here would
  // have been actively wrong — the old handler asked "is the mousedown inside
  // `pickerRef`", which a portalled option button no longer is, so it would
  // have unmounted the list on mousedown and the option's own click would
  // never have landed.

  if (!open) return null;

  const sortedSelected = [...selected].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("stophook.hint")}</p>
      {/* Picker trigger deliberately stays plain `bg-carbon-surface2` (not
          rainbow-hued): every other VALUE picker in this app (this same
          file's own offsite-target `<select>`, the Language/Theme card
          dropdowns, FolderBrowser's text field) is plain neutral chrome —
          rainbow hue in this app marks a genuine ACTION control (FoldersEditor's
          own "Hinzufügen" icon badge is that pattern's live example), never a
          value-holding input/picker. This editor's own former Save badge is
          gone entirely (live-save conversion, see this component's own
          top-level comment) — every row below now persists itself. */}
      <div className="inline-block">
        <button
          ref={pickerRef}
          type="button"
          aria-haspopup="listbox"
          aria-expanded={pickerOpen}
          onClick={() => setPickerOpen((v) => !v)}
          className="flex items-center gap-2 w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-surface3 transition-colors text-start"
        >
          <span className="min-w-0 flex-1 truncate">{t("stophook.title")}</span>
          <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className={`shrink-0 transition-transform ${pickerOpen ? "rotate-90" : "rtl:rotate-180"}`}>
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </button>
        {/* Portalled, not `absolute` inside this card: ContainerRow's own
            wrapper is `relative overflow-hidden` (ProgressBar needs that
            clip), which hard-clipped this panel at the card's bottom edge no
            matter what z-index it carried — jdp, live review: "Sie soll über
            die Card hinausgehen und voll angezeigt werden." See
            DropdownListbox.tsx for the full root cause. */}
        <DropdownListbox
          open={pickerOpen}
          onClose={() => setPickerOpen(false)}
          triggerRef={pickerRef}
          label={t("stophook.title")}
          multiselectable
        >
          <>
            {candidates.length === 0 && sortedSelected.length === 0 && (
              <p className="px-3 py-2 text-xs text-carbon-textMuted">{t("stophook.noCandidates")}</p>
            )}
            {candidates.map((c) => {
              const checked = selected.has(c.name);
              return (
                <button
                  // Keyed by name PLUS its own shake nonce — see
                  // FoldersEditor's identical mount-row key comment.
                  key={`${c.name}-${rowShake[c.name] ?? 0}`}
                  type="button"
                  role="option"
                  aria-selected={checked}
                  onClick={() => void toggle(c.name)}
                  disabled={!!rowBusy[c.name]}
                  className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-start transition-colors disabled:opacity-60 ${
                    checked ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                  }${rowShake[c.name] ? " glim-shake" : ""}`}
                >
                  <input type="checkbox" checked={checked} readOnly tabIndex={-1} className="pointer-events-none" style={{ accentColor: "var(--accent)" }} />
                  <span className="min-w-0 flex-1 truncate">{c.name}</span>
                </button>
              );
            })}
            {/* Stale entries: a previously-saved name no longer among the
                installed candidates above — still listed (so it stays
                removable) but marked with the existing notInstalled label. */}
            {sortedSelected.filter((n) => !candidateNames.has(n)).map((n) => (
              <button
                key={`${n}-${rowShake[n] ?? 0}`}
                type="button"
                role="option"
                aria-selected
                onClick={() => void toggle(n)}
                disabled={!!rowBusy[n]}
                className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-start text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-60${rowShake[n] ? " glim-shake" : ""}`}
              >
                <input type="checkbox" checked readOnly tabIndex={-1} className="pointer-events-none" style={{ accentColor: "var(--accent)" }} />
                <span dir="ltr" className="min-w-0 flex-1 truncate font-mono text-start">{n}</span>
                <span className="shrink-0 text-caption text-statusFail">{t("containers.notInstalled")}</span>
              </button>
            ))}
          </>
        </DropdownListbox>
      </div>
      {sortedSelected.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {sortedSelected.map((n) => (
            <span
              key={`${n}-${rowShake[n] ?? 0}`}
              className={`inline-flex items-center gap-1.5 rounded-pill bg-carbon-surface2 px-2 py-0.5 text-xs text-carbon-textSub${rowShake[n] ? " glim-shake" : ""}`}
            >
              {n}
              {/* A backup or an import passes over a name no container has, so
                  the chip says so without opening the picker. */}
              {!installedNames.has(n) && (
                <span className="text-caption text-statusFail">{t("containers.notInstalled")}</span>
              )}
              <Button
                label={t("stophook.remove").replace("{name}", n)}
                labelKey="stophook.remove"
                variant="chip"
                onClick={() => void toggle(n)}
                disabled={!!rowBusy[n]}
              />
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
