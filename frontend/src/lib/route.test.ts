import { test } from "node:test";
import assert from "node:assert/strict";
import { routeFor, withMock } from "./route.ts";

test("only named paths select a page and everything else is the overview", () => {
  assert.equal(routeFor("/"), "overview");
  assert.equal(routeFor("/workflows"), "workflows");
  assert.equal(routeFor("/workflows/"), "workflows");
  assert.equal(routeFor("/runners"), "runners");
  assert.equal(routeFor("/anything-else"), "overview");
  assert.equal(routeFor("/workflows/extra"), "overview");
});

test("withMock keeps the dev preview flag across navigation without touching other queries", () => {
  assert.equal(withMock("/workflows?status=failure", true), "/workflows?status=failure&mock=");
  assert.equal(withMock("/runners", true), "/runners?mock=");
  assert.equal(withMock("/runners", false), "/runners");
  assert.equal(withMock("/runners?mock=", true), "/runners?mock=");
});
