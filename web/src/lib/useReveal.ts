import { useCallback, useState } from "react";
import { useT } from "./i18n";

// useReveal — local show/hide state (+ its translated aria-labels) for the
// GlimStone "reveal eye" affordance (design-language.md, "The reveal eye
// (password/token fields)"; form-engine Task 6).
//
// Lives in lib/ next to this repo's other small hooks (useDragReorder,
// useOffsiteTargets) rather than in components/, and is deliberately the
// ONLY thing here that touches useState/useT. RevealInput.tsx stays a pure,
// hookless function component — the exact same shape as Toggle.tsx/Badge.tsx
// (props in, an element tree out), so it can be unit-tested by calling it
// directly with props, no renderer/jsdom needed, matching how this repo's
// test suite already treats Toggle/Badge (see Toggle.test.ts's header
// comment: "this repo's existing test suite is entirely `environment: node`
// with zero DOM-rendering infrastructure"). A call site does:
//
//   const reveal = useReveal();
//   <RevealInput {...reveal} value={...} onChange={...} />
//
export function useReveal() {
  const { t } = useT();
  const [visible, setVisible] = useState(false);
  const toggleVisible = useCallback(() => setVisible((v) => !v), []);
  return {
    visible,
    onToggleVisible: toggleVisible,
    showLabel: t("common.showValue"),
    hideLabel: t("common.hideValue"),
  };
}
