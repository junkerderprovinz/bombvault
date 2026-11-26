// @vitest-environment jsdom
// A folder set's snapshot lists the folders above the set's own path too, and
// the server refuses a pick among them, so only the set's own folder and what
// lies below it can be ticked.
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { en } from "../lib/i18n";
import type { FileEntry } from "../lib/api";
import { SnapshotFileTree } from "./SnapshotFileTree";

afterEach(cleanup);

const files: FileEntry[] = [
  { path: "/host", type: "dir" },
  { path: "/host/user", type: "dir" },
  { path: "/host/user/user", type: "dir" },
  { path: "/host/user/user/photos", type: "dir" },
  { path: "/host/user/user/photos/2024", type: "dir" },
  { path: "/host/user/user/photos/2024/beach.jpg", type: "file" },
  { path: "/host/user/user/photosets", type: "dir" },
] as FileEntry[];

const t = (key: string) => (en as Record<string, string>)[key] ?? key;

function show(filter = "", root?: string) {
  const onToggle = vi.fn();
  render(
    <SnapshotFileTree
      files={files}
      loading={false}
      error={null}
      filter={filter}
      onFilterChange={() => undefined}
      selected={new Set()}
      onToggle={onToggle}
      root={root}
      t={t as never}
    />,
  );
  return onToggle;
}

it("opens the tree down to the set's own folder and offers only it and what lies below it", () => {
  const onToggle = show("", "/host/user/user/photos");
  for (const above of ["/host", "/host/user", "/host/user/user", "/host/user/user/photosets"]) {
    expect(screen.queryByRole("checkbox", { name: above })).toBeNull();
  }
  fireEvent.click(screen.getByRole("checkbox", { name: "/host/user/user/photos" }));
  expect(onToggle).toHaveBeenCalledWith("/host/user/user/photos");
});

it("lists no match above the set's own folder while a filter is set", () => {
  show("photo", "/host/user/user/photos");
  expect(screen.queryByRole("checkbox", { name: "/host/user/user/photosets" })).toBeNull();
  expect(screen.getByRole("checkbox", { name: "/host/user/user/photos" })).toBeTruthy();
  expect(screen.getByRole("checkbox", { name: "/host/user/user/photos/2024/beach.jpg" })).toBeTruthy();
});

it("offers every folder of a listing that names no root", () => {
  show();
  expect(screen.getByRole("checkbox", { name: "/host" })).toBeTruthy();
});
