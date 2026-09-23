// Instances puts the three pages about another BombVault behind one entry, a
// tab each. They stay separate pages because they hold different things: a
// receiver is a repository somebody else sent here (location plus their
// APP_KEY), a fleet peer is a running instance asked over HTTP for its
// scorecard (URL plus bearer token), and a pull source is somebody else's
// repository this box fetches from. Each tab is gated on its own setting, and
// with only one switched on there is no strip.
import { useEffect, useState, type CSSProperties } from "react";
import { getSettings } from "../lib/api";
import type { Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { Selector } from "../components/Selector";
import { IconReceiver, IconFleet, IconDownload } from "../components/navGlyphs";
import { Receiver } from "./Receiver";
import { Fleet } from "./Fleet";
import { Pull } from "./Pull";

export const INSTANCE_TABS = ["receiver", "fleet", "pull"] as const;
export type InstanceTab = (typeof INSTANCE_TABS)[number];

const TAB_ICON: Record<InstanceTab, React.ReactNode> = {
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
    return isTab(h) ? h : "receiver";
  } catch {
    return "receiver";
  }
}

export function Instances() {
  const { t } = useT();
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
    receiver: t("receiver.title"),
    fleet: t("fleet.title"),
    pull: t("pull.title"),
  };

  const enabled: Record<InstanceTab, boolean> = {
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
    <div className={PAGE_SHELL}>
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("instances.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("instances.subtitle")}</p>
      </div>

      {visible.length > 1 && (
        <div className="inline-flex self-start max-w-full">
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
          />
        </div>
      )}

      {/* Keyed on the tab so the slide replays on every switch, with --tab-dir
          sending it the way the click went, as in Settings. */}
      {active && (
        <div key={active} className="glim-tab-slide flex flex-col" style={{ "--tab-dir": tabDir } as CSSProperties}>
          {active === "receiver" && <Receiver embedded />}
          {active === "fleet" && <Fleet embedded />}
          {active === "pull" && <Pull embedded />}
        </div>
      )}
    </div>
  );
}
