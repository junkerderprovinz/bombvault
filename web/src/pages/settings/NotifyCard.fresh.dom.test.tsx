// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
        notify: { on: "failure", webhookFormat: "", smtpPort: 0, smtpTls: "", gotifyEnabled: true, gotifyUrl: "https://push.example", gotifyPriorityFailure: 8, gotifyPrioritySuccess: 5 },
        gotifyTokenSet: false,
      }),
    setNotify: () => Promise.resolve({ ok: true }),
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

afterEach(cleanup);

describe("the notification card on a fresh install", () => {
  it("lights the policy that is in force", async () => {
    show();
    const failure = await screen.findByRole("tab", { name: en["notify.onFailure"] });
    await waitFor(() => expect(failure.getAttribute("aria-selected")).toBe("true"));
    expect(screen.getByRole("tab", { name: en["notify.onNever"] }).getAttribute("aria-selected")).toBe("false");
  });

  it("says a token typed here stays stored once its field is emptied", async () => {
    show();
    const token = (await screen.findByLabelText(en["notify.gotifyToken"])) as HTMLInputElement;
    fireEvent.change(token, { target: { value: "typed" } });
    await screen.findByText(en["settings.saved"], {}, { timeout: 3000 });
    fireEvent.change(token, { target: { value: "" } });
    await screen.findByText(en["notify.secretKept"]);
  });
});
