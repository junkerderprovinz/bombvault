import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";

/**
 * LateRedirect is Navigate for a page that redirects once its data is in. It
 * replaces the address only while the browser still shows that page: a tap
 * that left it has already pushed the next one, the router renders the old
 * page until that tap's render commits, and a plain Navigate would replace
 * the page the tap opened.
 */
export function LateRedirect({ to }: { to: string }) {
  const navigate = useNavigate();
  const { pathname, search, hash } = useLocation();
  useEffect(() => {
    const here = window.location;
    if (here.pathname !== pathname || here.search !== search || here.hash !== hash) return;
    navigate(to, { replace: true });
  }, [navigate, to, pathname, search, hash]);
  return null;
}
