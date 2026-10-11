// @vitest-environment jsdom
// The card serves every user of the host SSH link. ZFS backups and Unraid
// notifications need only SSH, so a host without libvirt must read as
// connected rather than failed, with the libvirt gap named for VM backups.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { OkEnvelope } from "../../lib/api";
import { IconClose } from "../../components/navGlyphs";

let testAnswer: OkEnvelope & { libvirt?: boolean; libvirtError?: string } = { ok: true, libvirt: true };

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower", publicKey: "ssh-ed25519 AAAA bombvault" }),
    testVMSSH: () => Promise.resolve(testAnswer),
  };
});

const { VMSSHCard } = await import("./VMSSHCard");

function Harness() {
  const { t } = useT();
  return <VMSSHCard t={t} />;
}

async function runTest() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["vm.ssh.test"] }));
  });
}

afterEach(() => {
  cleanup();
  testAnswer = { ok: true, libvirt: true };
});

it("is named after the host SSH link, not after VMs", async () => {
  await runTest();
  // The heading's accessible name also carries the hint bubble's text.
  expect(screen.getByRole("heading", { name: new RegExp(`^${en["vm.ssh.title"]}`) })).toBeTruthy();
  expect(en["vm.ssh.title"]).not.toMatch(/VM/);
});

it("reads as connected when SSH works and only libvirt is missing", async () => {
  testAnswer = { ok: true, libvirt: false, libvirtError: "virsh: command not found" };
  await runTest();
  expect(await screen.findByText(en["vm.ssh.testNoLibvirt"])).toBeTruthy();
  expect(screen.getByRole("button", { name: en["verdict.connected"] })).toBeTruthy();
});

it("turns the button green with Connected and a check when the link works", async () => {
  await runTest();
  const button = await screen.findByRole("button", { name: en["verdict.connected"] });
  expect(button.className).toContain("bg-statusOkSolid");
  expect(button.querySelector(".glim-check-draw")).not.toBeNull();
  expect(button.className).not.toContain("glim-shake");
  expect(screen.queryByText(en["vm.ssh.testNoLibvirt"])).toBeNull();
});

it("turns the button red with Not connected and a cross, shakes it and gives the reason above", async () => {
  testAnswer = { ok: false, error: "Permission denied (publickey)" };
  await runTest();
  const button = await screen.findByRole("button", { name: en["verdict.notConnected"] });
  expect(button.className).toContain("bg-statusFailSolid");
  expect(button.className).toContain("glim-shake");
  expect(button.querySelector(".glim-check-draw")).toBeNull();
  const { container, unmount } = render(<IconClose />);
  const cross = container.querySelector("svg")?.innerHTML;
  unmount();
  expect([...button.querySelectorAll("svg")].map((svg) => svg.innerHTML)).toContain(cross);
  const reason = screen.getByText("Permission denied (publickey)");
  expect(reason.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

it("shows the host and the public key as rows, the key with a copy button", async () => {
  await runTest();
  const host = screen.getByText(en["vm.ssh.host"]).closest("[data-search-row]")!;
  expect(host.textContent).toContain("tower");
  const key = screen.getByText(en["vm.ssh.keyLabel"]).closest("[data-search-row]") as HTMLElement;
  expect(key.querySelector("code")?.textContent).toBe("ssh-ed25519 AAAA bombvault");
  expect(within(key).getByRole("button", { name: en["common.copy"] })).toBeTruthy();
});

it("walks through the setup in four steps, copying the command first and testing last", async () => {
  await runTest();
  const steps = within(screen.getByRole("list")).getAllByRole("listitem");
  expect(steps).toHaveLength(4);
  expect(within(steps[0]).getByRole("button", { name: en["vm.ssh.copyCmd"] })).toBeTruthy();
  expect(steps[1].textContent).toBe(en["vm.ssh.stepTerminal"]);
  expect(steps[2].textContent).toBe(en["vm.ssh.stepPaste"]);
  expect(within(steps[3]).getByRole("button", { name: en["verdict.connected"] })).toBeTruthy();
  // The command goes to the clipboard, so its text stays off the page.
  expect(document.querySelector("pre")).toBeNull();
});

it("opens the full guide from a button at the end of the setup", async () => {
  await runTest();
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  fireEvent.click(screen.getByRole("button", { name: en["vm.ssh.fullGuide"] }));
  expect(open.mock.calls[0][0]).toContain("vm-backup-ssh-setup.md");
  open.mockRestore();
});
