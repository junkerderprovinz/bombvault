// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getNotify: () =>
      Promise.resolve({
        ok: true,
        notify: { scheduledSummary: false, notifyOnUpdate: false, channels: [{ id: "unraid", kind: "unraid", enabled: true }] },
      }),
  };
});

const { NotifyCard } = await import("./NotifyCard");

function Harness({ platformKind, platformSet }: { platformKind: string; platformSet: boolean }) {
  const { t } = useT();
  return <NotifyCard t={t} platformKind={platformKind} platformSet={platformSet} sections={[]} />;
}

function renderOn(platformKind: string, platformSet = false) {
  render(
    <MemoryRouter>
      <I18nProvider>
        <ToastProvider>
          <Harness platformKind={platformKind} platformSet={platformSet} />
        </ToastProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

afterEach(cleanup);

/** The banner stands in the channel's window, which Edit opens. */
async function openUnraid() {
  fireEvent.click(await screen.findByRole("button", { name: en["common.edit"] }));
}

describe("the Unraid notification banner", () => {
  it("marks the row of a channel that sends nothing on this host", async () => {
    renderOn("truenas");
    expect(await screen.findByText(en["notify.idle"])).toBeTruthy();
  });

  it("leaves the row unmarked on Unraid", async () => {
    renderOn("unraid");
    await screen.findByRole("button", { name: en["common.edit"] });
    expect(screen.queryByText(en["notify.idle"])).toBeNull();
  });

  it("names TrueNAS and leaves out the Unraid mount advice there", async () => {
    renderOn("truenas");
    await openUnraid();
    expect(await screen.findByText(/This host runs TrueNAS/)).toBeTruthy();
    expect(screen.queryByText(/\/host\/boot/)).toBeNull();
  });

  it("keeps the mount advice on a generic host, which may be Unraid without the mount", async () => {
    renderOn("generic");
    await openUnraid();
    expect(await screen.findByText(/\/host\/boot/)).toBeTruthy();
  });

  it("names the PLATFORM setting and leaves out the mount advice when PLATFORM chose generic", async () => {
    renderOn("generic", true);
    await openUnraid();
    expect(await screen.findByText(/PLATFORM sets this host to "generic"/)).toBeTruthy();
    expect(screen.queryByText(/\/host\/boot/)).toBeNull();
  });
});
