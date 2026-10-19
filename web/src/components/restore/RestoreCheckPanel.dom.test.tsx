// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { countText, en } from "../../lib/i18n";
import type { RestoreCheckResponse } from "../../lib/api";
import { RestoreCheckPanel } from "./RestoreCheckPanel";

const t = ((k: string, n?: number) => countText(en[k as keyof typeof en] ?? k, "en", n)) as never;

function show(result: RestoreCheckResponse) {
  return render(<RestoreCheckPanel check={{ state: { phase: "done", result } }} t={t} />);
}

const checks: RestoreCheckResponse["checks"] = [
  { id: "repository", status: "ok" },
  { id: "key", status: "ok", reason: "unencrypted" },
  { id: "snapshot", status: "ok", detail: "aaaa1111" },
  { id: "space", status: "ok", need: 2048, free: 4096 },
];

afterEach(cleanup);

function zvolPlan(counts: { changed?: number }): RestoreCheckResponse["plan"] {
  return {
    inPlace: true,
    added: 0,
    changed: counts.changed ?? 0,
    unchanged: 0,
    extra: 0,
    files: [],
    listCapped: false,
    partial: false,
    missing: false,
    definition: [],
    shared: [],
    zvols: [{ dataset: "tank/vms/testvm", size: 1073741824, change: "changed", safety: "tank/vms/testvm@bombvault-before-restore-", free: 54760833024 }],
  };
}

