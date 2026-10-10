import { useEffect, useRef, useState, type CSSProperties } from "react";
import { getBackupOrder, setBackupOrder } from "../../lib/api";
import type { Container, ContainerOrder } from "../../lib/api";
import { IconTipButton } from "../IconTipButton";
import { useT } from "../../lib/i18n";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { useReorder } from "../../lib/dragLift";
import { hueVars } from "../../lib/appearance";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// Backup-order panel (#119) — manual per-container backup sequence

// Per-browser: whether the backup-order card is collapsed (#124 — ptmorris1 has
// many containers). Same "bombvault.*" localStorage convention as the other UI prefs.
const BACKUP_ORDER_COLLAPSED_KEY = "bombvault.backupOrderCollapsed";

// BackupOrderPanel lets the user arrange the order scheduled + batch backups run
// in. The orderable set is the installed, schedule-included containers (never
// BombVault itself). It hydrates once from the persisted order (GET
// /api/containers/backup-order), then reconciles as containers come and go
// without discarding an in-progress reorder. Save PUTs the whole displayed
// sequence (authoritative: the list becomes the explicit order); Clear order
// PUTs an empty list, returning every container to the most-overdue-first
// tiebreak.
export function BackupOrderPanel({
  containers,
  t,
  hueIndex,
}: {
  containers: Container[];
  t: T;
  /** Rainbow position for THIS panel's own heading notch — GlimStone
   *  follow-up pass (jdp, live review, emphatic, fifth escalation of the
   *  standing colour-engine rule: "Warum muss ich dich immer wieder extra
   *  dran erinnern? Kannst du das jetzt nicht einfach selbst immer
   *  machen?"): this panel's collapsible-header title was still a plain
   *  `<span>`, never routed through Badge's tone="heading"/hueIndex the way
   *  every other static Card heading in the app now is (Dashboard.tsx's
   *  Card(), Config.tsx's Card, Settings.tsx's Card/ToggleRow, VMs.tsx's own
   *  VMBackupOrderPanel — its exact twin, already fixed a commit ago).
   *  Resolved by the caller's own `nextHue()` counter, called DIRECTLY at
   *  the JSX call site (never handed down as a function for this component
   *  to call from its own body — that exact shape is what caused the
   *  SummaryTier regression earlier this session: React doesn't invoke a
   *  child component's body until after the parent's own render pass has
   *  already returned, so a `nextHue` prop called from inside a child lands
   *  strictly after every sibling's own direct call already consumed its
   *  slot). Omit for a genuine singleton — same rule as every other
   *  `hueIndex` call site. */
  hueIndex?: number;
}) {
  const [savedOrder, setSavedOrder] = useState<ContainerOrder[] | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // whichever button triggered the failed persist() — Save or Reset — kept as
  // two separate nonces, mirroring VMs.tsx's identical VMBackupOrderPanel.
  const [shakeSave, setShakeSave] = useState(0);
  const [shakeReset, setShakeReset] = useState(0);
  const hydrated = useRef(false);
  // #124: collapse the whole card, persisted per browser.
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(BACKUP_ORDER_COLLAPSED_KEY) === "1";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    getBackupOrder()
      .then((res) => setSavedOrder(res.ok ? res.order ?? [] : []))
      .catch(() => setSavedOrder([]));
  }, []);

  useEffect(() => {
    if (savedOrder === null) return; // still loading the persisted order
    const orderable = containers
      .filter((c) => c.installed && c.includeInSchedule && !c.self)
      .map((c) => c.name);
    const set = new Set(orderable);
    const byName = (a: string, b: string) =>
      a.localeCompare(b, undefined, { sensitivity: "base" });
    if (!hydrated.current) {
      hydrated.current = true;
      const ranked = savedOrder
        .filter((o) => set.has(o.container))
        .sort((a, b) => a.order - b.order)
        .map((o) => o.container);
      const rest = orderable.filter((n) => !ranked.includes(n)).sort(byName);
      setNames([...ranked, ...rest]);
      return;
    }
    setNames((prev) => {
      const kept = prev.filter((n) => set.has(n));
      const added = orderable.filter((n) => !kept.includes(n)).sort(byName);
      const next = [...kept, ...added];
      return next.length === prev.length && next.every((n, i) => n === prev[i])
        ? prev
        : next;
    });
  }, [containers, savedOrder]);

  function move(index: number, dir: -1 | 1) {
    setNames((prev) => {
      const to = index + dir;
      if (to < 0 || to >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[to]] = [next[to], next[index]];
      return next;
    });
    setSaveState("idle");
  }

  // A row is carried by its grip and lands in its gap; the arrows are the
  // keyboard's way to do the same.
  const list = useRef<HTMLOListElement>(null);
  const drag = useReorder({
    ids: names,
    container: list,
    attr: "data-order-name",
    axis: "y",
    arm: "move",
    enabled: saveState !== "saving",
    onReorder: (next) => {
      setNames(next);
      setSaveState("idle");
    },
  });

  function toggleCollapsed() {
    setCollapsed((v) => {
      const next = !v;
      try {
        localStorage.setItem(BACKUP_ORDER_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        /* private mode / quota — collapse just won't persist */
      }
      return next;
    });
  }

  async function persist(order: string[], via: "save" | "reset") {
    setSaveState("saving");
    const bumpShake = via === "save" ? setShakeSave : setShakeReset;
    try {
      const res = await setBackupOrder(order);
      if (res.ok) {
        setSavedOrder(order.map((container, i) => ({ container, order: i + 1 })));
        push(t("backupOrder.saved"), "success");
      } else {
        push(res.error ?? t("backupOrder.saveError"), "fail");
        bumpShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("backupOrder.saveError"), "fail");
      bumpShake((n) => n + 1);
    } finally {
      setSaveState("idle");
    }
  }

  function clearOrder() {
    const sorted = [...names].sort((a, b) =>
      a.localeCompare(b, undefined, { sensitivity: "base" })
    );
    setNames(sorted);
    void persist([], "reset");
  }

  if (savedOrder === null) return null;

  return (
    // Rainbow-mode completeness sweep (jdp, live review, sixth escalation of
    // this same standing rule on this exact panel: "Es sind nicht alle
    // Buttons in den Regenbogen-Modus eingepflegt"): `.glim-hue` added below
    // — `glim-notch-card` alone only wires the reactive-mode hover reveal on
    // the heading Badge's own notch, it never redefines --accent/
    // --focus-ring, so the "Save" button further down stayed the flat theme
    // accent regardless of rainbow even after the title notch itself was
    // fixed. Same hueIndex prop the Badge already uses.
    //
    // relative + glim-notch-card: same "half-overlap card notch" pattern
    // every other real Card in this app uses (VMs.tsx's own
    // VMBackupOrderPanel is the closest twin — a single div carrying both
    // the visible surface AND the notch's positioned ancestor, no separate
    // outer wrapper needed since this box has no overflow-hidden to clip the
    // badge's own -11px poke above it). glim-notch-card is the hook
    // index.css's card-wide reactive-hover rule keys off, so hovering
    // anywhere on this panel (not just the tiny badge glyph) reveals its hue
    // in reactive rainbow mode.
    //   mt-4 (jdp: "Backup-Reihenfolge Card ist zu weit
    // oben, der Abstand nach oben ist zu klein"): the page's own outer
    // `flex flex-col gap-6` already puts a flat, uniform 24px between every
    // top-level section — measured live, byte-identical both above AND below
    // this panel (the controls row's own bottom edge to this div's own CSS
    // box top, and this div's bottom to the next card's top, both exactly
    // 24px). What actually reads as "too little" is this panel's OWN notch
    // badge poking `-translate-y-1/2` ABOVE that box — measured live at 11px
    // — which eats into the gap from the TOP side only (nothing pokes
    // downward below the panel, so its own bottom gap is unaffected): the
    // real visible whitespace between the controls row and the first
    // painted pixel of this card (the badge) measured only 13px, barely
    // half the page's other gaps. `mt-4` adds 16px on top of the existing
    // 24px flex gap (margin and `gap` are independent and stack, they don't
    // collapse into each other), landing the visible gap at ~29px —
    // deliberately a bit MORE than the page's plain 24px rhythm, not just
    // parity with it, matching jdp's own framing ("increase", not merely
    // "restore").
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-4 mt-4 flex flex-col gap-3${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      {/* Title notch, always visible regardless of collapse state (matches
          the PRE-fix behaviour, where title+count stayed visible collapsed
          and only the hint hid) — moved OUT of the disclosure <button>
          below: every real tone="heading" call site in this app keeps the
          Badge as its <h2>'s SOLE child (Dashboard.tsx's Card()/SummaryCell,
          Config.tsx's Card, this file's own notInstalledTitle below,
          StacksPanel above) because size="heading" makes the badge
          `position: absolute` — a flex-row sibling next to it would render
          at the badge's own now-vacated in-flow slot instead of after it.
          The count folds INSIDE the badge's own children instead (Badge's
          span is `inline-flex gap-1`, built to hold more than one child),
          same visual "title (N)" pairing as before, just now inheriting the
          badge's own solid accent-fill/accentContrast ink. */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("backupOrder.title")}
          {names.length > 0 && (
            <span className="ms-1.5 font-normal normal-case tracking-normal tabular-nums opacity-80">
              ({names.length})
            </span>
          )}
        </Badge>
      </h2>
      {/* Disclosure toggle, now chevron(+hint)-only: the title text that used
          to double as this button's accessible name moved into the h2 notch
          above, so `aria-label` keeps this control genuinely named rather
          than falling back to nothing once its only other content
          (`aria-hidden` chevron, hint text hidden while collapsed) has none
          to offer. `w-full` (unchanged) keeps the full row clickable even
          though the visible content is now just the chevron while collapsed. */}
      <button
        type="button"
        onClick={toggleCollapsed}
        aria-expanded={!collapsed}
        aria-label={t("backupOrder.title")}
        className="flex w-full items-start gap-2 text-start"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
          className={`mt-0.5 shrink-0 text-carbon-textSub transition-transform ${collapsed ? "rtl:rotate-180" : "rotate-90"}`}
        >
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
        {!collapsed && (
          <span className="min-w-0 flex-1 text-xs text-carbon-textMuted">{t("backupOrder.hint")}</span>
        )}
      </button>
      {!collapsed &&
        (names.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("backupOrder.empty")}</p>
        ) : (
          <>
            {/* The list is the rows' offsetParent, the layout a drag measures
                in. While a row is carried the others wiggle. */}
            <ol ref={list} className={`relative flex flex-col gap-1 ${drag.held !== null ? "glim-drag-armed" : ""}`}>
              {drag.order.map((name, i) => (
                <li
                  key={name}
                  data-order-name={name}
                  className={`flex select-none items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-1.5 ${drag.look(name)}`}
                >
                  {/* The grip is for a pointer; the keyboard uses the arrows. */}
                  <span
                    className="shrink-0 cursor-grab touch-none text-carbon-textSub active:cursor-grabbing"
                    aria-hidden="true"
                    onPointerDown={(e) => drag.press(e, name)}
                  >
                    <svg width="10" height="14" viewBox="0 0 10 14" fill="currentColor">
                      <circle cx="3" cy="3" r="1" />
                      <circle cx="7" cy="3" r="1" />
                      <circle cx="3" cy="7" r="1" />
                      <circle cx="7" cy="7" r="1" />
                      <circle cx="3" cy="11" r="1" />
                      <circle cx="7" cy="11" r="1" />
                    </svg>
                  </span>
                  <span className="w-6 text-xs text-carbon-textMuted tabular-nums">
                    {i + 1}.
                  </span>
                  <span className="flex-1 min-w-0 truncate text-sm text-carbon-text">
                    {name}
                  </span>
                  {/* IconTipButton, not plain <button> + `title` (whole-app
                      sweep — VMs.tsx's identical reorder pair converted in
                      the same pass). Both carried an `aria-label` plus a
                      duplicate native `title`, i.e. the OS balloon
                      IconTipButton.tsx exists to replace. Same tips, same
                      handlers, same disabled chrome. */}
                  <IconTipButton
                    tip={t("backupOrder.moveUp")}
                    onClick={() => move(i, -1)}
                    disabled={i === 0 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-pill p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 8.7 6 3.3 10.7 8.7Z" />
                    </svg>
                  </IconTipButton>
                  <IconTipButton
                    tip={t("backupOrder.moveDown")}
                    onClick={() => move(i, 1)}
                    disabled={i === names.length - 1 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-pill p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 3.3 6 8.7 10.7 3.3Z" />
                    </svg>
                  </IconTipButton>
                </li>
              ))}
            </ol>
            <div className="flex items-center gap-3 flex-wrap">
              <Button
                key={shakeReset}
        label={t("backupOrder.reset")}
          labelKey="backupOrder.reset"
                tone="subtle"
                onClick={clearOrder}
                disabled={saveState === "saving"}
                className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50${
                  shakeReset ? " glim-shake" : ""
                }`}
              />
              <Button
                key={shakeSave}
                label={t("backupOrder.save")}
                labelKey="backupOrder.save"
                tone="accent"
                onClick={() => void persist(names, "save")}
                disabled={saveState === "saving"}
                busy={saveState === "saving"}
                className={shakeSave ? "glim-shake" : ""}
              />
            </div>
          </>
        ))}
    </div>
  );
}
