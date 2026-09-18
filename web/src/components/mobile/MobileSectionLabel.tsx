import type { TranslationKey, useT } from "../../lib/i18n";

// ---------------------------------------------------------------------------
// MobileSectionLabel — the ONE section header of the mobile card language: a
// 12px uppercase letter-spaced label sitting between the cards (not a
// desktop-style overlapping Badge heading; the phone cards are flat, compact
// boxes). Kept as a shared component so every page block carrying
// phone-width sections composes the same label instead of re-authoring a
// private copy — the same single-registry discipline the nav model applies
// to destinations, applied to a component: one source of the label markup,
// consumers import it back.
// ---------------------------------------------------------------------------

export function MobileSectionLabel({ t, labelKey }: { t: ReturnType<typeof useT>["t"]; labelKey: TranslationKey }) {
  return (
    <h2 className="px-0.5 text-xs font-semibold uppercase tracking-[0.09em] text-carbon-textMuted">
      {t(labelKey)}
    </h2>
  );
}