describe("the restore plan", () => {
  it("counts what changes and lists the files on request", () => {
    show({
      ok: true,
      ready: true,
      checks,
      plan: {
        inPlace: true,
        added: 1,
        changed: 2,
        unchanged: 5,
        extra: 1,
        files: [
          { path: "/mnt/user/appdata/plex/new.conf", change: "added" },
          { path: "/mnt/user/appdata/plex/leftover.log", change: "extra" },
        ],
        listCapped: false,
        partial: false,
        missing: false,
        definition: [],
        shared: [],
      },
    });
    expect(screen.getByText("1 new file")).toBeTruthy();
    expect(screen.getByText("2 files replaced")).toBeTruthy();
    expect(screen.getByText("Needs 2.0 KB, 4.0 KB free.")).toBeTruthy();
    expect(screen.queryByText("/mnt/user/appdata/plex/new.conf")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: en["restoreCheck.plan.showFiles"] }));
    const files = screen.getByRole("dialog", { name: en["restoreCheck.plan.title"] });
    expect(within(files).getByText("/mnt/user/appdata/plex/new.conf")).toBeTruthy();
    expect(screen.getByText(en["restoreCheck.change.extra"])).toBeTruthy();
  });

  it("names the other containers on a shared folder and the definition changes", () => {
    show({
      ok: true,
      ready: true,
      checks,
      plan: {
        inPlace: true,
        added: 0,
        changed: 0,
        unchanged: 0,
        extra: 0,
        files: [],
        listCapped: false,
        partial: true,
        missing: false,
        definition: [
          { field: "image", change: "changed", backup: "plexinc/pms:1.40", now: "plexinc/pms:1.41" },
          { field: "env", name: "PLEX_CLAIM", change: "changed" },
        ],
        shared: [{ path: "/mnt/user/appdata/plex", containers: ["nextcloud-db", "tautulli"] }],
      },
    });
    expect(screen.getByText("/mnt/user/appdata/plex is also used by nextcloud-db, tautulli.")).toBeTruthy();
    expect(screen.getByText("backup plexinc/pms:1.40, now plexinc/pms:1.41")).toBeTruthy();
    expect(screen.getByText("PLEX_CLAIM")).toBeTruthy();
    expect(screen.getByText(en["restoreCheck.plan.partial"])).toBeTruthy();
  });

  it("marks a failed line and shows the server's reason", () => {
    show({
      ok: true,
      ready: false,
      checks: [
        { id: "repository", status: "ok" },
        { id: "key", status: "ok" },
        { id: "snapshot", status: "fail", detail: "snapshot bbbb2222 does not belong to this container" },
        { id: "space", status: "skip", reason: "no-snapshot" },
      ],
      plan: null,
    });
    expect(screen.getByText(en["restoreCheck.status.fail"])).toBeTruthy();
    expect(screen.getByText("snapshot bbbb2222 does not belong to this container")).toBeTruthy();
    expect(screen.getByText(en["restoreCheck.reason.noSnapshot"])).toBeTruthy();
  });

  it("ends each line with its state and puts what was found under the name of the check", () => {
    show({
      ok: true,
      ready: false,
      checks: [
        { id: "repository", status: "ok", detail: "/mnt/user/backups/bombvault/containers" },
        { id: "snapshot", status: "fail", detail: "gone" },
        { id: "space", status: "skip", reason: "no-snapshot" },
      ],
      plan: null,
    });
    const lines = screen.getAllByRole("listitem");
    expect(lines.map((line) => line.lastElementChild?.textContent)).toEqual([
      en["restoreCheck.status.ok"],
      en["restoreCheck.status.fail"],
      en["restoreCheck.status.skip"],
    ]);
    const named = screen.getByText(en["restoreCheck.line.repository"]);
    expect(named.parentElement?.nextElementSibling?.textContent).toBe("/mnt/user/backups/bombvault/containers");
  });

  it("names the missing dataset a new dataset would go under", () => {
    show({
      ok: true,
      ready: false,
      checks: [
        { id: "repository", status: "ok" },
        { id: "key", status: "ok" },
        { id: "snapshot", status: "ok" },
        { id: "space", status: "fail", reason: "parent-missing", detail: "tank/gone" },
      ],
      plan: null,
    });
    expect(screen.getByText(en["restoreCheck.reason.parentMissing"].replace("{name}", "tank/gone"))).toBeTruthy();
  });

  it("says to reinstall the app whose network is gone", () => {
    show({
      ok: true,
      ready: false,
      checks: [
        { id: "repository", status: "ok" },
        { id: "key", status: "ok" },
        { id: "snapshot", status: "ok" },
        { id: "network", status: "fail", reason: "network-gone", detail: "ix-webpage_default" },
      ],
      plan: null,
    });
    expect(
      screen.getByText(en["restoreCheck.reason.networkGone"].replace("{name}", "ix-webpage_default")),
    ).toBeTruthy();
  });

  it("says the networks could not be read and shows Docker's error", () => {
    const error = "restore: list networks: Cannot connect to the Docker daemon";
    show({
      ok: true,
      ready: false,
      checks: [
        { id: "repository", status: "ok" },
        { id: "key", status: "ok" },
        { id: "snapshot", status: "ok" },
        { id: "network", status: "fail", reason: "network-unreadable", detail: error },
      ],
      plan: null,
    });
    expect(screen.getByText(en["restoreCheck.reason.networkUnreadable"].replace("{error}", error))).toBeTruthy();
  });

  it("lists the zvol a VM restore writes whole and the snapshot it takes first", () => {
    show({
      ok: true,
      ready: true,
      checks: [...checks.slice(0, 3), { id: "space", status: "ok", need: 1073741824, free: 54760833024 }],
      plan: {
        inPlace: true,
        added: 0,
        changed: 0,
        unchanged: 0,
        extra: 0,
        files: [],
        listCapped: false,
        partial: false,
        missing: false,
        definition: [],
        shared: [],
        zvols: [
          { dataset: "tank/vms/testvm", size: 1073741824, change: "changed", safety: "tank/vms/testvm@bombvault-before-restore-", free: 54760833024 },
        ],
      },
    });
    expect(screen.getByText(en["restoreCheck.zvol.title"])).toBeTruthy();
    expect(screen.getByText("tank/vms/testvm")).toBeTruthy();
    expect(screen.getByText(en["restoreCheck.change.changed"])).toBeTruthy();
    expect(
      screen.getByText(
        `${en["restoreCheck.zvol.safety"].replace("{name}", "tank/vms/testvm@bombvault-before-restore-…")} ${en["restoreCheck.zvol.safetyStamp"]}`,
      ),
    ).toBeTruthy();
    expect(screen.queryByText(en["restoreCheck.reason.nothing"])).toBeNull();
  });

  it("shows no file counts for a VM whose disks are all zvols", () => {
    show({ ok: true, ready: true, checks, plan: zvolPlan({}) });
    expect(screen.getByText("tank/vms/testvm")).toBeTruthy();
    expect(screen.queryByText("0 new files")).toBeNull();
    expect(screen.queryByText("0 files not in the backup")).toBeNull();
  });

  it("keeps the file counts when a VM has a file disk beside its zvol", () => {
    show({ ok: true, ready: true, checks, plan: zvolPlan({ changed: 1 }) });
    expect(screen.getByText("1 file replaced")).toBeTruthy();
    expect(screen.getByText("0 new files")).toBeTruthy();
  });

  it("says the pool a zvol belongs on is missing", () => {
    show({
      ok: true,
      ready: false,
      checks: [...checks.slice(0, 3), { id: "space", status: "fail", reason: "pool-missing", detail: "tank" }],
      plan: null,
    });
    expect(screen.getByText(en["restoreCheck.reason.poolMissing"].replace("{name}", "tank"))).toBeTruthy();
  });
});

describe("a check still running", () => {
  it("lists the lines it will answer from the start", () => {
    render(<RestoreCheckPanel check={{ state: { phase: "running" } }} t={t} />);
    const pending = screen.getByTestId("restore-check-pending");
    expect(within(pending).getAllByRole("listitem").map((line) => line.textContent)).toEqual([
      en["restoreCheck.line.repository"],
      en["restoreCheck.line.key"],
      en["restoreCheck.line.snapshot"],
      en["restoreCheck.line.space"],
    ]);
  });

  it("gives way to the answers once they are in", () => {
    show({ ok: true, ready: true, checks, plan: null });
    expect(screen.queryByTestId("restore-check-pending")).toBeNull();
  });
});
