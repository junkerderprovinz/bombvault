import { test } from "node:test";
import assert from "node:assert/strict";
import { parseDestinationForm } from "../app/destinations/validate";

test("parseDestinationForm rejects empty repoPath", () => {
  assert.throws(
    () => parseDestinationForm({ name: "local", repoPath: "", password: "secret" }),
    (err: unknown) => {
      assert.ok(err instanceof Error, "expected an Error");
      return true;
    },
  );
});

test("parseDestinationForm rejects empty name", () => {
  assert.throws(() =>
    parseDestinationForm({ name: "", repoPath: "/data/repo", password: "secret" }),
  );
});

test("parseDestinationForm rejects empty password", () => {
  assert.throws(() =>
    parseDestinationForm({ name: "local", repoPath: "/data/repo", password: "" }),
  );
});

test("parseDestinationForm accepts valid input", () => {
  const result = parseDestinationForm({
    name: "my-repo",
    repoPath: "/mnt/user/backups/restic",
    password: "hunter2",
  });
  assert.equal(result.name, "my-repo");
  assert.equal(result.repoPath, "/mnt/user/backups/restic");
  assert.equal(result.password, "hunter2");
});
