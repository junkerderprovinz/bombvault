// @vitest-environment jsdom
// A failed run of a ZFS entry says why in the reader's language, and a replica
// whose target holds newer snapshots names the one to roll it back to.
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Run, ZFSDatasetView } from "../../lib/api";
import { en } from "../../lib/i18n";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  zfsRunMembers: () => Promise.resolve({ ok: true, members: [] }),
}));

const { ZFSRunDetailView } = await import("./ZFSDatasetRow");

const t = (key: string) => en[key as keyof typeof en];

afterEach(cleanup);

describe("the run history of a ZFS entry", () => {
  it("names the snapshot to roll the target back to after the translated reason", async () => {
    const snapshot = "tank/bombvault-replica/bottich/cache/appdata/plex@bombvault-replica-20261024115616";
    const run = {
      id: "r1",
      kind: "replica",
      status: "failed",
      error: `zfs replica: cache/appdata: 1 of 2 datasets failed: cache/appdata/plex (${snapshot}) [target-changed]`,
    } as Run;
    render(<ZFSRunDetailView run={run} item={{ members: [] } as unknown as ZFSDatasetView} t={t as never} />);
    expect(await screen.findByText(en["zfs.code.target-changed"], { exact: false })).toBeTruthy();
    const name = screen.getByText(snapshot);
    expect(name.tagName).toBe("BDI");
    expect(name.getAttribute("dir")).toBe("ltr");
  });
});
