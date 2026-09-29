// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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
    expect(screen.getByText("/mnt/user/appdata/plex/new.conf")).toBeTruthy();
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
});
