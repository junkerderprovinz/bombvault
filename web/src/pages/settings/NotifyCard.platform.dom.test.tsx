// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getNotify: () => Promise.resolve({ ok: true, notify: { unraid: true } }),
  };
});

const { NotifyCard } = await import("./NotifyCard");

function Harness({ platformKind }: { platformKind: string }) {
  const { t } = useT();
  return <NotifyCard t={t} platformKind={platformKind} sections={[]} />;
}

function renderOn(platformKind: string) {
  render(
    <MemoryRouter>
      <I18nProvider>
        <ToastProvider>
          <Harness platformKind={platformKind} />
        </ToastProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

afterEach(cleanup);

describe("the Unraid notification banner", () => {
  it("names TrueNAS and leaves out the Unraid mount advice there", async () => {
    renderOn("truenas");
    expect(await screen.findByText(/This host runs TrueNAS/)).toBeTruthy();
    expect(screen.queryByText(/\/host\/boot/)).toBeNull();
  });

  it("keeps the mount advice on a generic host, which may be Unraid without the mount", async () => {
    renderOn("generic");
    expect(await screen.findByText(/\/host\/boot/)).toBeTruthy();
  });
});
