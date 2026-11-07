/**
 * shake plays .glim-shake once on `el`, for a control that refused what was
 * asked of it. The class has to be gone for a whole frame before it comes
 * back, or the animation does not start again. Returns the cleanup.
 */
export function shake(el: HTMLElement): () => void {
  el.classList.remove("glim-shake");
  let inner = 0;
  const outer = requestAnimationFrame(() => {
    inner = requestAnimationFrame(() => el.classList.add("glim-shake"));
  });
  const done = (e: AnimationEvent) => {
    if (e.target === el) el.classList.remove("glim-shake");
  };
  el.addEventListener("animationend", done);
  return () => {
    cancelAnimationFrame(outer);
    cancelAnimationFrame(inner);
    el.removeEventListener("animationend", done);
  };
}
