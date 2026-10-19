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
});
