/**
 * A checkmark that draws itself in when an action finishes successfully. Sized
 * to sit inline with `text-sm`/`text-xs` body text.
 *
 * The mark is a filled shape like every glyph but the info bubble's (i), so it
 * draws in as a wipe: `.glim-check-draw` in index.css opens a clip from the
 * foot of the check to its tip. It takes `currentColor`, the status colour of
 * the element around it. Call sites render it only on the transition to
 * success, so mounting is what starts the animation.
 */
export function CheckDraw() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 16 16"
      fill="currentColor"
      className="glim-check-draw inline-block shrink-0 align-[-1px]"
      aria-hidden="true"
    >
      <path d="M3.78 7.72 6.42 10.36 12.15 3.31A1.1 1.1 0 0 1 13.85 4.69L7.35 12.69A1.1 1.1 0 0 1 5.72 12.78L2.22 9.28A1.1 1.1 0 0 1 3.78 7.72Z" />
    </svg>
  );
}
