// @vitest-environment jsdom
// The receiving server's window: the form sends what was entered and the
// window shows the outsider's password once afterwards; a partner's login can
// be revoked after a confirmation. Opened because something needs a receiving
// server, it starts with the form and says on closing whether one runs.
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
let forgotten = 0;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getReceiverServer: () => Promise.resolve({ ok: true, server: initial, defaultPort: 8000, hostMountRoot: "/host/user" }),
    setUpReceiverServer: (input: ReceiverServerInput) => {
      sent.push(input);
      return Promise.resolve({ ok: true, server: { ...server, port: input.port }, password: "one-time-pw", template: "download" });
    },
    forgetReceiverServer: () => {
      forgotten++;
      return Promise.resolve({ ok: true });
    },
    revokeReceiverLogin: (memberId: string) => {
      revoked.push(memberId);
      return Promise.resolve({ ok: true, server: { ...withPartners, logins: withPartners.logins.filter((l) => l.memberId !== memberId) } });
    },
  };
});

const { ReceiverServerWindow } = await import("./ReceiverServerWindow");

const closed: boolean[] = [];

function Opened({ lead }: { lead?: string }) {
  const { t } = useT();
  return <ReceiverServerWindow t={t} lead={lead} onClose={(running) => closed.push(running)} />;
}

async function renderWindow(lead?: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Opened lead={lead} />
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
  closed.length = 0;
  forgotten = 0;
});

describe("ReceiverServerWindow", () => {
  it("sets up a receiver from the form and shows the outsider's password once", async () => {
    await renderWindow();

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
    expect(screen.queryByRole("dialog", { name: en["receiver.server.setUp"] })).toBeNull();
    expect(screen.getByText(server.url)).toBeTruthy();
    expect(screen.getByText("vault-box")).toBeTruthy();
    expect(screen.getByText(en["receiver.server.protected"])).toBeTruthy();
    expect(screen.getByText(en["receiver.server.templateDownload"])).toBeTruthy();
    expect(screen.getByText("one-time-pw")).toBeTruthy();
    expect(screen.getByText(en["receiver.server.noLogins"])).toBeTruthy();
  });

  it("keeps the login for someone outside the group with the server it belongs to", async () => {
    initial = server;
    await renderWindow();
    expect(screen.getByText(en["receiver.server.outsideUser"])).toBeTruthy();
    expect(screen.getByText("vault-box")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: en["receiver.server.setUp"] })).toBeNull();
  });

  it("asks for the address when this BombVault is known only by a name a browser used", async () => {
    initial = { ...server, url: "", needsHost: true };
    await renderWindow();
    expect(screen.getByText(en["receiver.server.needsHost"])).toBeTruthy();
    expect(screen.queryByText(en["receiver.server.noAddress"])).toBeNull();
  });

  it("starts with the form and says why when something needs a receiving server", async () => {
    await renderWindow("This needs a receiving server.");
    const form = screen.getByRole("dialog", { name: en["receiver.server.setUp"] });
    expect(form.textContent).toContain("This needs a receiving server.");
    fireEvent.change(screen.getByPlaceholderText("user/restic"), { target: { value: "user/restic" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["receiver.server.startContinue"] }));
    });
    expect(sent).toHaveLength(1);
    expect(screen.getByText("one-time-pw")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["common.done"] }));
    expect(closed).toEqual([true]);
  });

  it("says on closing that no server runs when the form was left", async () => {
    await renderWindow("This needs a receiving server.");
    fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    fireEvent.click(screen.getByRole("button", { name: en["common.done"] }));
    expect(closed).toEqual([false]);
  });

  it("does not open the form by itself where a server already runs", async () => {
    initial = server;
    await renderWindow("This needs a receiving server.");
    expect(screen.queryByRole("dialog", { name: en["receiver.server.setUp"] })).toBeNull();
  });

  it("revokes a partner's login after asking", async () => {
    initial = withPartners;
    await renderWindow();
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

  it("stops offering the server only after asking", async () => {
    initial = withPartners;
    await renderWindow();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["receiver.server.forget"] }));
    });
    expect(forgotten).toBe(0);
    expect(screen.getByText(en["receiver.server.forgetAsk"])).toBeTruthy();

    const buttons = screen.getAllByRole("button", { name: en["receiver.server.forget"] });
    await act(async () => {
      fireEvent.click(buttons[buttons.length - 1]);
    });
    expect(forgotten).toBe(1);
  });
});
