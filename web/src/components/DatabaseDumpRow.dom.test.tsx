// @vitest-environment jsdom
// The card's dump row is where someone decides whether a database is dumped at
// all, so what it says about the container's data has to match what the backend
// reported: one sentence per coverage, never two that contradict each other.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import type { Container } from "../lib/api";

const setDbDumpOff = vi.fn(() => Promise.resolve({ ok: true }));
const setDbDumpEngine = vi.fn(() => Promise.resolve({ ok: true }));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    setDbDumpOff: (...a: unknown[]) => setDbDumpOff(...(a as [])),
    setDbDumpEngine: (...a: unknown[]) => setDbDumpEngine(...(a as [])),
  };
});

const push = vi.fn();
vi.mock("../lib/toast", () => ({ useToast: () => ({ push }) }));

const { DatabaseDumpRow } = await import("./DatabaseDumpRow");

const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;

function makeContainer(over: Partial<Container> = {}): Container {
  return {
    name: "immich_postgres",
    image: "postgres:16",
    state: "running",
    status: "Up 3 hours",
    ip: "",
    installed: true,
    includeInSchedule: true,
    lastBackup: null,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    dbEngine: "postgres",
    dbSuggestedEngine: "",
    dbTier: "curated",
    dbDumpOff: false,
    dbDumpEngine: "",
    dbDumpLabelOff: false,
    dbDumpsGlobalOff: false,
    dbDataCoverage: "stopped",
    dbDumpHookOverlap: false,
    ...over,
  };
}

function renderRow(over: Partial<Container> = {}) {
  return render(
    <I18nProvider>
      <AdvancedProvider>
        <DatabaseDumpRow container={makeContainer(over)} t={t} />
      </AdvancedProvider>
    </I18nProvider>
  );
}

function toggle(): HTMLElement {
  return screen.getByRole("switch", { name: en["dbdump.toggle"] });
}

/** The engine the picker shows; the other labels only size the trigger. */
function shownEngine(): string {
  const picker = screen.getByRole("combobox", { name: en["dbdump.engineLabel"] });
  return picker.querySelector('span[aria-hidden="false"]')?.textContent ?? "";
}

/** The hints and the coverage explanation sit in (i) bubbles, which carry their
 *  text as the accessible name. */
function hints(): string[] {
  return Array.from(document.querySelectorAll("[aria-label]"))
    .map((el) => el.getAttribute("aria-label") ?? "")
    .filter((label) => label !== en["dbdump.toggle"]);
}

function filled(key: keyof typeof en, params: Record<string, string>): string {
  let text: string = en[key];
  for (const [token, value] of Object.entries(params)) text = text.replace(`{${token}}`, value);
  return text;
}

beforeEach(() => {
  setDbDumpOff.mockClear();
  setDbDumpEngine.mockClear();
  push.mockClear();
  localStorage.clear();
});
afterEach(cleanup);

