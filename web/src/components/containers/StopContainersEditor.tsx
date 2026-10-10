import { useEffect, useRef, useState } from "react";
import { setStopContainers } from "../../lib/api";
import type { Container } from "../../lib/api";
import { DropdownListbox } from "../DropdownListbox";
import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// StopContainersEditor picks the other containers to stop during this
// container's backup, a database for instance. The caller controls `open`.
// The choices come from the installed containers, so a name cannot be typed
// in a way that never matches at backup time.
export function StopContainersEditor({
  name,
  initial,
  installedContainers,
  open,
  t,
}: {
  name: string;
  initial: string[];
  /** Every installed container, as the page already fetched it for its rows. */
  installedContainers: Container[];
  open: boolean;
  t: T;
}) {
  const [selected, setSelected] = useState<Set<string>>(() => new Set(initial));
  const [pickerOpen, setPickerOpen] = useState(false);
  // On the button, not its wrapper: DropdownListbox sizes the portalled panel
  // to what this ref measures, and the wrapper is a flex item that stretches
  // to the card's full width.
  const pickerRef = useRef<HTMLButtonElement>(null);
  const { push } = useToast();
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});

  // Re-seed when the saved value changes underneath this editor, after a
  // reload for instance. Keyed on the content, not the array's identity: the
  // call site passes `container.stopContainers ?? []`, a new array on every
  // parent render, and re-seeding from it would drop the picks made since the
  // last reload. The next toggle would then store the shortened list.
  const initialKey = JSON.stringify(initial);
  useEffect(() => {
    setSelected(new Set(JSON.parse(initialKey) as string[]));
  }, [initialKey]);

  // A container cannot stop itself, and BombVault's own container is never
  // offered.
  const candidates = installedContainers
    .filter((c) => c.name !== name && !c.self)
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));

  // A saved name that matches no installed container stays visible and
  // removable, marked as not installed. Dropping it on the next save would
  // lose data the user never asked to lose.
  const candidateNames = new Set(candidates.map((c) => c.name));
  const installedNames = new Set(installedContainers.map((c) => c.name));

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

  if (!open) return null;

  const sortedSelected = [...selected].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("stophook.hint")}</p>
      {/* Neutral chrome: the hue marks controls that act, and this one holds
          a value. */}
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
        {/* Portalled: the container card clips its overflow for the progress
            bar, which would cut a panel positioned inside it off at the
            card's bottom edge. */}
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
                  // The shake count in the key remounts the row, which
                  // replays the animation.
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
            {/* Saved names that are not among the candidates, listed so they
                stay removable. */}
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
