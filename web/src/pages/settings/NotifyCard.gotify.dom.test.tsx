// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { NotifyConfig, OkEnvelope } from "../../lib/api";

const saved: Partial<NotifyConfig>[] = [];
let answer: OkEnvelope = { ok: true };

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
      return Promise.resolve(answer);
    },
  };
});

const { NotifyCard } = await import("./NotifyCard");

function Harness() {
  const { t } = useT();
  return <NotifyCard t={t} platformKind="unraid" sections={[]} />;
}

function show() {
  render(
    <MemoryRouter>
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

async function pause(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  answer = { ok: true };
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
  saved.length = 0;
});

describe("the Gotify channel", () => {
  it("shows its server, a stored token without its value, and both priorities", async () => {
    show();
    expect(((await screen.findByLabelText("Server URL")) as HTMLInputElement).value).toBe("https://push.example");
    expect(screen.getByText("Saved. Leave the field empty to keep it.")).toBeTruthy();
    expect(screen.getByRole("combobox", { name: "Priority on failure" }).textContent).toContain("8");
    expect(screen.getByRole("combobox", { name: "Priority on success" }).textContent).toContain("5");
  });

  it("saves a picked priority with the token left blank, so the stored one stays", async () => {
    show();
    fireEvent.click(await screen.findByRole("combobox", { name: "Priority on failure" }));
    await pause(50);
    fireEvent.click(screen.getByRole("option", { name: "10" }));
    await pause(50);
    expect(saved.at(-1)).toMatchObject({ gotifyEnabled: true, gotifyPriorityFailure: 10, gotifyToken: "" });
  });

  it("holds a new server address until the token is typed again, then sends both", async () => {
    show();
    fireEvent.change(await screen.findByLabelText("Server URL"), { target: { value: "https://other.example" } });
    await pause(1000);
    expect(saved).toHaveLength(0);
    expect(screen.getByText("The new address is saved once you enter this again.")).toBeTruthy();
    fireEvent.change(screen.getByLabelText("App token"), { target: { value: "new-token" } });
    await pause(1000);
    expect(saved.at(-1)).toMatchObject({ gotifyUrl: "https://other.example", gotifyToken: "new-token" });
  });

  it("says a refused save in the reader's language", async () => {
    answer = { ok: false, error: "a Gotify priority runs from 0 to 10", code: "gotify-priority" };
    show();
    fireEvent.click(await screen.findByRole("combobox", { name: "Priority on success" }));
    await pause(50);
    fireEvent.click(screen.getByRole("option", { name: "2" }));
    expect(await screen.findByText("A Gotify priority runs from 0 to 10.")).toBeTruthy();
  });
});
