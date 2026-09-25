// @vitest-environment jsdom
// A window's buttons are one row at its bottom, aligned to the end of the line,
// with the action that goes ahead last (GlimStone rule 15). Each window is
// rendered, because where the row lands is a fact about the finished tree, and
// the source is read so that no window is left out.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { I18nProvider, countText, en, type TranslationKey, type useT } from "../lib/i18n";
import type { FileSetView, FleetPeer } from "../lib/api";
import { WindowActions } from "./WindowActions";

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getDirectRepo: async () => ({ ok: true, repo: null, suggestion: null }),
  listRuns: async () => ({ ok: true, runs: [] }),
  ackRuns: async () => ({ ok: true }),
  browse: async () => ({ ok: true, dirs: [{ name: "appdata", path: "user/appdata" }] }),
  listRepos: async () => ({ ok: true, repos: [] }),
  getCloudCredSets: async () => ({ ok: true, sets: [] }),
}));

const { ConfirmDialog } = await import("./ConfirmDialog");
const { DirectRepoDialog } = await import("./placement/DirectRepoDialog");
const { useConfirm } = await import("../lib/useConfirm");
const { CryptoDonateDialog } = await import("./CryptoDonateDialog");
const { WhatsNewDialog } = await import("./WhatsNewDialog");
const { ErrorDetailPanel } = await import("./ErrorDetailPanel");
const { FolderBrowser } = await import("./FolderBrowser");
const { FileSetDialog } = await import("../pages/Files");
const { PullDialog } = await import("../pages/Pull");
const { ReceiverDialog } = await import("../pages/Receiver");
const { FleetDialog, ProposeMeshDialog } = await import("../pages/Fleet");

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

/** The class list of the shared row, read off the component itself. */
function sharedRow(): string {
  const { container } = render(<WindowActions>{null}</WindowActions>, { container: document.createElement("div") });
  return (container.firstElementChild as HTMLElement).className;
}

const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as ReturnType<typeof useT>["t"];

const documents: FileSetView = {
  id: "set1",
  name: "Documents",
  path: "documents",
  excludes: [],
  enabled: true,
  lastBackup: 0,
  pathExists: true,
};

const tower: FleetPeer = {
  id: "p1",
  name: "tower",
  url: "https://192.168.1.50:3443",
  enabled: true,
  lastPollAt: 0,
  lastPollOk: null,
  lastPollError: "",
  lastPollInstanceName: "",
  lastPollVersion: "",
  lastPollDomains: [],
  createdAt: 0,
  sortOrder: 0,
  hasToken: true,
};

function shown(node: ReactNode) {
  render(<I18nProvider>{node}</I18nProvider>);
}

/** Opens the shared confirmation the way a restore, a delete or a prune does. */
function AskFirst() {
  const { confirm, confirmDialog } = useConfirm();
  return (
    <>
      <button type="button" onClick={() => void confirm("Restore over the running container?", { confirmLabel: "Restore" })}>
        ask
      </button>
      {confirmDialog}
    </>
  );
}

type Shown = {
  /** The file that renders the window, as the source scan names it. */
  file: string;
  name: string;
  open: () => void | Promise<void>;
  /** The words of the button that has to end the row. */
  last: string;
};

const WINDOWS: Shown[] = [
  {
    file: "components/ConfirmDialog.tsx",
    name: "a confirmation",
    open: () =>
      shown(
        <ConfirmDialog
          title="Confirm"
          message="Delete this backup?"
          confirmLabel="Delete"
          cancelLabel="Cancel"
          onConfirm={() => {}}
          onCancel={() => {}}
        />,
      ),
    last: "Delete",
  },
  {
    file: "lib/useConfirm.tsx",
    name: "a restore confirmation",
    open: async () => {
      shown(<AskFirst />);
      await act(async () => document.querySelector<HTMLButtonElement>("button")!.click());
    },
    last: "Restore",
  },
  {
    file: "components/placement/DirectRepoDialog.tsx",
    name: "the direct repository window",
    open: () => shown(<DirectRepoDialog target={{ id: "t1", name: "B2" }} mode="create" onDone={() => {}} onClose={() => {}} />),
    last: en["directRepo.addAndUse"],
  },
  {
    file: "components/CryptoDonateDialog.tsx",
    name: "the crypto window",
    open: () => shown(<CryptoDonateDialog onClose={() => {}} />),
    last: en["common.close"],
  },
  {
    file: "components/WhatsNewDialog.tsx",
    name: "the release notes",
    open: () => {
      vi.stubGlobal("fetch", async () => ({
        ok: true,
        json: async () => ({ ok: true, body: "## Fixed\n\n- a thing", htmlUrl: "https://example.org" }),
      }));
      shown(<WhatsNewDialog version="v9.0.0" onClose={() => {}} />);
    },
    last: en["whatsnew.close"],
  },
  {
    file: "components/ErrorDetailPanel.tsx",
    name: "the error panel",
    open: () => shown(<ErrorDetailPanel onClose={() => {}} />),
    last: en["errorPanel.resolveAll"],
  },
  {
    file: "components/FolderBrowser.tsx",
    name: "the folder picker",
    open: async () => {
      shown(<FolderBrowser label="Restore folder" value="user" hostMountRoot="/mnt" onChange={() => {}} />);
      await act(async () => screen.getByRole("button", { name: en["folder.browseTitle"] }).click());
    },
    last: en["folder.use"],
  },
  {
    file: "pages/Files.tsx",
    name: "the folder set window",
    open: () =>
      shown(
        <FileSetDialog initial={documents} presetSeed={null} hostMountRoot="/mnt/user" t={t} onClose={() => {}} onSaved={() => {}} />,
      ),
    last: en["settings.save"],
  },
  {
    file: "pages/Pull.tsx",
    name: "the pull source window",
    open: () => shown(<PullDialog initial={null} t={t} onClose={() => {}} onSaved={() => {}} />),
    last: en["settings.save"],
  },
  {
    file: "pages/Receiver.tsx",
    name: "the received repository window",
    open: () => shown(<ReceiverDialog initial={null} t={t} onClose={() => {}} onSaved={() => {}} />),
    last: en["settings.save"],
  },
  {
    file: "pages/Fleet.tsx",
    name: "the fleet peer window",
    open: () => shown(<FleetDialog initial={null} t={t} onClose={() => {}} onSaved={() => {}} />),
    last: en["settings.save"],
  },
  {
    file: "pages/Fleet.tsx",
    name: "the mesh proposal window",
    open: () => shown(<ProposeMeshDialog peer={tower} t={t} onClose={() => {}} />),
    last: en["fleet.mesh.send"],
  },
];

function tsxFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) return tsxFiles(full);
    return /\.tsx$/.test(entry) && !entry.includes(".test.") ? [full] : [];
  });
}

/** Every file that renders a modal window, with how many it renders. */
function windowFiles(): Map<string, number> {
  const found = new Map<string, number>();
  for (const file of tsxFiles(SRC)) {
    const count = readFileSync(file, "utf8").split('aria-modal="true"').length - 1;
    if (count > 0) found.set(relative(SRC, file).replace(/\\/g, "/"), count);
  }
  return found;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("every window", () => {
  it("is rendered below", () => {
    const missing: string[] = [];
    for (const [file, count] of windowFiles()) {
      const rendered = WINDOWS.filter((w) => w.file === file).length;
      if (rendered < count) missing.push(`${file}: ${count} windows, ${rendered} rendered here`);
    }
    expect(missing, "Add each window to WINDOWS.").toEqual([]);
  });

  it("builds its row from WindowActions", () => {
    const own = [...windowFiles().keys()].filter(
      (file) => !readFileSync(join(SRC, file), "utf8").includes("<WindowActions"),
    );
    expect(own, "These windows lay out their own button row.").toEqual([]);
  });
});

describe.each(WINDOWS)("$name", (w) => {
  it("ends in one button row, aligned to the end, with the action that goes ahead last", async () => {
    await w.open();
    await act(async () => {});
    const windows = document.querySelectorAll<HTMLElement>('[role="dialog"][aria-modal="true"]');
    expect(windows).toHaveLength(1);
    const dialog = windows[0];

    const row = dialog.lastElementChild as HTMLElement;
    expect(row.className).toMatch(/\bjustify-end\b/);
    const rows = [...dialog.querySelectorAll(".justify-end")].filter((el) => el.querySelector("button"));
    expect(rows, "a second button row").toEqual([row]);

    const buttons = [...row.querySelectorAll("button, a")];
    const last = buttons[buttons.length - 1];
    expect(last.textContent).toContain(w.last);
    expect(row.lastElementChild === last || row.lastElementChild!.contains(last)).toBe(true);

    // Long content scrolls between the title and the row, never the row away.
    for (let el = row.parentElement; el && el !== dialog.parentElement; el = el.parentElement) {
      expect(el.className, "the row sits inside a scrolling box").not.toMatch(/\boverflow-(?:y-)?auto\b/);
    }

    const header = dialog.firstElementChild!;
    if (header.querySelector("h2")) expect(header.querySelectorAll("button"), "a button in the title row").toHaveLength(0);

    // The shared row, also where a part of the window renders it.
    expect(row.className, "the row is WindowActions").toBe(sharedRow());
  });
});

describe("the folder picker inside the folder set window", () => {
  // In place the picker has no window of its own, so its pair ends the panel
  // in a row that wraps and spaces like WindowActions, without the window's
  // padding.
  it("ends its panel in a row laid out like WindowActions", async () => {
    shown(<FileSetDialog initial={documents} presetSeed={null} hostMountRoot="/mnt/user" t={t} onClose={() => {}} onSaved={() => {}} />);
    await act(async () => screen.getByRole("button", { name: en["folder.browseTitle"] }).click());

    const use = screen.getByRole("button", { name: en["folder.use"] });
    const row = use.closest<HTMLElement>(".justify-end")!;
    const close = within(row).getByRole("button", { name: en["common.close"] });
    expect(close.compareDocumentPosition(use) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(row.parentElement!.lastElementChild).toBe(row);

    const layout = (cls: string) => cls.split(/\s+/).filter((c) => !/^(?:p[xy]?|shrink)-/.test(c)).sort();
    expect(layout(row.className)).toEqual(layout(sharedRow()));
  });
});
