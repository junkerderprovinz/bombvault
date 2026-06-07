import { test } from "node:test";
import assert from "node:assert/strict";
import { decideRoute } from "../lib/route-guard";

test("unauthenticated user on a protected route is sent to /login", () => {
  assert.equal(decideRoute({ path: "/dashboard", authed: false }), "/login");
  assert.equal(decideRoute({ path: "/spike", authed: false }), "/login");
});

test("authenticated user is allowed through protected routes", () => {
  assert.equal(decideRoute({ path: "/dashboard", authed: true }), null);
  assert.equal(decideRoute({ path: "/spike", authed: true }), null);
});

test("public routes are always allowed", () => {
  assert.equal(decideRoute({ path: "/login", authed: false }), null);
  assert.equal(decideRoute({ path: "/onboarding", authed: false }), null);
});

test("authenticated user visiting /login is bounced to /dashboard", () => {
  assert.equal(decideRoute({ path: "/login", authed: true }), "/dashboard");
});
