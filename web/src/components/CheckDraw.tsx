/**
 * A checkmark that draws itself when an action finishes successfully. Sized to
 * sit inline with `text-sm`/`text-xs` body text.
 *
 * `pathLength="1"` makes the path one unit long whatever its geometry, so the
 * `.glim-check-draw` animation in index.css can run the dash offset from 1 to 0
 * without measuring it. The stroke takes `currentColor`, the status colour of
 * the element around it. Call sites render it only on the transition to
 * success, so mounting is what starts the animation.
 */
export function CheckDraw() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="inline-block shrink-0 align-[-1px]"
      aria-hidden="true"
    >
      <path pathLength="1" className="glim-check-draw" d="M3 8.5L6.5 12L13 4" />
    </svg>
  );
}
