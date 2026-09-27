// PageTitle names a routed page for assistive tech only. The sidebar entry
// for the tab is already visible and highlighted, so the page itself carries
// no visible heading; this h1 stays for a screen reader's page-title
// announcement and its heading list.
//
// sr-only is `position: absolute`, which takes the h1 out of its parent's
// flex layout entirely, so a page with nothing else in its header still opens
// flush with the rail's top line (see index.css's rail-alignment rule, which
// keys off glim-page-title to still find the badge of a card that follows
// directly).
export function PageTitle({ children }: { children: string }) {
  return <h1 className="glim-page-title sr-only">{children}</h1>;
}
