// @vitest-environment jsdom
// The card picks media servers by image name until the user chooses, and a
// choice has to stick: once saved without mediaServersAuto the backend stops
// guessing.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { StreamingSettings } from "../../lib/api";

const saved: StreamingSettings[] = [];
let streamingNow = "";
let refuse = false;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getStreaming: () =>
      Promise.resolve({
        ok: true,
        settings: { enabled: false, mediaServers: ["plex"], mediaServersAuto: true, thresholdMbit: 2, limitKiB: 512, holdMin: 5 },
        candidates: [
          { name: "jellyfin", image: "jellyfin/jellyfin", hostNetwork: true },
          { name: "plex", image: "plexinc/pms-docker", hostNetwork: false },
        ],
        streaming: streamingNow,
      }),
    setStreaming: (v: StreamingSettings) => {
      saved.push(v);
      return Promise.resolve(refuse ? { ok: false, error: "database is locked" } : { ok: true });
    },
  };
});

const { StreamingCard } = await import("./StreamingCard");

function Harness() {
  const { t } = useT();
  return <StreamingCard t={t} />;
}

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

afterEach(() => {
  cleanup();
  saved.length = 0;
  streamingNow = "";
  refuse = false;
});

it("shows the media servers picked by image name", async () => {
  await renderCard();
  expect(screen.getByText("plex")).toBeTruthy();
  expect(screen.queryByText(en["streaming.none"])).toBeNull();
});

it("switching the throttle on keeps the picked servers automatic", async () => {
  await renderCard();
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: new RegExp(en["streaming.toggle"]) }));
  });
  expect(saved).toHaveLength(1);
  expect(saved[0].enabled).toBe(true);
  expect(saved[0].mediaServersAuto).toBe(true);
});

it("choosing a server stores the list as a choice", async () => {
  await renderCard();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["streaming.pick"] }));
  });
  const jellyfin = screen.getByRole("option", { name: /jellyfin/ });
  expect(jellyfin.textContent).toContain(en["streaming.hostNetwork"]);
  await act(async () => {
    fireEvent.click(jellyfin);
  });
  expect(saved.at(-1)).toMatchObject({ mediaServers: ["jellyfin", "plex"], mediaServersAuto: false });
});

it("names the server whose stream slows the copies", async () => {
  streamingNow = "plex";
  await renderCard();
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: new RegExp(en["streaming.toggle"]) }));
  });
  expect(screen.getByText(en["streaming.now"].replace("{name}", "plex"))).toBeTruthy();
});

it("takes back a server choice the server refused, so the next save leaves it out", async () => {
  await renderCard();
  refuse = true;
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["streaming.pick"] }));
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("option", { name: /jellyfin/ }));
  });
  expect(screen.getByRole("option", { name: /jellyfin/ }).getAttribute("aria-selected")).toBe("false");
  refuse = false;
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: new RegExp(en["streaming.toggle"]) }));
  });
  expect(saved.at(-1)).toMatchObject({ enabled: true, mediaServers: ["plex"], mediaServersAuto: true });
});
