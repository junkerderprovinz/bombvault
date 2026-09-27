// A key stores its client's id, so an id that changes or collides would put the
// wrong mark on keys already made. The rest keeps a button from opening a
// dialog with nothing to copy or a mark that is not there.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { CLIENT_MARKS } from "./mcpClientMarks";
import { CLOUD_CLIENTS, LOCAL_CLIENTS, OTHER_CLIENT, clientById, snippetFor } from "./mcpClients";

const indexCss = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");
const ALL = [...LOCAL_CLIENTS, ...CLOUD_CLIENTS, OTHER_CLIENT];
const input = { origin: "https://tower:3443", endpointPath: "/mcp", key: "<your key>", selfSigned: true };

describe("the client list", () => {
  it("has 28 clients on this computer and 4 in the cloud", () => {
    expect(LOCAL_CLIENTS).toHaveLength(28);
    expect(CLOUD_CLIENTS).toHaveLength(4);
  });

  it("gives every client an id the server stores", () => {
    const ids = ALL.map((c) => c.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const id of ids) expect(id).toMatch(/^[a-z0-9][a-z0-9-]{0,31}$/);
  });

  it("has a mark and a tile colour for every button", () => {
    for (const c of [...LOCAL_CLIENTS, ...CLOUD_CLIENTS]) {
      expect(CLIENT_MARKS[c.mark ?? ""], c.id).toBeDefined();
    }
    for (const c of ALL) expect(indexCss, c.tile).toContain(`.${c.tile} {`);
  });

  it("has a configuration to copy for every client that gets a setup", () => {
    for (const c of ALL) {
      if (c.oauth) expect(snippetFor(c, input), c.id).toBeUndefined();
      else expect(snippetFor(c, input), c.id).toContain("https://tower:3443/mcp");
    }
  });

  it("sends only the cloud clients to the internet warning", () => {
    expect(CLOUD_CLIENTS.every((c) => c.group === "cloud")).toBe(true);
    expect(LOCAL_CLIENTS.every((c) => c.group === "local")).toBe(true);
  });

  it("finds the client of a stored key, and nothing for none or an unknown one", () => {
    expect(clientById("cursor")?.name).toBe("Cursor");
    expect(clientById("")).toBeUndefined();
    expect(clientById("other")).toBeUndefined();
    expect(clientById("gone")).toBeUndefined();
  });
});
