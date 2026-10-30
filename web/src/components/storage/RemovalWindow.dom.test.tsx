// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { LocationRemoval, StorageLocation } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  previewLocationRemoval: vi.fn(),
  previewLocationErasure: vi.fn(),
  removeLocation: vi.fn(),
}));

const api = await import("../../lib/api");
const { RemovalWindow } = await import("./RemovalWindow");

const VAULT: StorageLocation = {
  id: "destination:v", object: "destination", kind: "offsite", provider: "drive", backend: "rclone", name: "Vault", where: "rclone:vault:bv",
  enabled: true, offPremises: true, sections: [], protection: { immutable: false, testable: false }, capacity: {},
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("keeps a storage location that holds an own folder and says where to move it", async () => {
  const removal: LocationRemoval = {
    sections: [
      {
        domain: "containers", use: "home", targetId: "", repo: "rclone:vault:bv/containers", folder: "containers",
        primary: false, appendOnly: false, entries: 0, backups: 0, listedAt: 0, lastCopyAt: 0,
      },
    ],
  };
  vi.mocked(api.previewLocationRemoval).mockResolvedValue({ ok: true, removal });

  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RemovalWindow location={VAULT} onClose={() => {}} onRemoved={() => {}} />
        </ToastProvider>
      </I18nProvider>
    );
  });

  const alert = screen.getByRole("alert");
  expect(alert.textContent).toContain("The own folder of a section lies here.");
  expect(alert.textContent).toContain("Containers");
  expect(screen.queryByRole("radiogroup")).toBeNull();
  expect((screen.getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(true);
});

it("names the busy section when a removal has to wait for it", async () => {
  const removal: LocationRemoval = {
    sections: [
      {
        domain: "vms", use: "copy", targetId: "t1", repo: "rclone:vault:bv/vms",
        primary: false, appendOnly: false, entries: 1, backups: 2, listedAt: 0, lastCopyAt: 0,
      },
    ],
  };
  vi.mocked(api.previewLocationRemoval).mockResolvedValue({ ok: true, removal });
  vi.mocked(api.removeLocation).mockResolvedValue({ ok: false, code: "domain-busy", error: "a backup is currently running", domain: "vms" });

  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RemovalWindow location={VAULT} onClose={() => {}} onRemoved={() => {}} />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    screen.getByRole("button", { name: "Remove storage location" }).click();
  });

  expect(
    screen.getByText("Nothing was removed, because another task is using VMs right now, such as a backup, a copy or a check. Try again when it has finished.")
  ).toBeTruthy();
});
