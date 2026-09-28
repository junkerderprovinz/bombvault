// Instances puts everything about other BombVault instances behind one entry,
// a tab each. Pairing comes first, because the other three build on its
// group: a receiver watches a repository a member sends here, a fleet peer is
// a member asked for its scorecard, and a pull source is a member's
// repository this box fetches from. Pairing shows whenever one of those is
// on; each of the three is gated on its own setting.
import { useEffect, useState, type CSSProperties } from "react";
import { getSettings } from "../lib/api";
import type { Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { PageTitle } from "../components/PageTitle";
import { Selector } from "../components/Selector";
import { useIsDesktop } from "../lib/useMediaQuery";
import { IconReceiver, IconFleet, IconDownload } from "../components/navGlyphs";
import { Receiver } from "./Receiver";
import { Fleet } from "./Fleet";
import { Pull } from "./Pull";
import { Pairing } from "./Pairing";
import { IconLink } from "../components/glyphs";

export const INSTANCE_TABS = ["pairing", "receiver", "fleet", "pull"] as const;
export type InstanceTab = (typeof INSTANCE_TABS)[number];

const TAB_ICON: Record<InstanceTab, React.ReactNode> = {
  pairing: <IconLink />,
  receiver: <IconReceiver />,
  fleet: <IconFleet />,
  // Receiver's glyph twice would be ambiguous in glyph mode, and pulling moves
  // data toward this box.
  pull: <IconDownload />,
};

function isTab(v: string): v is InstanceTab {
  return (INSTANCE_TABS as readonly string[]).includes(v);
}

/** Which tab a fresh mount should show: the one in the URL hash if it names
 *  one, else the first. Settings has not loaded yet at this point, so the
 *  choice is corrected below once it has. */
function tabFromHash(): InstanceTab {
  try {
    const h = window.location.hash.replace(/^#/, "");
    return isTab(h) ? h : "pairing";
  } catch {
    return "pairing";
  }
}

export function Instances() {
  const { t } = useT();
  const isDesktop = useIsDesktop();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [tab, setTab] = useState<InstanceTab>(tabFromHash);
  const [tabDir, setTabDir] = useState<1 | -1>(1);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok && res.settings) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  // A hash typed or pasted into the address bar switches the tab, the same way
  // Settings' own strip behaves.
  useEffect(() => {
    function onHash() {
      const h = window.location.hash.replace(/^#/, "");
      if (isTab(h)) setTab(h);
    }
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  // One literal key per tab. A key built from the tab id would make
  // i18n.orphans.test.ts count every page title as used; its scanner reads the
  // source text, comments included.
  const tabLabel: Record<InstanceTab, string> = {
    pairing: t("pairing.title"),
    receiver: t("receiver.title"),
    fleet: t("fleet.title"),
    pull: t("pull.title"),
  };

  const enabled: Record<InstanceTab, boolean> = {
    pairing: (settings?.receiverEnabled || settings?.fleetEnabled || settings?.pullEnabled) ?? false,
    receiver: settings?.receiverEnabled ?? false,
    fleet: settings?.fleetEnabled ?? false,
    pull: settings?.pullEnabled ?? false,
  };
  const visible = INSTANCE_TABS.filter((k) => enabled[k]);

  // Nothing renders before settings arrive. A tab from the URL whose setting is
  // off falls back to the first one that is on instead of a blank panel.
  const active: InstanceTab | null = visible.includes(tab) ? tab : (visible[0] ?? null);

  function choose(next: InstanceTab) {
    const from = visible.indexOf(active ?? next);
    const to = visible.indexOf(next);
    if (from !== -1 && to !== -1) setTabDir(to > from ? 1 : -1);
    setTab(next);
    try {
      window.history.replaceState(null, "", `#${next}`);
    } catch {
      /* history unavailable; the tab still switches */
    }
  }

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("instances.title")}</PageTitle>

      {visible.length > 1 && (
        <Selector
          items={visible.map((k) => ({
            id: k,
            label: tabLabel[k],
            icon: TAB_ICON[k],
            title: tabLabel[k],
          }))}
          label={t("instances.title")}
          select="one"
          active={active}
          onChange={(id) => {
            if (isTab(id)) choose(id);
          }}
          size="lg"
          equalWidth
          // Separate tabs, as in Settings.
          variant="chip"
          inline={isDesktop}
        />
      )}

      {/* Keyed on the tab so the slide replays on every switch, with --tab-dir
          sending it the way the click went, as in Settings. */}
      {active && (
        <div key={active} className="glim-tab-slide flex flex-col" style={{ "--tab-dir": tabDir } as CSSProperties}>
          {active === "pairing" && <Pairing embedded />}
          {active === "receiver" && <Receiver embedded />}
          {active === "fleet" && <Fleet embedded />}
          {active === "pull" && <Pull embedded />}
        </div>
      )}
    </div>
  );
}
