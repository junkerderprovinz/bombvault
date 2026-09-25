// @vitest-environment jsdom
// The server reads a schedule on its own clock and the page shows every other
// time in the browser's zone, so a line on a server that runs on another clock
// has to say which one its time is on.
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";
import { browserOffsetSeconds, zoneLabel } from "../lib/scheduleZone";

const serverZone = { name: "UTC", offsetSeconds: browserOffsetSeconds() + 7200 };

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: {}, hostMountRoot: "/host", platform: "unraid", scheduleZone: serverZone }),
  };
});

const { EffectiveScheduleLine } = await import("./EffectiveScheduleLine");

afterEach(cleanup);

it("names the server's clock when the browser runs on another", async () => {
  render(
    <EffectiveScheduleLine
      effective={{ kind: "domain", spec: "daily 03:00", alsoSpec: "" }}
      domainLabelKey="jobs.zfsSection"
    />
  );
  const when = en["cadence.serverClock"]
    .replace("{when}", en["cadence.fmtDaily"].replace("{time}", "3:00"))
    .replace("{zone}", zoneLabel(serverZone));
  await waitFor(() => expect(screen.getByText(/Result/).closest("p")?.textContent).toContain(when));
});
