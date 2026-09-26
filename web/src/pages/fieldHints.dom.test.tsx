// @vitest-environment jsdom
// A window explains its fields in an info bubble on the label, not in a grey
// line under the field, which is read once and then only costs height
// (GlimStone rule 8).
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import type { ReactNode } from "react";
import { I18nProvider, countText, en, type TranslationKey, type useT } from "../lib/i18n";
import type { FileSetView, FleetPeer } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  browse: async () => ({ ok: true, dirs: [] }),
  listRepos: async () => ({ ok: true, repos: [] }),
  getCloudCredSets: async () => ({ ok: true, sets: [] }),
}));

const { FileSetDialog } = await import("./Files");
const { ReceiverDialog } = await import("./Receiver");
const { FleetDialog, ProposeMeshDialog } = await import("./Fleet");

const t = ((key: TranslationKey, n?: number) => countText(en[key], "en", n)) as unknown as ReturnType<typeof useT>["t"];

const documents: FileSetView = {
  id: "set1",
  name: "Documents",
  path: "documents",
  excludes: [],
  enabled: true,
  lastBackup: 0,
  pathExists: true,
};

const tower: FleetPeer = {
  id: "p1",
  name: "tower",
  url: "https://192.168.1.50:3443",
  enabled: true,
  lastPollAt: 0,
  lastPollOk: null,
  lastPollError: "",
  lastPollInstanceName: "",
  lastPollVersion: "",
  lastPollDomains: [],
  createdAt: 0,
  sortOrder: 0,
  hasToken: true,
};

const noop = () => {};

const WINDOWS: { name: string; node: ReactNode; hints: TranslationKey[] }[] = [
  {
    name: "the folder set window",
    node: <FileSetDialog initial={documents} presetSeed={null} hostMountRoot="/mnt/user" t={t} onClose={noop} onSaved={noop} />,
    hints: ["files.pathHint", "files.pathChangeHint", "files.excludesHint"],
  },
  {
    name: "the received repository window",
    node: <ReceiverDialog initial={null} t={t} onClose={noop} onSaved={noop} />,
    hints: ["receiver.repoLocationHint", "receiver.appKeyHint", "receiver.deadManHoursHint", "receiver.checkCadenceHint"],
  },
  {
    name: "the fleet peer window",
    node: <FleetDialog initial={null} t={t} onClose={noop} onSaved={noop} />,
    hints: ["fleet.urlHint", "fleet.tokenHint"],
  },
  {
    name: "the mesh proposal window",
    node: <ProposeMeshDialog peer={tower} t={t} onClose={noop} />,
    hints: ["fleet.mesh.baseUrlHint"],
  },
];

afterEach(cleanup);

describe.each(WINDOWS)("$name", ({ node, hints }) => {
  it("explains its fields in info bubbles on their labels", async () => {
    await act(async () => {
      render(<I18nProvider>{node}</I18nProvider>);
    });
    const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
    const bubbles = [...dialog.querySelectorAll<HTMLElement>("label [aria-label]")];
    for (const key of hints) {
      expect(bubbles.some((b) => b.getAttribute("aria-label")!.includes(en[key])), `${key} in a bubble`).toBe(true);
      expect(dialog.textContent, `${key} printed on the page`).not.toContain(en[key]);
    }
  });
});
