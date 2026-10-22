import { describe, expect, it } from "vitest";
import type { Provider } from "../../lib/api";
import { previewLocation } from "./useDestinationDraft";

const minio: Provider = { id: "minio", name: "MinIO", backend: "s3", group: "selfs3", route: "s3" };

describe("previewLocation", () => {
  it("puts a plain S3 endpoint in front of the folder", () => {
    expect(previewLocation(minio, { endpoint: "http://minio:9000/" }, "/bv/box/")).toBe("s3:http://minio:9000/bv/box");
  });

  it("reads a restic address in the endpoint field the way the server does", () => {
    expect(previewLocation(minio, { endpoint: "s3:http://minio:9000" }, "bv/box")).toBe("s3:http://minio:9000/bv/box");
    expect(previewLocation(minio, { endpoint: "s3:minio:9000/old/path" }, "bv/box")).toBe("s3:https://minio:9000/bv/box");
  });

  it("reads a rest-server address the way the server does", () => {
    const rest: Provider = { id: "rest", name: "rest-server", backend: "", group: "server", route: "rest" };
    expect(previewLocation(rest, { url: "rest:http://nas:8000/tower/" }, "")).toBe("rest:http://nas:8000/tower");
    expect(previewLocation(rest, { url: "nas:8000/tower" }, "")).toBe("rest:http://nas:8000/tower");
    expect(previewLocation(rest, { url: "https://nas/tower" }, "")).toBe("rest:https://nas/tower");
  });
});
