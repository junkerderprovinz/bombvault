// @vitest-environment jsdom
// A receiver another instance of the group runs is offered above the
// providers, and picking it fills in the rest-server form with this
// instance's own login on it.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { GroupReceiver, Provider } from "../../lib/api";

const rest: Provider = { id: "rest", name: "rest-server", backend: "", group: "server", route: "rest", mark: "IconShield", auth: "login", lock: true };

const receivers: GroupReceiver[] = [
  { memberId: "m-attic", name: "attic", url: "http://192.168.1.20:8001", needsAddress: false },
  { memberId: "m-barn", name: "barn", needsAddress: true },
];

const asked: string[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getProviders: () => Promise.resolve({ ok: true, providers: [rest], backends: {} }),
    listGroupReceivers: () => Promise.resolve({ ok: true, receivers }),
    groupReceiverLogin: (memberId: string) => {
      asked.push(memberId);
      return Promise.resolve({
        ok: true,
        login: { name: "attic", url: "http://192.168.1.20:8001/cellar", user: "cellar", password: "s3cret" },
      });
    },
  };
});

const { DestinationWizard } = await import("./DestinationWizard");

afterEach(() => {
  cleanup();
  asked.length = 0;
});

async function renderWizard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <DestinationWizard onDone={() => {}} onCancel={() => {}} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

describe("DestinationWizard with receivers from the group", () => {
  it("fills the rest-server form with the picked member's receiver", async () => {
    await renderWizard();
    expect(screen.getByText(en["dest.group.fromGroup"])).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /^attic / }));
    });

    expect(asked).toEqual(["m-attic"]);
    expect(screen.getByDisplayValue("http://192.168.1.20:8001/cellar")).toBeTruthy();
    expect(screen.getByDisplayValue("cellar")).toBeTruthy();
    expect((screen.getByDisplayValue("s3cret") as HTMLInputElement).type).toBe("password");
    expect(screen.getByDisplayValue("attic")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["offsite.test"] })).toBeTruthy();
  });

  it("offers a member reached only through the relay without letting it be picked", async () => {
    await renderWizard();
    expect(screen.queryByRole("button", { name: /barn/ })).toBeNull();
    expect(screen.getByLabelText(`barn ${en["dest.receiver.needsAddress"]}`)).toBeTruthy();
  });
});
