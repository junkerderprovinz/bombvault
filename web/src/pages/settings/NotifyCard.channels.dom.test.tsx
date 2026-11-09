// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { REDACTED, type NotifyAnswer, type NotifyChannel, type NotifyConfig } from "../../lib/api";

const stored = (): NotifyConfig => ({
  scheduledSummary: false,
  notifyOnUpdate: false,
  channels: [
    {
      id: "gotify",
      kind: "gotify",
      enabled: true,
      on: "always",
      gotify: { url: "https://push.example", token: REDACTED, priorityFailure: 8, prioritySuccess: 5 },
    },
    { id: "phone", kind: "ntfy", name: "Phone", enabled: false, ntfy: { url: "https://ntfy.example/bv" } },
  ],
});

let onServer: NotifyConfig = stored();
let loadFails = false;
const saved: NotifyConfig[] = [];
let answer: NotifyAnswer = { ok: true };
const tested: NotifyChannel[] = [];
let testAnswer: NotifyAnswer = { ok: true };

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getNotify: () => (loadFails ? Promise.reject(new Error("offline")) : Promise.resolve({ ok: true, notify: onServer })),
    setNotify: (c: NotifyConfig) => {
      saved.push(structuredClone(c));
      return Promise.resolve(answer);
    },
    testNotify: (c: NotifyChannel) => {
      tested.push(structuredClone(c));
      return Promise.resolve(testAnswer);
    },
  };
});

const { NotifyCard } = await import("./NotifyCard");

function Harness() {
  const { t } = useT();
  return <NotifyCard t={t} platformKind="unraid" sections={["container", "zfs"]} />;
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

function channel(id: string): HTMLElement {
  return document.querySelector(`[data-channel="${id}"]`) as HTMLElement;
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  onServer = stored();
  loadFails = false;
  answer = { ok: true };
  testAnswer = { ok: true };
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
  saved.length = 0;
  tested.length = 0;
});

describe("the channel list", () => {
  it("shows each channel by its name or its kind, with its switch and its own policy", async () => {
    show();
    const gotify = await screen.findByRole("switch", { name: en["notify.gotify"] });
    expect(gotify.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("switch", { name: "Phone" }).getAttribute("aria-checked")).toBe("false");
    const always = within(channel("gotify")).getByRole("tab", { name: en["notify.onAlways"] });
    expect(always.getAttribute("aria-selected")).toBe("true");
    const failure = within(channel("phone")).getByRole("tab", { name: en["notify.onFailure"] });
    expect(failure.getAttribute("aria-selected")).toBe("true");
  });

  it("shows a stored token as kept without its value", async () => {
    show();
    const token = (await within(await waitChannel("gotify")).findByLabelText(en["notify.gotifyToken"])) as HTMLInputElement;
    expect(token.value).toBe("");
    expect(within(channel("gotify")).getByText(en["notify.secretKept"])).toBeTruthy();
  });

  it("saves a typed field with the whole list and sends a stored secret back unchanged", async () => {
    show();
    fireEvent.change(within(await waitChannel("phone")).getByLabelText(en["notify.ntfyUrl"]), {
      target: { value: "https://ntfy.example/other" },
    });
    await pause(1000);
    expect(saved).toHaveLength(1);
    expect(saved[0].channels[1].ntfy?.url).toBe("https://ntfy.example/other");
    expect(saved[0].channels[0].gotify?.token).toBe(REDACTED);
  });

  it("saves a switch at once", async () => {
    show();
    fireEvent.click(await screen.findByRole("switch", { name: "Phone" }));
    await pause(50);
    expect(saved.at(-1)?.channels[1].enabled).toBe(true);
  });

  it("saves a picked policy for that channel alone", async () => {
    show();
    fireEvent.click(within(await waitChannel("phone")).getByRole("tab", { name: en["notify.onNever"] }));
    await pause(50);
    expect(saved.at(-1)?.channels.map((c) => c.on)).toEqual(["always", "never"]);
  });

  it("holds a new server address until the token is typed again, then sends both", async () => {
    show();
    const gotify = await waitChannel("gotify");
    fireEvent.change(within(gotify).getByLabelText(en["notify.gotifyUrl"]), { target: { value: "https://other.example" } });
    await pause(1000);
    expect(saved.at(-1)?.channels[0].gotify?.url ?? "https://push.example").toBe("https://push.example");
    expect(within(gotify).getByText(en["notify.secretAgain"])).toBeTruthy();
    fireEvent.change(within(gotify).getByLabelText(en["notify.gotifyToken"]), { target: { value: "new-token" } });
    await pause(1000);
    expect(saved.at(-1)?.channels[0].gotify).toMatchObject({ url: "https://other.example", token: "new-token" });
  });

  it("puts a field back when its save fails", async () => {
    answer = { ok: false, error: "disk full" };
    show();
    const name = within(await waitChannel("phone")).getByLabelText(en["notify.name"]) as HTMLInputElement;
    fireEvent.change(name, { target: { value: "Tablet" } });
    await pause(1000);
    expect((within(channel("phone")).getByLabelText(en["notify.name"]) as HTMLInputElement).value).toBe("Phone");
  });

  it("keeps a refused value and says under its field what is wrong", async () => {
    answer = { ok: false, code: "invalid", error: "notify: bad_url", problems: [{ index: 1, channel: "phone", field: "ntfy.url", code: "bad_url" }] };
    show();
    const url = within(await waitChannel("phone")).getByLabelText(en["notify.ntfyUrl"]) as HTMLInputElement;
    fireEvent.change(url, { target: { value: "ntfy.example" } });
    await pause(1000);
    expect(within(channel("phone")).getByText(en["notify.problem.badUrl"])).toBeTruthy();
    expect((within(channel("phone")).getByLabelText(en["notify.ntfyUrl"]) as HTMLInputElement).value).toBe("ntfy.example");
  });

  it("adds a switched-off channel of the picked kind with an id of its own", async () => {
    onServer = { ...stored(), channels: [{ id: "ntfy", kind: "ntfy", enabled: true, ntfy: { url: "https://n.example/a" } }] };
    show();
    await waitChannel("ntfy");
    fireEvent.click(screen.getByRole("button", { name: en["notify.add"] }));
    fireEvent.click(await screen.findByRole("option", { name: en["notify.kindNtfy"] }));
    await pause(50);
    expect(saved.at(-1)?.channels[1]).toMatchObject({ id: "ntfy-2", kind: "ntfy", enabled: false });
  });

  it("removes a channel", async () => {
    show();
    fireEvent.click(within(await waitChannel("phone")).getByRole("button", { name: en["notify.remove"] }));
    await pause(50);
    expect(saved.at(-1)?.channels.map((c) => c.id)).toEqual(["gotify"]);
  });

  it("tests the channel as the form shows it and says why it failed", async () => {
    testAnswer = { ok: false, code: "auth", status: 401, error: "gotify: HTTP 401" };
    show();
    const gotify = await waitChannel("gotify");
    fireEvent.click(within(gotify).getByRole("button", { name: en["notify.test"] }));
    expect(await within(gotify).findByText(en["notify.result.auth"])).toBeTruthy();
    expect(tested[0]).toMatchObject({ id: "gotify", kind: "gotify" });
  });

  it("saves nothing while the stored channels could not be read", async () => {
    loadFails = true;
    show();
    expect(await screen.findByText(en["settings.notLoadedNoSave"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["notify.add"] }));
    fireEvent.click(await screen.findByRole("option", { name: en["notify.kindNtfy"] }));
    await pause(50);
    expect(saved).toHaveLength(0);
  });
});

