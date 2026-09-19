// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { FileSetView } from "../lib/api";
import type { TranslationKey } from "../lib/i18n";

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    listRepos: vi.fn(async () => ({ ok: true, repos: [] })),
    browse: vi.fn(async () => ({ ok: true, dirs: [], status: "ok", truncated: false })),
  };
});

const { FileSetDialog } = await import("./Files");
const { en } = await import("../lib/i18n");

const t = ((key: TranslationKey) => en[key]) as unknown as Parameters<typeof FileSetDialog>[0]["t"];

const documents: FileSetView = {
  id: "set1",
  name: "Documents",
  path: "documents",
  excludes: [],
  enabled: true,
  lastBackup: 0,
  pathExists: true,
};

async function renderDialog(initial: FileSetView | null) {
  render(
    <FileSetDialog initial={initial} presetSeed={null} hostMountRoot="/host/user" t={t} onClose={() => {}} onSaved={() => {}} />,
  );
  await act(async () => {});
}

afterEach(() => {
  cleanup();
});

describe("the folder set's name", () => {
  it("is locked once the set has backups, with the reason in an info bubble", async () => {
    await renderDialog({ ...documents, lastBackup: 1_757_000_000 });
    expect((screen.getByDisplayValue("Documents") as HTMLInputElement).disabled).toBe(true);
    expect(screen.getByLabelText(en["files.nameLocked"])).toBeTruthy();
  });

  it("stays editable while the set has no backups", async () => {
    await renderDialog(documents);
    expect((screen.getByDisplayValue("Documents") as HTMLInputElement).disabled).toBe(false);
    expect(screen.queryByLabelText(en["files.nameLocked"])).toBeNull();
  });

  it("stays editable for a new set", async () => {
    await renderDialog(null);
    expect((screen.getByPlaceholderText("documents") as HTMLInputElement).disabled).toBe(false);
  });
});
