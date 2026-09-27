// @vitest-environment jsdom
// The card serves every user of the host SSH link. ZFS backups and Unraid
// notifications need only SSH, so a host without libvirt must read as
// connected rather than failed, with the libvirt gap named for VM backups.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { OkEnvelope } from "../../lib/api";

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
  expect(screen.queryByText(en["vm.ssh.testFail"])).toBeNull();
});

it("says libvirt is reachable when it is", async () => {
  await runTest();
  expect(await screen.findByText(en["vm.ssh.testOk"])).toBeTruthy();
});

it("fails when the SSH link itself fails", async () => {
  testAnswer = { ok: false, error: "Permission denied (publickey)" };
  await runTest();
  expect(await screen.findAllByText(en["vm.ssh.testFail"])).not.toHaveLength(0);
});
