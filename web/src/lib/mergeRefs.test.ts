import { expect, it, vi } from "vitest";
import { mergeRefs } from "./mergeRefs";

it("sets a callback ref and an object ref to the same node", () => {
  const seen: (string | null)[] = [];
  const callback = vi.fn((el: string | null) => seen.push(el));
  const object = { current: null as string | null };

  const combined = mergeRefs(callback, object);
  combined("node");

  expect(seen).toEqual(["node"]);
  expect(object.current).toBe("node");
});

it("clears every ref on unmount", () => {
  const object = { current: "node" as string | null };
  const combined = mergeRefs(object);

  combined(null);

  expect(object.current).toBeNull();
});

it("skips undefined and null refs instead of throwing", () => {
  const object = { current: null as string | null };
  const combined = mergeRefs(undefined, object, null);

  expect(() => combined("node")).not.toThrow();
  expect(object.current).toBe("node");
});
