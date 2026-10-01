// @vitest-environment jsdom
// A settings search result opens the Integrations page and marks the card at
// once, so the API, Home Assistant and mDNS cards have to be there before their
// settings arrive, and their switches have to be found by name once they have.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { jumpTarget } from "./SettingsSearch";

let loaded = false;
const pending = () => new Promise<never>(() => undefined);

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listApiTokens: () => pending(),
    getHomeAssistant: () =>
      loaded
        ? Promise.resolve({
            ok: true,
            settings: {
              enabled: false,
              host: "",
              port: 1883,
              username: "",
              passwordSet: false,
              tls: false,
              prefix: "bombvault",
              buttons: false,
              nodeId: "",
              status: { connected: false, error: "" },
            },
            cooldownMinutes: 15,
            itemStartsPerDay: 4,
          })
        : pending(),
    getMdns: () =>
      loaded ? Promise.resolve({ ok: true, enabled: true, running: true, url: "https://bombvault.local:3443", instance: "BombVault", error: "" }) : pending(),
  };
});

const { ApiTokensCard } = await import("./ApiTokensCard");
const { HomeAssistantCard } = await import("./HomeAssistantCard");
const { NetworkCard } = await import("./NetworkCard");

async function renderCards() {
  let container!: HTMLElement;
  await act(async () => {
    ({ container } = render(
      <I18nProvider>
        <ToastProvider>
          <ApiTokensCard passwordSet={false} />
          <HomeAssistantCard />
          <NetworkCard />
        </ToastProvider>
      </I18nProvider>
    ));
  });
  return container;
}

afterEach(() => {
  cleanup();
  loaded = false;
});

it("offers the API, Home Assistant and mDNS cards as search targets while their settings load", async () => {
  const root = await renderCards();
  for (const card of [en["api.title"], en["ha.title"], en["mdns.title"]]) {
    expect(jumpTarget(root, { card })?.getAttribute("data-search-card")).toBe(card);
  }
});

it("finds the Home Assistant and mDNS switches by name once their settings are read", async () => {
  loaded = true;
  const root = await renderCards();
  for (const [card, row] of [
    ["ha.title", "ha.enable"],
    ["ha.title", "ha.tls"],
    ["ha.title", "ha.buttons"],
    ["mdns.title", "mdns.enable"],
  ] as const) {
    expect(jumpTarget(root, { card: en[card], row: en[row] })?.getAttribute("data-search-row")).toBe(en[row]);
  }
});
