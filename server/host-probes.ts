import { execFile } from "node:child_process";
import { access, constants } from "node:fs/promises";
import Docker from "dockerode";
import { getConfig } from "../lib/config";
import { version as resticVersion } from "../lib/restic";
import type { Probe, ProbeResult } from "./spike-report";

// Real host probes. Each must DEGRADE GRACEFULLY: any failure becomes a
// { ok: false, error } ProbeResult, never a throw. None of these are asserted in
// CI (no Docker/KVM/Unraid on runners); the user runs them on the real box.
function runCli(name: string, bin: string, args: string[]): Probe {
  return () =>
    new Promise<ProbeResult>((resolve) => {
      execFile(bin, args, { timeout: 10_000 }, (err, stdout, stderr) => {
        if (err) {
          resolve({ name, ok: false, error: (stderr || err.message).trim() });
        } else {
          resolve({ name, ok: true, detail: (stdout || stderr).trim().split("\n")[0] });
        }
      });
    });
}

export const probeRestic: Probe = async () => {
  try {
    return { name: "restic version", ok: true, detail: await resticVersion() };
  } catch (err) {
    return { name: "restic version", ok: false, error: (err as Error).message };
  }
};

export const probeDocker: Probe = async () => {
  try {
    const docker = new Docker({ socketPath: "/var/run/docker.sock" });
    const containers = await docker.listContainers({ all: true });
    return { name: "docker socket", ok: true, detail: `${containers.length} containers visible` };
  } catch (err) {
    return { name: "docker socket", ok: false, error: (err as Error).message };
  }
};

export const probeVirsh: Probe = runCli("libvirt (virsh)", "virsh", [
  "-c",
  "qemu:///system",
  "list",
  "--all",
]);

export const probeQemuImg: Probe = runCli("qemu-img", "qemu-img", ["--version"]);
export const probeRclone: Probe = runCli("rclone", "rclone", ["version"]);

export const probeAppdata: Probe = async () => {
  const dir = getConfig().APPDATA_DIR;
  try {
    await access(dir, constants.R_OK);
    return { name: "appdata readable", ok: true, detail: dir };
  } catch (err) {
    return { name: "appdata readable", ok: false, error: `${dir}: ${(err as Error).message}` };
  }
};

// The default probe set, in the order the spike reports them.
export const DEFAULT_PROBES: Probe[] = [
  probeRestic,
  probeDocker,
  probeVirsh,
  probeQemuImg,
  probeRclone,
  probeAppdata,
];
