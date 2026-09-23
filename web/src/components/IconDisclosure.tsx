// The disclosure chevron, pointing right when closed and down when open. It is
// not in the generated glyphs.tsx because it carries state, which only the call
// site knows.
//
// Closed, it points toward the content it would reveal, which is left on an
// RTL page; open, it points down in both. Hence `rotate-90` when open and
// `rtl:rotate-180` only when closed.
export function IconDisclosure({ open }: { open: boolean }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 12 12"
      fill="none"
      className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
    >
      <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
    </svg>
  );
}
