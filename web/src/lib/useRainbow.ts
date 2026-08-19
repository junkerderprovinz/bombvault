import { useSyncExternalStore } from "react";
import { rainbowState, subscribeRainbow, type RainbowState } from "./appearance";

/**
 * useRainbow is the React binding for the document-level rainbow state
 * (GlimStone form-engine Phase 2, Task 2). appearance.ts stays framework-free
 * (Task 1's own reasoning — see that file's header) so a sibling app can copy
 * it verbatim; this hook is the one place a React component subscribes.
 *
 * A caller doesn't need the returned value — it calls this once per list
 * (not once per row: "the palette changes for every row at once anyway",
 * matching knightloader/web/src/components/TaskList.tsx's TaskListCard) purely
 * to register for re-renders, then reads rainbowAt()/hueVars() directly during
 * that render. That is what makes rainbow on/off/reactive/rotate/palette
 * edits repaint every subscribed list live, with no page reload.
 *
 * useSyncExternalStore rather than an effect + local state: the palette is
 * read during render to colour a row, so a component that learned about a
 * change one paint late would show the previous colour for a frame every time
 * a setting changes — identical reasoning to knightloader's own copy of this
 * file, verbatim otherwise.
 */
export function useRainbow(): RainbowState {
  return useSyncExternalStore(subscribeRainbow, rainbowState, () => rainbowState());
}
