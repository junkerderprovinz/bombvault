import type { Ref } from "react";

/** Sets one DOM node on every ref passed in, so a component's own ref and a
 *  caller's can both point at the same element. */
export function mergeRefs<T>(...refs: (Ref<T> | undefined)[]): (el: T | null) => void {
  return (el) => {
    for (const ref of refs) {
      if (typeof ref === "function") ref(el);
      else if (ref) ref.current = el;
    }
  };
}
