// @vitest-environment jsdom
// The receiving server's card: the form sends what was entered and the card
// shows the outsider's password once afterwards; a partner's login can be
// revoked after a confirmation.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { ReceiverServer, ReceiverServerInput } from "../../lib/api";

const server: ReceiverServer = {
  containerName: "rest-server",
  folder: "user/restic",
  hostPath: "/mnt/user/restic",
  port: 8001,
  user: "vault-box",
  host: "",
  url: "http://192.168.1.20:8001",
  createdAt: 1,
  present: true,
  check: "protected",
  checkDetail: "",
  checkedAt: 0,
  logins: [],
};

const withPartners: ReceiverServer = {
  ...server,
  logins: [
    { memberId: "m-attic", name: "attic", user: "attic-login", createdAt: 1_700_000_000 },
    { memberId: "m-barn", name: "barn", user: "barn-login", createdAt: 1_700_000_000 },
  ],
};

let initial: ReceiverServer | null = null;
const sent: ReceiverServerInput[] = [];
const revoked: string[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getReceiverServer: () => Promise.resolve({ ok: true, server: initial, defaultPort: 8000, hostMountRoot: "/host/user" }),
    setUpReceiverServer: (input: ReceiverServerInput) => {
      sent.push(input);
      return Promise.resolve({ ok: true, server: { ...server, port: input.port }, password: "one-time-pw", template: "download" });
    },
    revokeReceiverLogin: (memberId: string) => {
      revoked.push(memberId);
      return Promise.resolve({ ok: true, server: { ...withPartners, logins: withPartners.logins.filter((l) => l.memberId !== memberId) } });
    },
  };
});

const { ReceiverServerCard } = await import("./ReceiverServerCard");

function Card() {
  const { t } = useT();
  return <ReceiverServerCard t={t} />;
}

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Card />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

afterEach(() => {
  cleanup();
  initial = null;
  sent.length = 0;
  revoked.length = 0;
});

describe("ReceiverServerCard", () => {
  it("sets up a receiver from the form and shows the outsider's password once", async () => {
    await renderCard();

    fireEvent.click(screen.getByRole("button", { name: en["receiver.server.setUp"] }));
    expect(screen.getByRole("dialog", { name: en["receiver.server.setUp"] })).toBeTruthy();
    const start = screen.getByRole("button", { name: en["receiver.server.start"] }) as HTMLButtonElement;
    expect(start.disabled).toBe(true);

    fireEvent.change(screen.getByPlaceholderText("user/restic"), { target: { value: "user/restic" } });
    fireEvent.change(screen.getByDisplayValue("8000"), { target: { value: "8001" } });
    expect(start.disabled).toBe(false);

    await act(async () => {
      fireEvent.click(start);
    });

    expect(sent).toEqual([{ folder: "user/restic", port: 8001, host: "" }]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText(server.url)).toBeTruthy();
    expect(screen.getByText("vault-box")).toBeTruthy();
    expect(screen.getByText(en["receiver.server.protected"])).toBeTruthy();
    expect(screen.getByText(en["receiver.server.templateDownload"])).toBeTruthy();
    expect(screen.getByText("one-time-pw")).toBeTruthy();
    expect(screen.getByText(en["receiver.server.noLogins"])).toBeTruthy();
  });

  it("revokes a partner's login after asking", async () => {
    initial = withPartners;
    await renderCard();
    expect(screen.getByText("attic")).toBeTruthy();
    expect(screen.getByText("barn")).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getAllByRole("button", { name: en["receiver.server.revoke"] })[0]);
    });
    expect(revoked).toEqual([]);
    expect(screen.getByText(en["receiver.server.revokeAsk"].replace("{name}", "attic"))).toBeTruthy();

    const buttons = screen.getAllByRole("button", { name: en["receiver.server.revoke"] });
    await act(async () => {
      fireEvent.click(buttons[buttons.length - 1]);
    });
    expect(revoked).toEqual(["m-attic"]);
    expect(screen.queryByText("attic-login")).toBeNull();
    expect(screen.getByText("barn-login")).toBeTruthy();
  });
});
