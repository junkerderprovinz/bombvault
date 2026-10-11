import { describe, expect, it } from "vitest";
import type { BackendOption, Provider } from "./api";
import { connectionPatch, storedConnection, type StoredConnection } from "./storageConnection";

function provider(route: Provider["route"]): Provider {
  return { id: route, name: route, backend: "", group: "server", route };
}

function option(name: string): BackendOption {
  return { name, help: "", required: false, secret: false, password: false, advanced: false, essential: true, default: "" };
}

const isSecret = (o: BackendOption) => o.name === "pass";

describe("storedConnection", () => {
  it("reads a folder destination's path off its address", () => {
    expect(storedConnection(provider("path"), "/mnt/remotes/nas/bombvault", {}, undefined)).toEqual({
      values: { path: "/mnt/remotes/nas/bombvault" },
      hidden: [],
      dir: "",
    });
  });

  it("reads a rest-server's address without the scheme and its user off the credentials", () => {
    const stored = storedConnection(provider("rest"), "rest:https://nas:8000/tower", { restUser: "tower" }, undefined);
    expect(stored.values).toEqual({ url: "https://nas:8000/tower", user: "tower" });
    expect(stored.hidden).toEqual(["pass"]);
  });

  it("splits an S3 address into the endpoint and the bucket with its folder", () => {
    const stored = storedConnection(provider("s3"), "s3:https://s3.example.com/bucket/a/b", { s3KeyId: "key", s3Region: "eu" }, undefined);
    expect(stored.values).toEqual({ endpoint: "https://s3.example.com", region: "eu", access_key_id: "key" });
    expect(stored.dir).toBe("bucket/a/b");
  });

  it("reads an S3 endpoint written without a scheme", () => {
    expect(storedConnection(provider("s3"), "s3:s3.amazonaws.com/bucket", {}, undefined)).toMatchObject({
      values: { endpoint: "s3.amazonaws.com" },
      dir: "bucket",
    });
  });

  it("takes an rclone remote's fields from its section and names the ones it keeps to itself", () => {
    const section = { name: "box", type: "sftp", options: [{ name: "host", value: "h" }, { name: "pass", hidden: true }, { name: "port", value: "" }] };
    expect(storedConnection(provider("rclone"), "rclone:box:backups/bv", {}, section)).toEqual({
      values: { host: "h", port: "" },
      hidden: ["pass"],
      dir: "backups/bv",
    });
  });

  it("reads an rclone remote without a folder and without its section", () => {
    expect(storedConnection(provider("rclone"), "rclone:box:", {}, undefined)).toEqual({ values: {}, hidden: [], dir: "" });
  });
});

describe("connectionPatch", () => {
  const stored: StoredConnection = { values: { host: "h", user: "u" }, hidden: ["pass"], dir: "bv" };
  const fields = [option("host"), option("user"), option("pass")];

  it("is empty while nothing differs from what is stored", () => {
    expect(connectionPatch(fields, isSecret, stored, { host: "h", user: " u ", pass: "" }, "bv")).toEqual({});
  });

  it("carries a changed field and leaves the others out", () => {
    expect(connectionPatch(fields, isSecret, stored, { host: "other" }, "bv")).toEqual({ settings: { host: "other" } });
  });

  it("carries an emptied field, which removes the setting", () => {
    expect(connectionPatch(fields, isSecret, stored, { user: "" }, "bv")).toEqual({ settings: { user: "" } });
  });

  it("carries a secret only when one was typed", () => {
    expect(connectionPatch(fields, isSecret, stored, { pass: "new" }, "bv")).toEqual({ settings: { pass: "new" } });
  });

  it("carries the folder when it moved, whatever slashes surround it", () => {
    expect(connectionPatch(fields, isSecret, stored, {}, "/bv/")).toEqual({});
    expect(connectionPatch(fields, isSecret, stored, {}, "bv/next")).toEqual({ dir: "bv/next" });
  });

  it("ignores what was typed into a field the form no longer shows", () => {
    expect(connectionPatch([option("host")], isSecret, stored, { user: "x" }, "bv")).toEqual({});
  });
});
