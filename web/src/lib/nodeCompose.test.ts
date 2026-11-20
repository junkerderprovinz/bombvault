import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { composeFile } from "./nodeCompose";

const FILE = readFileSync(new URL("../../../deploy/docker-compose.node.yml", import.meta.url), "utf8");
const VALUES = { MODE: "node", APP_KEY: "ab".repeat(32), NODE_HOME: "bvhome1.eyJpZCI6ImhvbWUifQ", NODE_NAME: 'VPS "Falkenstein"' };

describe("composeFile", () => {
  it("is the compose file of the repository with the node's values in place", () => {
    const filled = composeFile(VALUES).split("\n");
    const lines = FILE.split("\n");
    expect(filled).toHaveLength(lines.length);
    const changed = lines.flatMap((line, i) => (line === filled[i] ? [] : [[line.trim(), filled[i].trim()]]));
    expect(changed).toEqual([
      ['APP_KEY: ""', `APP_KEY: "${VALUES.APP_KEY}"`],
      ['NODE_HOME: ""', `NODE_HOME: "${VALUES.NODE_HOME}"`],
      ['NODE_NAME: ""', 'NODE_NAME: "VPS \\"Falkenstein\\""'],
    ]);
  });
});
