// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { NotifyConfig } from "../../lib/api";

const saved: Partial<NotifyConfig>[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getNotify: () =>
      Promise.resolve({
        ok: true,
        notify: { on: "failure", gotifyEnabled: true, gotifyUrl: "https://push.example", gotifyPriorityFailure: 8, gotifyPrioritySuccess: 5 },
        gotifyTokenSet: true,
      }),
    setNotify: (c: NotifyConfig) => {
      saved.push(c);
      return Promise.resolve({ ok: true });
    },
  };
});

const { NotifyCard } = await import("./NotifyCard");

function Harness() {
  const { t } = useT();
  return <NotifyCard t={t} platformKind="unraid" sections={[]} />;
}

afterEach(() => {
  cleanup();
  saved.length = 0;
});

describe("the Gotify channel", () => {
  it("shows its server, a stored token without its value, and both priorities", async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <Harness />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
    expect(((await screen.findByLabelText("Server URL")) as HTMLInputElement).value).toBe("https://push.example");
    expect((screen.getByLabelText("App token") as HTMLInputElement).placeholder).toBe("saved (leave blank to keep)");
    expect((screen.getByLabelText("Priority on failure") as HTMLInputElement).value).toBe("8");
    expect((screen.getByLabelText("Priority on success") as HTMLInputElement).value).toBe("5");
  });

  it("saves a changed priority with the token left blank, so the stored one stays", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <Harness />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
    fireEvent.change(await screen.findByLabelText("Priority on failure"), { target: { value: "10" } });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
    vi.useRealTimers();
    expect(saved.at(-1)).toMatchObject({ gotifyEnabled: true, gotifyPriorityFailure: 10, gotifyToken: "" });
  });
});
