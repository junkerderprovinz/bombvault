// runReason has no framework dependency, and RunReasonText returns a plain
// element tree, so both are checked as objects without jsdom. The stub resolver
// returns the key, which makes the chosen translation assertable.
import { describe, expect, it } from "vitest";
import {
  isOwnReason,
  runReason,
  runReasonParts,
  RunReasonText,
  RUN_REASON_PREFIXES,
} from "./runReason";
import type { TranslationKey } from "./i18n";

const t = (key: TranslationKey): string => key;

interface ElementNode {
  type?: unknown;
  props?: { dir?: string; children?: unknown };
}

function children(node: unknown): unknown[] {
  const kids = (node as ElementNode).props?.children;
  return Array.isArray(kids) ? kids : [kids];
}

describe("runReason", () => {
  it("translates database dump reasons with detail", () => {
    expect(runReason("database dump failed: the database refused the login: FATAL x", t)).toBe(
      "runReason.dbdumpAuth: FATAL x"
    );
    expect(runReasonParts("database dump failed: the database refused the login: FATAL x", t)).toEqual({
      head: "runReason.dbdumpAuth",
      detail: "FATAL x",
    });
  });

  it("translates every reason it knows, with and without a detail", () => {
    for (const [reason, key] of Object.entries(RUN_REASON_PREFIXES)) {
      expect(runReason(reason, t)).toBe(key);
      expect(runReasonParts(`${reason}: tail`, t)).toEqual({ head: key, detail: "tail" });
    }
  });

  it("covers the reasons that carry more than a tool message", () => {
    expect(runReason("database dump failed: the database's system tables need an upgrade", t)).toBe(
      "runReason.dbdumpNeedsUpgrade"
    );
    expect(runReason("database dump failed: the backup's own time limit was reached", t)).toBe(
      "runReason.dbdumpBackupCap"
    );
    expect(runReason("database dump failed: a damaged dump snapshot could not be removed: 1a2b3c4d", t)).toBe(
      "runReason.dbdumpLeftover: 1a2b3c4d"
    );
    expect(runReason("database import failed and the old data could not be put back: /a, /b", t)).toBe(
      "runReason.dbimportRollback: /a, /b"
    );
    expect(runReason("database imported with errors: 3, /old", t)).toBe("runReason.dbimportErrors: 3, /old");
  });

  it("prefers an exact match over a prefix", () => {
    expect(runReason("cancelled by the user", t)).toBe("runReason.cancelled");
    expect(runReasonParts("cancelled by the user", t)).toEqual({
      head: "runReason.cancelled",
      detail: "",
    });
  });

  it("hands back a reason it does not know", () => {
    expect(runReason("Fatal: repository is already locked", t)).toBe("Fatal: repository is already locked");
    expect(runReasonParts("Fatal: repository is already locked", t)).toEqual({
      head: "Fatal: repository is already locked",
      detail: "",
    });
    expect(runReason("", t)).toBe("");
  });

  it("owns a reason only while it stands alone", () => {
    expect(isOwnReason("database dump failed: the dump was empty")).toBe(true);
    expect(isOwnReason("database dump failed: the dump was empty: exit 1")).toBe(false);
    expect(isOwnReason("Fatal: repository is already locked")).toBe(false);
  });
});

describe("RunReasonText", () => {
  it("isolates the detail and leaves the head to the page direction", () => {
    const parts = children(RunReasonText({ reason: "database dump failed: the dump tool reported an error: ERROR 1045", t }));
    expect(parts[0]).toBe("runReason.dbdumpTool");
    expect(parts[1]).toBe(": ");
    const detail = parts[2] as ElementNode;
    expect(detail.type).toBe("bdi");
    expect(detail.props?.dir).toBe("ltr");
    expect(detail.props?.children).toBe("ERROR 1045");
  });

  it("renders a reason without a detail as bare text", () => {
    const parts = children(RunReasonText({ reason: "database dump failed: no progress", t }));
    expect(parts).toEqual(["runReason.dbdumpStalled"]);
  });
});
