// AccentCard, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import { AccentPresetSwatch } from "../settings/shared";
import { Badge } from "../../components/Badge";
import { getAccent, setAccent, DEFAULT_ACCENT, getAccentPresets, setAccentPresets, DEFAULT_ACCENT_PRESETS } from "../../lib/accent";
import { useState } from "react";
import { useT } from "../../lib/i18n";

export function AccentCard({
  t,
}: {
  t: ReturnType<typeof useT>["t"];
}) {
  const [accentHex, setAccentHex] = useState<string>(() => getAccent());
  const [presets, setPresets] = useState<string[]>(() => getAccentPresets());

  function selectAccent(hex: string) {
    setAccentHex(hex);
    setAccent(hex);
  }

  function changePreset(index: number, hex: string) {
    setPresets((prev) => {
      const next = prev.slice();
      next[index] = hex;
      return setAccentPresets(next);
    });
  }

  // ONE reset, TWO things reset (see this component's own header comment for
  // the "two competing resets" root cause). Both halves of the row's state
  // get their own predicate so the single control can be enabled whenever
  // EITHER has drifted, and the click can restore both unconditionally.
  const accentIsDefault = accentHex.toLowerCase() === DEFAULT_ACCENT.toLowerCase();
  const presetsAreDefault = presets.every(
    (hex, i) => hex.toLowerCase() === DEFAULT_ACCENT_PRESETS[i]?.toLowerCase()
  );
  const nothingToReset = accentIsDefault && presetsAreDefault;

  return (
    <div className="flex items-center gap-3 flex-wrap">
      {/* Row label — bare now, no trailing colon (jdp, live-review: "Der
          Doppelpunkt nach Akzentfarbe und Farbpalette weg"). The colon was
          only ever appended here in JSX, never baked into the
          settings.accentColor string itself (that string is ALSO read bare,
          with no colon, as the swatch's own aria-label/title below — see
          this key's history a few rounds back) — checked across all 42
          locales, none bakes a colon into the translated text either, so
          dropping the literal ":" here is the complete, single-source fix. */}
      <span className="text-sm text-carbon-text">{t("settings.accentColor")}</span>
      {/* EVERYTHING else on this row — the presets and their reset — is one
          right-aligned group (jdp, live-review: "Das Akzentfarbeauswahlfeld
          auch nach rechts"). Same `ms-auto` idiom this app uses everywhere
          else for "push this to the row's own far right"
          (Containers.tsx/Fleet.tsx's trailing metadata, and the
          rainbow-reset-badge row a few hundred lines below).
            The group used to hold a standalone custom-colour swatch and a
          "Voreinstellungen:" caption as well; both are gone ([432]) and the
          reasoning is at the next comment down. */}
      <div className="flex items-center gap-2 flex-wrap ms-auto">
        {/* The standalone custom-colour swatch and the "Voreinstellungen:"
            caption are GONE ([432], jdp: "das Farbpickerfeld und der text
            Voreinstellungen soll weg").
              Both were redundant rather than wrong. Every one of the eight
            swatches below already OPENS the same picker — that is what
            AccentPresetSwatch's wrapper does — so a ninth swatch whose only
            job was "open the picker" offered nothing the row did not already
            offer, while looking like a colour you could select. Two controls
            that do the same thing, one of which also lies about what it is.
              The caption went with it for the ordinary reason: a row of
            coloured discs in a card titled Akzentfarbe does not need a word
            telling you it is a row of colours. Naming the obvious is the kind
            of text that survives because nobody re-reads it, not because
            anybody needed it.
              Nothing is lost: a custom colour is still reachable, by opening
            any preset and changing it, which is also the only way it ever
            persisted. */}
        {presets.map((hex, i) => (
          <AccentPresetSwatch
            key={i}
            hex={hex}
            index={i}
            active={accentHex.toLowerCase() === hex.toLowerCase()}
            onSelect={selectAccent}
            onChangePreset={(v) => changePreset(i, v)}
            t={t}
          />
        ))}
        {/* THE reset for this whole row — ONE control that restores BOTH the
            active accent (accentHex → DEFAULT_ACCENT) and the eight preset
            swatches (presets → DEFAULT_ACCENT_PRESETS). Previously these
            were two separate, competing controls sitting side by side; see
            this component's own header comment for the full root cause and
            why merging them is the fix rather than repairing either one.
              `disabled={nothingToReset}` — enabled the moment EITHER half has
            drifted, which is the normal case for anyone who has picked any
            non-default accent at all (one click on any preset but the first
            already gets you there). The old `disabled={presetsAreDefault}`
            tracked only the RARER half — editing a preset's own colour via
            the popover — so in every ordinary session this badge sat
            permanently greyed out: present, correctly shaped, and completely
            inert (jdp: "Der Resetbutton ist da, hat aber keine Funktion").
            An always-disabled control is not a fixed control.
              SQUARE (jdp, live-review: "Die Zurücksetzen-Option soll ein
            quadratischer Badge mit Glyph sein"). `shape="square"` still
            resolves through `rounded-control` (the shape engine's own live
            token — see Badge.tsx's BadgeShape doc), so this genuinely reads
            round/soft/square per the user's own shape-engine choice, not a
            hardcoded corner — under the "Rund" shape-engine setting this
            renders as a full circle, same as every other square badge in
            the app, which is expected, not a regression.
              NEUTRAL now, not hue-tinted (jdp, re-reporting: "Der
            Reset-Badge soll ... nicht farbig sein, damit er sich besser von
            den Farbflächen abhebt" — it must NOT blend in as one more colour
            option in a row whose entire job is presenting colour options).
            DELIBERATE EXCEPTION to this app's standing "every icon-only
            badge gets real colour-engine integration" rule: a reset control
            sitting directly beside the very colour swatches it resets is
            the one role where taking a rainbow/accent hue is actively
            counter-productive — a coloured reset button reads as just
            another swatch to click through, not as a distinct utility
            action set apart from the row it acts on. `tone="neutral"`
            (Badge's own default, spelled out explicitly here so this
            exception is visible at the call site, not an accident of
            omission) resolves to `bg-carbon-surface2 text-carbon-textSub` —
            the same flat, hue-immune grey chip every other "no real status"
            badge in the app already uses. `border-2 border-carbon-border`
            (NEW) matches AccentPresetSwatch's own wrapping border exactly
            (see that component's own JSX a few dozen lines up) — without
            it, this Badge's solid fill reaches the full 28×28 box edge to
            edge while every neighbouring swatch's visible colour disc is
            actually only 24×24 (28px border-box minus its own 2px ring),
            so even at an IDENTICAL measured bounding box the reset control
            read as visibly BIGGER (jdp: "der Reset-Badge ist größer als die
            Farbfelder") — an optical-weight mismatch a bounding-box
            measurement alone can't catch. The border closes that gap: same
            28×28 outer box, same 24×24 inner content area as every swatch
            beside it.
              IconResetArrow (redesigned — see that icon's own header
            comment for why the old thinner ring/arrowhead is gone) — the
            established "counter-clockwise arrow = reset" convention already
            used by Sidebar.tsx's IconRecovery/IconRestore, redrawn bolder
            specifically for this small-badge-in-a-busy-row role.
              ALWAYS rendered, disabled (never conditionally unmounted) — see
            this component's own header comment for the earlier "isn't a badge
            at all" root cause that established this pattern.
              size="icon" — the app's ONE square-icon-badge size (32px), not
            a number this control derives from its own row. It used to be
            28px, chosen to match the swatches beside it; when every square
            icon badge in the app was unified on 32px, the swatch row moved
            WITH the badge (AccentPresetSwatch's inner disc w-6 → w-7, so its
            bordered wrapper measures 32px, and the custom-accent swatch
            w-6 → w-8, which also closes a stray 24-vs-28px gap it had against
            its own presets). Badges set the size, swatches follow — a swatch
            is a colour disc, not an icon badge, so it is the free variable
            here. Same treatment on the rainbow-palette reset below.
              tip (not title/ariaLabel) — same convention: an icon-only
            trigger gets IconTipButton's real hover/focus bubble, not a silent
            native title balloon. Text is settings.accentReset, which NAMES
            BOTH halves of what one click now does ("Reset accent color and
            presets") — the old settings.accentPresetsReset ("Reset presets")
            described only half the action and would have been an outright lie
            on a control that also resets the live accent. Kept identical to
            the rainbow-palette reset badge below in shape/tone/size/glyph/
            border — the established "these two mirror each other" pairing.
              tone="neutral" IS A DELIBERATE EXCEPTION to "every icon badge
            goes in the colour engine", written down here so a later sweep
            does not helpfully convert it to tone="active" — the round that
            put "the last six grey icon badges into the colour engine"
            (FolderBrowser's browse badge, the Registry add/remove pair,
            VMSSHCard's two copy badges, the Recovery-Kit download badge)
            left these two behind on purpose and never said why. The reason,
            measured live: this badge is the LAST child of a row of 32px
            `border-2` colour SWATCHES, and it is chrome-identical to them —
            same box, same border, same radius. An accent/rainbow fill would
            make it read as one more swatch in the row, i.e. as a colour you
            can PICK, when clicking it instead THROWS AWAY the picked colour.
            The neutral fill is what distinguishes "reset this row" from "a
            member of this row"; it is not an un-migrated grey. Same reasoning
            applies verbatim to the rainbow-palette reset badge below, whose
            row of eight swatches makes the clash even more literal. */}
        {/* DELIBERATE EXCEPTION to the #178 button engine (jdp, 2026-08-29:
            "der Resetbutton von Akzentfarbe und Farbpallette soll so bleiben
            wie er war"). It stays a square icon badge with a NEUTRAL fill,
            for the reason spelled out above: this control sits inside the row
            of colours it throws away, and giving it a label and an accent
            fill would make it read as a member of that row. Do not fold this
            into Button. */}
        <Badge
          as="button"
          shape="square"
          size="icon"
          tone="neutral"
          tip={t("settings.accentReset")}
          onClick={() => {
            selectAccent(DEFAULT_ACCENT);
            setPresets(setAccentPresets(DEFAULT_ACCENT_PRESETS));
          }}
          disabled={nothingToReset}
          className="border-2 border-carbon-border"
        >
          <IconResetArrow />
        </Badge>
      </div>
    </div>
  );
}

