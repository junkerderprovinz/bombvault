// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider, en } from "../lib/i18n";

const browse = vi.fn(async (path: string) => ({
  ok: true,
  dirs: ["appdata", "photos"].map((name) => ({ name, path: path ? `${path}/${name}` : name })),
}));
vi.mock("../lib/api", () => ({
  browse: (p: string) => browse(p),
  createFolder: vi.fn(),
}));
vi.mock("../lib/toast", () => ({ useToast: () => ({ push: vi.fn() }) }));

import { FolderBrowser } from "./FolderBrowser";

const scrolled: { el: Element; arg: unknown }[] = [];

beforeEach(() => {
  scrolled.length = 0;
  browse.mockClear();
  Element.prototype.scrollIntoView = function (this: Element, arg?: unknown) {
    scrolled.push({ el: this, arg });
  } as Element["scrollIntoView"];
});

afterEach(() => {
  cleanup();
});

function renderInPlace() {
  render(
    <I18nProvider>
      <FolderBrowser inDialog label="Folder" value="" hostMountRoot="/mnt/user" onChange={() => {}} />
    </I18nProvider>,
  );
}

it("brings the panel in place into view once, when it opens", async () => {
  renderInPlace();
  fireEvent.click(screen.getByRole("button", { name: en["folder.browseTitle"] }));
  await screen.findByRole("button", { name: "photos" });

  const use = screen.getByRole("button", { name: en["folder.use"] });
  expect(scrolled).toHaveLength(1);
  expect(scrolled[0].arg).toEqual({ block: "nearest" });
  expect(scrolled[0].el.contains(use)).toBe(true);
  // The margin is the window body's own bottom padding, so the panel rests
  // with the same room below it as the window's last row.
  expect(scrolled[0].el.className).toContain("scroll-mb-4");

  fireEvent.click(screen.getByRole("button", { name: "photos" }));
  await screen.findByRole("button", { name: ".." });
  expect(scrolled).toHaveLength(1);
});

it("keeps the listing's box while the next folder loads", async () => {
  renderInPlace();
  fireEvent.click(screen.getByRole("button", { name: en["folder.browseTitle"] }));
  const photos = await screen.findByRole("button", { name: "photos" });
  const box = photos.parentElement!;

  browse.mockReturnValueOnce(new Promise(() => {}));
  fireEvent.click(photos);
  expect(box.isConnected).toBe(true);
  expect(box.textContent).toContain(en["folder.loading"]);
});