describe("DatabaseDumpRow", () => {
  it("stays out of the way of a container that is not a database", () => {
    const { container } = renderRow({ dbTier: "", dbEngine: "" });
    expect(container.innerHTML).toBe("");
  });

  it("switches the dump off for a recognised database", async () => {
    renderRow();
    expect((toggle() as HTMLInputElement).getAttribute("aria-checked")).toBe("true");
    await act(async () => {
      fireEvent.click(toggle());
    });
    await waitFor(() => expect(setDbDumpOff).toHaveBeenCalledWith("immich_postgres", true));
  });

  it("names the engine in the hint and says the dump was recognised", () => {
    renderRow();
    expect(hints()).toContain(filled("dbdump.toggleHint", { engine: "PostgreSQL" }));
  });

  it("says the label switched the dump on when that is what happened", () => {
    renderRow({ dbTier: "label", dbEngine: "mariadb" });
    expect(hints()).toContain(filled("dbdump.toggleHintLabel", { engine: "MariaDB" }));
    expect(hints()).not.toContain(filled("dbdump.toggleHint", { engine: "MariaDB" }));
  });

  it("replaces the hint where the dump is the only consistent copy, rather than adding to it", () => {
    for (const coverage of ["live", "none"] as const) {
      cleanup();
      renderRow({ dbDataCoverage: coverage });
      expect(hints()).toContain(filled("dbdump.toggleHintOnlyCopy", { engine: "PostgreSQL" }));
      expect(hints()).not.toContain(filled("dbdump.toggleHint", { engine: "PostgreSQL" }));
    }
  });

  it("claims no coverage it does not know", () => {
    renderRow({ dbDataCoverage: "unknown" });
    expect(hints()).toContain(filled("dbdump.toggleHintUnknown", { engine: "PostgreSQL" }));
    expect(hints()).not.toContain(filled("dbdump.toggleHint", { engine: "PostgreSQL" }));
    expect(hints()).not.toContain(filled("dbdump.toggleHintOnlyCopy", { engine: "PostgreSQL" }));
  });

  it("shows the line that says what the files backup of this database is worth", () => {
    for (const [coverage, key] of [
      ["stopped", "dbdump.coverageStopped"],
      ["live", "dbdump.coverageLive"],
      ["none", "dbdump.coverageNone"],
      ["unknown", "dbdump.coverageUnknown"],
    ] as const) {
      cleanup();
      renderRow({ dbDataCoverage: coverage });
      expect(screen.getByText(en[key])).toBeTruthy();
      expect(hints()).toContain(en["dbdump.coverageHint"]);
    }
  });

  it("warns about a coverage that needs the dump, and stays quiet about one that does not", () => {
    renderRow({ dbDataCoverage: "stopped" });
    expect(screen.getByText(en["dbdump.coverageStopped"]).className).not.toContain("statusWarn");
    cleanup();
    renderRow({ dbDataCoverage: "live" });
    expect(screen.getByText(en["dbdump.coverageLive"]).className).toContain("statusWarn");
  });

  it("asks before switching off a dump that is the only copy, in the words of that case", async () => {
    for (const [coverage, key] of [
      ["live", "dbdump.offConfirmLive"],
      ["none", "dbdump.offConfirmNone"],
      ["unknown", "dbdump.offConfirmUnknown"],
    ] as const) {
      cleanup();
      setDbDumpOff.mockClear();
      renderRow({ dbDataCoverage: coverage });
      await act(async () => {
        fireEvent.click(toggle());
      });
      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByText(en[key])).toBeTruthy();
      for (const other of ["dbdump.offConfirmLive", "dbdump.offConfirmNone", "dbdump.offConfirmUnknown"] as const) {
        if (other !== key) expect(within(dialog).queryByText(en[other])).toBeNull();
      }
      expect(setDbDumpOff).not.toHaveBeenCalled();
    }
  });

  it("switches off once the question is answered, and not when it is declined", async () => {
    renderRow({ dbDataCoverage: "live" });
    await act(async () => {
      fireEvent.click(toggle());
    });
    const dialog = await screen.findByRole("dialog");
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(setDbDumpOff).not.toHaveBeenCalled();
    expect((toggle() as HTMLElement).getAttribute("aria-checked")).toBe("true");
  });

  it("asks nothing when the files backup already holds a consistent copy", async () => {
    renderRow({ dbDataCoverage: "stopped" });
    await act(async () => {
      fireEvent.click(toggle());
    });
    await waitFor(() => expect(setDbDumpOff).toHaveBeenCalledWith("immich_postgres", true));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("offers a lookalike the engine it guessed, and stores it when the dump is switched on", async () => {
    renderRow({ dbTier: "lookalike", dbEngine: "", dbSuggestedEngine: "mysql", dbDumpEngine: "" });
    expect(toggle().getAttribute("aria-checked")).toBe("false");
    expect(hints()).toContain(filled("dbdump.lookalikeHint", { engine: "MySQL" }));
    await act(async () => {
      fireEvent.click(toggle());
    });
    await waitFor(() => expect(setDbDumpEngine).toHaveBeenCalledWith("immich_postgres", "mysql"));
  });

  it("clears the engine of a lookalike that is switched off again", async () => {
    renderRow({ dbTier: "lookalike", dbEngine: "", dbSuggestedEngine: "mysql", dbDumpEngine: "mysql" });
    expect(toggle().getAttribute("aria-checked")).toBe("true");
    await act(async () => {
      fireEvent.click(toggle());
    });
    await waitFor(() => expect(setDbDumpEngine).toHaveBeenCalledWith("immich_postgres", ""));
  });

  it("keeps the engine picker for the advanced view", async () => {
    renderRow({ dbTier: "lookalike", dbEngine: "", dbSuggestedEngine: "mysql", dbDumpEngine: "mysql" });
    expect(screen.queryByRole("combobox", { name: en["dbdump.engineLabel"] })).toBeNull();
    cleanup();
    localStorage.setItem("bombvault.advanced", "1");
    renderRow({ dbTier: "lookalike", dbEngine: "", dbSuggestedEngine: "mysql", dbDumpEngine: "mysql" });
    const picker = screen.getByRole("combobox", { name: en["dbdump.engineLabel"] });
    await act(async () => {
      fireEvent.click(picker);
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("option", { name: "MariaDB" }));
    });
    await waitFor(() => expect(setDbDumpEngine).toHaveBeenCalledWith("immich_postgres", "mariadb"));
  });

  it("shows a lookalike's dump as off while an earlier opt-out still stops it", () => {
    renderRow({ dbTier: "lookalike", dbEngine: "", dbSuggestedEngine: "mysql", dbDumpEngine: "mysql", dbDumpOff: true });
    expect(toggle().getAttribute("aria-checked")).toBe("false");
    expect(screen.queryByText(en["dbdump.noDumpYet"])).toBeNull();
  });

  it("shows the engine a lookalike is dumped with, and the one picked last", async () => {
    localStorage.setItem("bombvault.advanced", "1");
    renderRow({ dbTier: "lookalike", dbEngine: "", dbSuggestedEngine: "mysql", dbDumpEngine: "" });
    await act(async () => {
      fireEvent.click(toggle());
    });
    const picker = await screen.findByRole("combobox", { name: en["dbdump.engineLabel"] });
    expect(shownEngine()).toBe("MySQL");
    await act(async () => {
      fireEvent.click(picker);
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("option", { name: "MariaDB" }));
    });
    await waitFor(() => expect(setDbDumpEngine).toHaveBeenCalledWith("immich_postgres", "mariadb"));
    expect(shownEngine()).toBe("MariaDB");
  });

  it("keeps the dump on where the container's label names the engine", () => {
    renderRow({ dbTier: "label", dbEngine: "mariadb", dbDumpOff: true, dbDataCoverage: "live" });
    expect(toggle().getAttribute("aria-checked")).toBe("true");
    expect(toggle().hasAttribute("disabled")).toBe(true);
    expect(hints()).toContain(filled("dbdump.toggleHintLabel", { engine: "MariaDB" }));
  });

  it("leaves the switch alone where the container's own label decides", () => {
    renderRow({ dbDumpLabelOff: true, dbDumpOff: false });
    expect(toggle().getAttribute("aria-checked")).toBe("false");
    expect(toggle().hasAttribute("disabled")).toBe(true);
    expect(hints()).toContain(en["dbdump.labelOff"]);
  });

  it("shows the stored switch as off while dumps are off for every container", () => {
    renderRow({ dbDumpsGlobalOff: true, dbDumpOff: false });
    expect(toggle().getAttribute("aria-checked")).toBe("false");
    expect(toggle().hasAttribute("disabled")).toBe(true);
    expect(hints()).toContain(en["dbdump.globalOff"]);
  });

  it("says when the container's own pre-backup command already dumps the database", () => {
    renderRow({ dbDumpHookOverlap: true });
    expect(screen.getByText(en["dbdump.hookOverlap"])).toBeTruthy();
  });

  it("waits for the first dump instead of leaving the line empty", () => {
    renderRow();
    expect(screen.getByText(en["dbdump.noDumpYet"])).toBeTruthy();
  });

  it("says how much the last dump held", () => {
    renderRow({ lastDbDump: { at: 1_700_000_000, status: "success", bytes: 5_242_880, error: "" } });
    expect(screen.getByText(/5\.0 MB/)).toBeTruthy();
    expect(document.body.textContent).not.toContain("{result}");
    expect(document.body.textContent).not.toContain("{when}");
  });

  it("keeps a partial dump's note visible, in the warning tone", () => {
    renderRow({
      lastDbDump: {
        at: 1_700_000_000,
        status: "success",
        bytes: 1024,
        error: "database dump covers one database only",
      },
    });
    const note = screen.getByText(en["runReason.dbdumpOneDatabase"]);
    expect(note.className).toContain("statusWarn");
  });

  it("translates a failure and offers the remedy", () => {
    renderRow({
      lastDbDump: {
        at: 1_700_000_000,
        status: "failed",
        bytes: 0,
        error: "database dump failed: the database refused the login: FATAL: password authentication failed",
      },
    });
    const failure = screen.getByText(en["runReason.dbdumpAuth"], { exact: false });
    expect(failure.className).toContain("statusFail");
    expect(hints()).toContain(en["dbdump.fixAuth"]);
  });

  it("leaves the tool's own message off the card", () => {
    renderRow({
      lastDbDump: {
        at: 1_700_000_000,
        status: "failed",
        bytes: 0,
        error:
          "database dump failed: the container is paused or restarting: dockercli: exec create: container is paused, " +
          "restarting or stopped: Error response from daemon: Container pg is paused, unpause the container before exec",
      },
    });
    expect(screen.getByText(en["runReason.dbdumpNotRunning"])).toBeTruthy();
    expect(document.body.textContent).not.toContain("dockercli");
    expect(document.body.textContent).not.toContain("Error response from daemon");
  });

  it("says a cancelled dump was cancelled and offers no remedy for it", () => {
    renderRow({ lastDbDump: { at: 1_700_000_000, status: "failed", bytes: 0, error: "cancelled by the user" } });
    expect(screen.getByText(en["dbdump.resultCancelled"])).toBeTruthy();
    expect(hints()).not.toContain(en["dbdump.fixAuth"]);
  });

  it("says a cancelled dump may still be running in the container", () => {
    renderRow({
      lastDbDump: { at: 1_700_000_000, status: "failed", bytes: 0, error: "cancelled by the user: orphan stop failed" },
    });
    expect(screen.getByText(en["dbdump.resultCancelled"])).toBeTruthy();
    expect(screen.getByText(en["runReason.dbdumpOrphan"]).className).toContain("statusWarn");
  });

  it("puts the switch back and says so when the setting could not be saved", async () => {
    setDbDumpOff.mockResolvedValueOnce({ ok: false, error: "" });
    renderRow();
    await act(async () => {
      fireEvent.click(toggle());
    });
    await waitFor(() => expect(push).toHaveBeenCalledWith(en["dbdump.settingFailed"], "fail"));
    expect(toggle().getAttribute("aria-checked")).toBe("true");
  });
});
