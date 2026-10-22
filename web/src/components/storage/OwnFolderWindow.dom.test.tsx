// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { CloudCredSetInfo, OwnFolderStep, StorageLocation, StorageLocationSection } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

class NoopEventSource {
  onmessage: unknown = null;
  onerror: unknown = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(domain: StorageLocationSection["domain"], use: "home" | "copy", where: string): StorageLocationSection {
  return { domain, use, where, enabled: true, immutable: false, retention: KEEP, compression: "auto", limitUpload: 0, limitDownload: 0, own: [] };
}

function place(over: Partial<StorageLocation>): StorageLocation {
  return {
    id: "", object: "destination", kind: "offsite", provider: "", backend: "s3", name: "", where: "", enabled: true,
    offPremises: true, sections: [], protection: { immutable: false, testable: false }, capacity: {}, ...over,
  };
}

const LOCAL = place({
  id: "path:local", object: "path", kind: "local", backend: "local", name: "Local", where: "user/backups",
  sections: [section("containers", "home", "user/backups/containers"), section("vms", "home", "user/backups/vms")],
});
const HETZNER = place({
  id: "destination:h", name: "Hetzner S3", where: "s3:https://fsn1/bv", credsRef: "hetzner",
  sections: [section("containers", "copy", "s3:https://fsn1/bv/containers")],
});
const SETS: CloudCredSetInfo[] = [
  { id: "hetzner", name: "Hetzner S3", s3KeyId: "H3TZ", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false },
  { id: "nas", name: "NAS", s3KeyId: "", s3Region: "", restUser: "bv", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true, default: true },
];

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => SETS,
}));
vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  listStorageLocations: vi.fn(),
  previewOwnFolders: vi.fn(),
  moveOwnFolders: vi.fn(),
}));

vi.mock("./SectionPathWindow", () => ({
  SectionPathWindow: ({ domain }: { domain: string }) => <div data-testid="path-window">{domain}</div>,
}));

const api = await import("../../lib/api");
const previewOwnFolders = vi.mocked(api.previewOwnFolders);
const moveOwnFolders = vi.mocked(api.moveOwnFolders);
const { OwnFolderWindow } = await import("./OwnFolderWindow");

const STEP: OwnFolderStep = {
  domain: "containers", from: "user/backups/containers", to: "s3:https://fsn1/bv/containers", same: false,
  copyAt: "t-copy", copies: ["Wasabi"], backups: 415, pending: 3, key: "own:containers",
};

let closed = 0;

async function open(domains: StorageLocationSection["domain"][], target?: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <OwnFolderWindow domains={domains} target={target} onClose={() => closed++} />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {});
}

async function settle() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(500);
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  closed = 0;
  vi.mocked(api.listStorageLocations).mockResolvedValue({ ok: true, locations: [LOCAL, HETZNER] });
  previewOwnFolders.mockResolvedValue({ ok: true, steps: [STEP] });
  moveOwnFolders.mockResolvedValue({ ok: true, steps: [STEP] });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.useRealTimers();
});

it("offers each storage location with the sign-in it brings", async () => {
  await open(["containers"]);

  const hetzner = screen.getByRole("radio", { name: "Hetzner S3" });
  expect(hetzner.textContent).toContain("Signs in with Hetzner S3");
  expect(hetzner.textContent).toContain("Holds a copy of Containers");
  expect(hetzner.textContent).toContain("s3:https://fsn1/bv/containers");
  const local = screen.getByRole("radio", { name: "Local" });
  expect(local.textContent).toContain("Now");
  expect(local.textContent).toContain("On this host, no sign-in needed");
});

it("says before the switch that the copy there becomes the own folder", async () => {
  await open(["containers"]);
  await act(async () => {
    fireEvent.click(screen.getByRole("radio", { name: "Hetzner S3" }));
  });
  await settle();

  expect(previewOwnFolders).toHaveBeenLastCalledWith({ domains: ["containers"], copyOver: true, location: "destination:h" });
  const what = screen.getByRole("list");
  expect(what.textContent).toContain("Hetzner S3 already holds a copy of Containers. That copy becomes the own folder, and copying to Hetzner S3 ends.");
  expect(what.textContent).toContain("The 3 backups it lacks are copied over first.");
  expect(what.textContent).toContain("Copies keep going to Wasabi.");
  expect(what.textContent).toContain("Protection, bandwidth limits and the growth alarm come from Hetzner S3.");

  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: "Copy the backups already made" }));
  });
  expect(screen.getByRole("list").textContent).toContain("The 3 newest backups stay in Local.");
});

