/** BusyRing is the turning ring a control shows while the server works on what
 *  it asked for. */
export function BusyRing() {
  return (
    <span
      className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
      style={{ borderColor: "currentColor", borderTopColor: "transparent" }}
    />
  );
}
