import { useCallback, useState } from "react";
import { useT } from "./i18n";

// useReveal holds the show/hide state and translated labels for RevealInput,
// which stays a hookless component so tests can call it with plain props:
//
//   const reveal = useReveal();
//   <RevealInput {...reveal} value={...} onChange={...} />
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