it("moves several sections in one step", async () => {
  await open(["containers"], "destination:h");
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: "VMs" }));
  });
  await settle();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));
  });

  expect(moveOwnFolders).toHaveBeenCalledWith({ domains: ["containers", "vms"], copyOver: true, location: "destination:h" });
  expect(closed).toBe(1);
});

it("signs a typed remote address in with a saved sign-in", async () => {
  await open(["vms"]);
  await act(async () => {
    fireEvent.click(screen.getByRole("radio", { name: "Another place" }));
  });
  fireEvent.change(screen.getByLabelText("Address"), { target: { value: "rest:http://nas:8000/bv" } });
  await settle();

  const signIn = screen.getByRole("combobox", { name: "Sign-in" });
  expect(signIn.textContent).toContain("NAS (default)");
  expect(previewOwnFolders).toHaveBeenLastCalledWith({ domains: ["vms"], copyOver: true, address: "rest:http://nas:8000/bv", credsRef: "nas" });
});

it("opens the safety settings of a typed address once the section points there", async () => {
  const moved: OwnFolderStep = { ...STEP, domain: "vms", to: "rest:http://nas:8000/bv/vms", copyAt: undefined, copies: [], pending: 0 };
  previewOwnFolders.mockResolvedValue({ ok: true, steps: [moved] });
  moveOwnFolders.mockResolvedValue({ ok: true, steps: [moved] });
  await open(["vms"]);
  await act(async () => {
    fireEvent.click(screen.getByRole("radio", { name: "Another place" }));
  });
  fireEvent.change(screen.getByLabelText("Address"), { target: { value: "rest:http://nas:8000/bv" } });
  await settle();
  expect(within(screen.getByRole("dialog")).queryByRole("button", { name: "Open" })).toBeNull();

  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Switch" }));
  });

  expect(screen.getByTestId("path-window").textContent).toBe("vms");
  expect(closed).toBe(0);
});

it("says when the copy there keeps more and when the own folder goes on again", async () => {
  previewOwnFolders.mockResolvedValue({ ok: true, steps: [{ ...STEP, widerKeep: true, switchesOn: true }] });
  await open(["containers"], "destination:h");
  await settle();

  const what = screen.getByRole("list").textContent;
  expect(what).toContain("That copy keeps some backups longer than the keep rules of Containers. From the switch on, Containers keeps");
  expect(what).toContain("The switch turns the own folder back on.");
});

it("offers the switch without the copy when the old own folder does not answer", async () => {
  previewOwnFolders.mockImplementation(async (req) =>
    req.copyOver ? { ok: false, code: "own-folder-unreadable", error: "unreadable" } : { ok: true, steps: [STEP] }
  );
  await open(["containers"], "destination:h");
  await settle();

  const alert = screen.getByRole("alert");
  expect(alert.textContent).toContain("The backups in the old own folder could not be read");
  expect(alert.textContent).toContain('Switch off "Copy the backups already made"');
});

it("says under What happens why the switch cannot go ahead and that it can without the copy", async () => {
  previewOwnFolders.mockImplementation(async (req) =>
    req.copyOver ? { ok: false, code: "own-folder-unreadable", error: "backup path is not mounted yet" } : { ok: true, steps: [STEP] }
  );
  await open(["containers"], "destination:h");
  await settle();

  const alert = screen.getByRole("alert");
  expect(alert.textContent).toContain("The own folder cannot move like this.");
  expect(alert.textContent).toContain("The backups in the old own folder could not be read");
  expect(alert.textContent).toContain('Switch off "Copy the backups already made"');
  expect((screen.getByRole("button", { name: "Switch" }) as HTMLButtonElement).disabled).toBe(true);

  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: "Copy the backups already made" }));
  });
  await settle();

  expect(previewOwnFolders).toHaveBeenLastCalledWith({ domains: ["containers"], copyOver: false, location: "destination:h" });
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getByRole("list").textContent).toContain("The 3 newest backups stay in Local.");
  expect((screen.getByRole("button", { name: "Switch" }) as HTMLButtonElement).disabled).toBe(false);
});

it("says the old backups cannot be read and that the switch can go ahead without them", async () => {
  previewOwnFolders.mockImplementation(async (req) =>
    req.copyOver ? { ok: false, code: "own-folder-unreadable", error: "backup path is not mounted yet" } : { ok: true, steps: [STEP] }
  );
  await open(["containers"], "destination:h");
  await settle();

  const alert = screen.getByRole("alert");
  expect(alert.textContent).toContain("The backups in the old own folder could not be read");
  expect(alert.textContent).toContain('Switch off "Copy the backups already made"');
  expect((screen.getByRole("button", { name: "Switch" }) as HTMLButtonElement).disabled).toBe(true);
});
