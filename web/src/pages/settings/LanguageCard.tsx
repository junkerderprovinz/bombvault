// LanguageCard, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import { Card } from "../settings/shared";
import { DropdownListbox } from "../../components/DropdownListbox";
import { Flag } from "../../components/Sidebar";
import { useRef, useState } from "react";
import { useT } from "../../lib/i18n";

// ---------------------------------------------------------------------------
// Language Card (GlimStone follow-up pass, live-review point 9) — the app's
// UI-language switcher, MOVED here out of Sidebar.tsx's own footer, not
// duplicated (jdp: "verschieb den Sprachschalter... auch als eigene card ins
// allgemein setting"). Same picker mechanism as before: useT()'s
// lang/setLanguage/languages (lib/i18n.ts — a flat 42-locale list, persisted
// to localStorage's "bv-lang" key, applied to <html lang>/[dir] immediately,
// no Save step) and Sidebar.tsx's own exported `Flag` glyph for each entry.
// Only the TRIGGER's styling changed, from the sidebar's nav-rail look
// (navBase/navInactive, which key off `--sidebar-text`/`--sidebar-hover` and
// mean nothing outside the rail) to a plain bg-carbon-surface2 button — the
// same idle-chip fill every other inline picker trigger in this file already
// uses (e.g. VMSSHCard's copy buttons above). The dropdown listbox itself
// (role="listbox", flag+label options, outside-click/Escape-to-close) is
// reused verbatim; only the open direction flipped from `bottom-full` (the
// sidebar footer sits at the viewport's bottom edge) to `top-full` (this
// Card sits in normal page flow, so it opens downward like any other
// dropdown on this page).
export function LanguageCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { lang, setLanguage, languages } = useT();
  const [open, setOpen] = useState(false);
  // The BUTTON itself, not its wrapper — DropdownListbox sizes the portalled
  // panel to what this ref measures, and an `inline-block` wrapper inside a
  // flex column is blockified and stretched to the Card's full width. See
  // StopContainersEditor's own copy of this note in Containers.tsx.
  const ref = useRef<HTMLButtonElement>(null);

  const current = languages.find((l) => l.code === lang) ?? languages[0];

  // Outside-click / Escape / scroll dismissal lives in DropdownListbox now,
  // together with the panel it dismisses — this card's own two hand-rolled
  // listener effects are gone. They were also the pattern Containers.tsx's
  // multi-select copied wholesale, and that copy sat under an
  // `overflow-hidden` card that hard-clipped the panel (see
  // DropdownListbox.tsx's header). Fixing the copy and leaving the original
  // behind as a second, subtly different implementation of the same control
  // is exactly the sibling drift this repo keeps out; both call sites now
  // render the one shared, portalled panel.
  //   Not merely cosmetic here either: this list is 42 locales deep and
  // always opened straight downward with no viewport awareness at all, so on
  // a short window it ran off the bottom edge. The shared panel clamps and
  // flips above the trigger when there is no room below.

  return (
    <Card title={t("settings.language")} hueIndex={hueIndex}>
      <div className="inline-block">
        {/* w-48 (GlimStone follow-up pass, live-review round — "widen the
            Language button, then match the Theme button to it"): was
            content-hugging (only as wide as the current flag+label pair),
            which read as too narrow/incidental for a deliberate settings
            control. w-48 (192px) isn't an arbitrary new number — it was the
            SAME width this button's own dropdown listbox already hard-coded,
            so the trigger sits flush above the exact footprint of the menu it
            opens, rather than a narrower button popping open a visibly wider
            list. That number is no longer restated on the listbox at all:
            DropdownListbox sizes the portalled panel to THIS trigger's own
            measured width, so the two can no longer drift apart.
            `truncate`/`min-w-0` on
            the label span below keeps a genuinely long locale name (this
            list has 42) from overflowing the now-fixed width instead of
            just growing the button the way it used to. */}
        <button
          ref={ref}
          type="button"
          aria-label={`${t("language.label")}: ${current.label}`}
          title={`${t("language.label")}: ${current.label}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
          className="flex items-center gap-2.5 w-48 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
        >
          <Flag code={current.flag} />
          <span className="min-w-0 truncate text-start">{current.label}</span>
        </button>
        <DropdownListbox
          open={open}
          onClose={() => setOpen(false)}
          triggerRef={ref}
          label={t("language.label")}
        >
          {languages.map((l) => (
            <button
              key={l.code}
              type="button"
              role="option"
              aria-selected={l.code === lang}
              onClick={() => { setLanguage(l.code); setOpen(false); }}
              className={`flex items-center gap-2.5 w-full px-3 py-2 text-sm text-start transition-colors ${
                l.code === lang
                  ? "bg-carbon-surface3 text-carbon-text"
                  : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              }`}
            >
              <Flag code={l.flag} />
              <span>{l.label}</span>
            </button>
          ))}
        </DropdownListbox>
      </div>
    </Card>
  );
}
