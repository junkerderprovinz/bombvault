import { useEffect, useRef, useState, type CSSProperties } from "react";
import { PageTitle } from "../components/PageTitle";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { Navigate, useLocation, useParams } from "react-router-dom";
import { SettingsRail } from "../components/SettingsRail";
import { SettingsSearch, jumpTarget, markHit, type SearchJump } from "./settings/SettingsSearch";
import {
  FALLBACK_PAGE,
  LEGACY_HASH,
  isSettingsPage,
  orderPages,
  readLastPage,
  readOrder,
  writeLastPage,
  writeOrder,
  type SettingsPageId,
} from "./settings/settingsPages";
import { SettingsStoreContext, useSettingsStore } from "./settings/settingsStore";
import { PAGE_COMPONENTS } from "./settings/pages";

// bv-convention-exception: page-uses-page-shell: the rail stands beside the
// page, and the content column next to it carries PAGE_SHELL_RESPONSIVE.
export function SettingsPage() {
  const { t } = useT();
  const { push } = useToast();
  const store = useSettingsStore();
  const { settings, loadError } = store;

  const { page: param } = useParams();
  const page: SettingsPageId = isSettingsPage(param) ? param : FALLBACK_PAGE;
  const location = useLocation();
  const { hash } = location;
  // /settings alone names no page: a legacy hash from an old link leads to the
  // page holding that card, otherwise the page opened last.
  const redirect = isSettingsPage(param)
    ? null
    : param === undefined
      ? (LEGACY_HASH[hash.replace(/^#/, "")] ?? readLastPage())
      : FALLBACK_PAGE;
  const [order, setOrder] = useState(readOrder);
  const pages = orderPages(order);
  // A page slides in from the side its tile sits on in the rail.
  const shownPage = useRef(page);
  const pageDir = useRef<1 | -1>(1);
  if (shownPage.current !== page) {
    const ids = pages.map((p) => p.id);
    pageDir.current = ids.indexOf(page) > ids.indexOf(shownPage.current) ? 1 : -1;
    shownPage.current = page;
  }
  // Only a page named in the address counts: /settings alone renders the
  // fallback for a moment before its redirect, and must not overwrite it.
  useEffect(() => {
    if (isSettingsPage(param)) writeLastPage(param);
  }, [param]);

  // A hash such as /settings/integrity#anomalies names a card below the fold,
  // or a field that should take the cursor, and a search result names a card
  // and row to mark. Anything else opens a page at its top, since the scroller
  // is shared by every page.
  const settingsLoaded = settings !== null;
  const handledJump = useRef("");
  useEffect(() => {
    if (!settingsLoaded) return;
    const jump = (location.state as { jump?: SearchJump } | null)?.jump;
    const anchor = hash.replace(/^#/, "");
    if (jump && handledJump.current !== location.key) {
      handledJump.current = location.key;
      const content = document.querySelector<HTMLElement>("[data-settings-content]");
      const target = content && (jump.card || jump.row) ? jumpTarget(content, jump) : null;
      if (target) {
        // jsdom has no scrollIntoView.
        target.scrollIntoView?.({ block: "center" });
        markHit(target);
        // Cards that load their own settings grow after the jump, this one or
        // those above it, so the target is centred again while they settle.
        if (content && typeof ResizeObserver !== "undefined") {
          const settle = new ResizeObserver(() => target.scrollIntoView({ block: "center" }));
          settle.observe(content);
          window.setTimeout(() => settle.disconnect(), 1200);
        }
      } else if (jump.card) {
        push(t("settings.search.notShown").replace("{name}", jump.row ?? jump.card), "warn");
      }
      if (target || jump.card) return;
    }
    if (anchor) {
      const target = document.getElementById(anchor);
      if (target instanceof HTMLInputElement) {
        target.scrollIntoView?.({ block: "center" });
        target.focus();
      } else {
        target?.scrollIntoView?.({ block: "start" });
      }
      return;
    }
    document.getElementById("bv-main")?.scrollTo?.({ top: 0 });
  }, [location, hash, settingsLoaded, page]); // eslint-disable-line react-hooks/exhaustive-deps

  if (loadError) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-statusFail">{loadError}</p>
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      </div>
    );
  }

  const Page = PAGE_COMPONENTS[page];

  return (
    <div className="flex flex-1 gap-3 md:gap-10">
      {redirect && <Navigate to={`/settings/${redirect}`} replace />}
      <SettingsRail
        items={pages.map((p) => ({ id: p.id, label: t(p.label), icon: p.icon, to: `/settings/${p.id}` }))}
        active={page}
        label={t("settings.railLabel")}
        onReorder={(ids) => {
          setOrder(ids);
          writeOrder(ids);
        }}
      />
      <div data-settings-content className={`${PAGE_SHELL_RESPONSIVE} min-w-0 flex-1`}>
      <PageTitle>{t("settings.title")}</PageTitle>
      <SettingsSearch pages={pages} />
      {/* Keyed on the page, so the slide replays on every change of page. */}
      <div
        key={page}
        data-settings-page
        className="flex flex-col gap-6 md:gap-10 glim-tab-slide flex-1"
        style={{ "--tab-dir": pageDir.current } as CSSProperties}
      >
      <SettingsStoreContext.Provider value={{ ...store, settings }}>
        <Page />
      </SettingsStoreContext.Provider>
      </div>
      {store.confirmDialog}
      </div>
    </div>
  );
}