describe("a webhook channel", () => {
  it("saves a header by its name and keeps a stored header value", async () => {
    onServer = {
      ...stored(),
      channels: [
        {
          id: "hook",
          kind: "webhook",
          enabled: true,
          webhook: { url: "https://hooks.example/in", method: "POST", headers: { Authorization: REDACTED }, body: "" },
        },
      ],
    };
    show();
    const hook = await waitChannel("hook");
    fireEvent.click(within(hook).getByRole("button", { name: en["notify.webhookHeaderAdd"] }));
    const names = within(hook).getAllByLabelText(en["notify.webhookHeaderName"]);
    fireEvent.change(names[1], { target: { value: "X-Source" } });
    const values = within(hook).getAllByLabelText(en["notify.webhookHeaderValue"]);
    fireEvent.change(values[1], { target: { value: "bombvault" } });
    await pause(1000);
    expect(saved.at(-1)?.channels[0].webhook?.headers).toEqual({ Authorization: REDACTED, "X-Source": "bombvault" });
  });
});

describe("a Healthchecks channel", () => {
  it("leaves a section added without its own address out of what it saves", async () => {
    onServer = {
      ...stored(),
      channels: [{ id: "checks", kind: "healthchecks", enabled: true, on: "always", healthchecks: { url: "https://hc.example/all", byKey: { zfs: "https://hc.example/zfs" } } }],
    };
    show();
    const checks = await waitChannel("checks");
    expect(within(checks).getByText(en["notify.hcRuleOthers"])).toBeTruthy();
    fireEvent.click(within(checks).getByRole("button", { name: en["exceptions.add"] }));
    fireEvent.click(await screen.findByRole("option", { name: en["nav.containers"] }));
    fireEvent.change(within(checks).getByRole("textbox", { name: `${en["notify.healthchecks"]}: ${en["nav.containers"]}` }), {
      target: { value: "https://hc.example/ct" },
    });
    await pause(1000);
    expect(saved.at(-1)?.channels[0].healthchecks?.byKey).toEqual({ zfs: "https://hc.example/zfs", container: "https://hc.example/ct" });
    fireEvent.click(within(checks).getByRole("button", { name: `${en["exceptions.remove"]}: ${en["nav.zfs"]}` }));
    await pause(50);
    expect(saved.at(-1)?.channels[0].healthchecks?.byKey).toEqual({ container: "https://hc.example/ct" });
    fireEvent.change(within(checks).getByRole("textbox", { name: `${en["notify.healthchecks"]}: ${en["nav.containers"]}` }), {
      target: { value: "" },
    });
    await pause(1000);
    expect(saved.at(-1)?.channels[0].healthchecks?.byKey).toEqual({});
  });
});

async function waitChannel(id: string): Promise<HTMLElement> {
  await screen.findAllByRole("switch");
  const el = channel(id);
  expect(el, `channel ${id}`).toBeTruthy();
  return el;
}