// "Reset to default" arrow — the Accent Card's preset-row reset button and
// the Off-site tab's rainbow-palette reset button (two separate call sites,
// same icon-only square/circle badge). FILLED (design-language.md "Icon
// glyphs"): a filled ring segment + a solid triangular arrowhead, not a
// `stroke`-width line — same construction family as Sidebar.tsx's own
// IconRecovery/IconRestore.
//
// REDRAWN, BOLDER (GlimStone follow-up round, jdp re-reporting after a prior
// round's fix didn't hold up: "Der Reset-Badge soll einen besseren Glyph
// bekommen" — the previous thinner ring, at this file's 16×16 icon-only-
// badge scale sitting directly inside a busy row of 8 bright colour
// swatches, read as an ambiguous "C" rather than an unambiguous reset arrow).
// Thickened the ring from a 1.8px band (outer r=6 / inner r=4.2) to a 3.0px
// band (outer r=6 / inner r=3.3) and enlarged the arrowhead proportionally —
// verified live at actual 28×28 badge size, on both a dark and a light
// neutral chip background (this glyph's own two real call sites are now
// both `tone="neutral"`, never a bright fill), reading unambiguously as a
// counter-clockwise "undo/reset" arrow at that size where the old thinner
// version did not.
//
// DELIBERATELY DIVERGES from IconRecovery/IconRestore now (previously
// identical path data to both — see each of THEIR OWN header comments,
// which this round leaves untouched about EACH OTHER but corrects about
// this icon): those two live in contexts with no adjacent competing colour
// (a full-page nav-tab icon; a plain single badge in a list row) and keep
// the original, slightly finer proportions that already read clearly there.
// This icon's one specific job — reading as "reset," at 16px, positioned
// directly beside 8 saturated colour swatches actively competing for the
// eye — is a strictly harder legibility case that earns its own bolder
// tuning rather than inheriting a ratio measured for an easier one. Renamed
// from IconResetSwirl to IconResetArrow to make that split explicit: this is
// no longer "the same glyph, reused," it is a purpose-built sibling.
export function IconResetArrow() {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true">
      <path d="M14 8A6 6 0 1 1 8 2L8 4.7A3.3 3.3 0 1 0 11.3 8Z" />
      <path d="M8 1 3.5 3 8 5Z" />
    </svg>
  );
}
