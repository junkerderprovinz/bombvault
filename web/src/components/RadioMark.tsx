/** RadioMark is the mark of a choice of one: a ring, with a dot in it once the
 *  choice is made. It stays round in every shape, so it never reads as a box
 *  to tick. */
export function RadioMark({ on, className = "" }: { on: boolean; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={`flex h-[18px] w-[18px] flex-none items-center justify-center rounded-full border-2 ${
        on ? "border-accent" : "border-carbon-textMuted"
      } ${className}`}
    >
      {on && <span className="h-2 w-2 rounded-full bg-accent" />}
    </span>
  );
}
