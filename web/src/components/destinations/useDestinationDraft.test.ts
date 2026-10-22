import { describe, expect, it } from "vitest";
import type { Backend, BackendOption, Provider } from "../../lib/api";
import { bucketToOpen, endpointBucket, formFields, previewLocation } from "./useDestinationDraft";

const minio: Provider = { id: "minio", name: "MinIO", backend: "s3", group: "selfs3", route: "s3" };

const option = (name: string, extra: Partial<BackendOption> = {}): BackendOption => ({
  name, help: "", required: false, secret: false, password: false, advanced: false, essential: false, default: "", ...extra,
});

const s3Backend: Backend = {
  name: "s3",
  description: "S3",
  options: [
    option("access_key_id", { essential: true }),
    option("region", { essential: true, providers: ["AWS", "Other"] }),
    option("endpoint", { essential: true, providers: ["AWS", "Other"] }),
    option("location_constraint", { providers: ["AWS"] }),
    option("acl", { providers: ["AWS", "IDrive"] }),
    option("server_side_encryption", { providers: ["AWS"] }),
    option("storage_class", { providers: ["AWS"] }),
    option("bucket_object_lock_enabled"),
  ],
};

const s3Provider = (id: string, provider: string): Provider => ({
  id, name: id, backend: "s3", group: "storage", route: "s3", preset: { provider },
});

describe("formFields", () => {
  it("asks for the endpoint of an S3 service rclone finds it for by itself", () => {
    const fields = formFields(s3Provider("idrivee2", "IDrive"), s3Backend);
    expect(fields.main.map((o) => o.name)).toEqual(["access_key_id", "endpoint"]);
  });

  it("offers only the further S3 settings that shape a new bucket", () => {
    const fields = formFields(s3Provider("aws", "AWS"), s3Backend);
    expect(fields.more.map((o) => o.name)).toEqual(["location_constraint", "bucket_object_lock_enabled"]);
  });
});

describe("the bucket a test opens", () => {
  it("comes from the field, or from a bucket's address pasted as the endpoint", () => {
    expect(bucketToOpen(minio, " bv/ ", {})).toBe("bv");
    expect(bucketToOpen(minio, "", { endpoint: "https://acct.r2.cloudflarestorage.com/backups" })).toBe("backups");
    expect(bucketToOpen(minio, "", { endpoint: "s3:http://minio:9000/bv/box" })).toBe("bv");
    expect(bucketToOpen(minio, "", { endpoint: "http://minio:9000" })).toBe("");
    expect(endpointBucket("minio:9000/")).toBe("");
  });

  it("is none for a provider that is not S3", () => {
    const rest: Provider = { id: "rest", name: "rest-server", backend: "", group: "server", route: "rest" };
    expect(bucketToOpen(rest, "bv", {})).toBe("");
  });
});

describe("previewLocation", () => {
  it("puts a plain S3 endpoint in front of the folder", () => {
    expect(previewLocation(minio, { endpoint: "http://minio:9000/" }, "/bv/box/")).toBe("s3:http://minio:9000/bv/box");
  });

  it("reads a restic address in the endpoint field the way the server does", () => {
    expect(previewLocation(minio, { endpoint: "s3:http://minio:9000" }, "bv/box")).toBe("s3:http://minio:9000/bv/box");
    expect(previewLocation(minio, { endpoint: "s3:minio:9000/old/path" }, "bv/box")).toBe("s3:https://minio:9000/bv/box");
  });

  it("leaves out the bucket of a bucket's address pasted as the endpoint", () => {
    expect(previewLocation(minio, { endpoint: "https://acct.r2.cloudflarestorage.com/backups" }, "backups/bv")).toBe(
      "s3:https://acct.r2.cloudflarestorage.com/backups/bv"
    );
  });
});
