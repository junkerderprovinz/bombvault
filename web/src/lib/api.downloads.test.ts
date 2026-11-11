// A download is a plain link the browser follows, and its preflight asks the
// same route with check=1. Both have to reach the route for every source,
// the item's own location included, which carries no source parameter.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { checkDbDumpDownload, dbDumpDownloadURL } from "./api";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) });
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const asked = () => String(fetchMock.mock.calls[0][0]);

describe("the download of a database dump", () => {
  it.each([
    ["local", false, "/api/containers/pg/dbdumps/a1a1a1a1/download"],
    ["local", true, "/api/containers/pg/dbdumps/a1a1a1a1/download?gz=1"],
    ["offsite:t1", true, "/api/containers/pg/dbdumps/a1a1a1a1/download?source=offsite%3At1&gz=1"],
  ])("from %s, gzipped %s, links to the route", (source, gz, want) => {
    expect(dbDumpDownloadURL("pg", "a1a1a1a1", source, gz)).toBe(want);
  });

  it.each([
    ["local", "/api/containers/pg/dbdumps/a1a1a1a1/download?check=1"],
    ["offsite:t1", "/api/containers/pg/dbdumps/a1a1a1a1/download?source=offsite%3At1&check=1"],
  ])("from %s asks the route first", async (source, want) => {
    await checkDbDumpDownload("pg", "a1a1a1a1", source);
    expect(asked()).toBe(want);
  });
});
