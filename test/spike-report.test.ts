import { test } from "node:test";
import assert from "node:assert/strict";
import { assembleReport, type Probe } from "../server/spike-report";

const ok: Probe = async () => ({ name: "ok-check", ok: true, detail: "all good" });
const fail: Probe = async () => ({ name: "fail-check", ok: false, error: "boom" });
const thrower: Probe = async () => {
  throw new Error("unexpected crash");
};

test("assembleReport reports each probe with ok/fail and detail", async () => {
  const report = await assembleReport([ok, fail]);
  assert.equal(report.checks.length, 2);
  assert.equal(report.checks[0].ok, true);
  assert.equal(report.checks[0].detail, "all good");
  assert.equal(report.checks[1].ok, false);
  assert.equal(report.checks[1].error, "boom");
});

test("a probe that throws is captured as a failed check, never crashes the report", async () => {
  const report = await assembleReport([thrower]);
  assert.equal(report.checks.length, 1);
  assert.equal(report.checks[0].ok, false);
  assert.match(report.checks[0].error ?? "", /unexpected crash/);
});

test("overall is true only when every check passes", async () => {
  assert.equal((await assembleReport([ok])).overall, true);
  assert.equal((await assembleReport([ok, fail])).overall, false);
});

test("report includes a generatedAt timestamp", async () => {
  const report = await assembleReport([ok]);
  assert.equal(typeof report.generatedAt, "number");
  assert.ok(report.generatedAt > 0);
});
