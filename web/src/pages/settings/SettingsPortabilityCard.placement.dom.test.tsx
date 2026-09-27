// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { ImportSettingsSummary } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { renderWithProviders, targetPreview } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());
const file = vi.hoisted(() => ({ summary: null as unknown }));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
  importSettingsPreview: () => Promise.resolve({ ok: true, summary: file.summary }),
}));

const { SettingsPortabilityCard } = await import("./SettingsPortabilityCard");
const { subscribePlacement } = await import("../../lib/placementEvents");

function summary(over: Partial<ImportSettingsSummary>): ImportSettingsSummary {
  return {
    schemaVersion: 1,
    exportedAt: "2026-09-18T10:00:00Z",
    appVersion: "v8.12.0",
    offsiteTargets: 2,
    namedRepos: 1,
    credentials: { present: false, cloud: false, rclone: false, notify: false, credSets: 0 },
    settingsGroups: [],
    placementDefaults: 3,
    copyRules: null,
    places: 2,
    newTargets: [],
    ...over,
  };
}

const withCreds = (credSets: number) => ({ present: true, cloud: true, rclone: false, notify: false, credSets });

const hetzner = {
  id: "t-new",
  domain: "vms" as const,
  name: "Hetzner",
  preview: targetPreview({ formerlyExcluded: [{ identity: "vm:win11", skip: ["t-b2"] }] }),
};

function Harness({ applyImport }: { applyImport: (text: string) => Promise<{ ok: boolean }> }) {
  const { t } = useT();
  return <SettingsPortabilityCard t={t} applyImport={applyImport} />;
}

async function pickFile(applyImport = vi.fn(async () => ({ ok: true }))) {
  renderWithProviders(<Harness applyImport={applyImport} />);
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  await act(async () => {
    fireEvent.change(input, { target: { files: [{ text: () => Promise.resolve("{}") }] } });
  });
  return applyImport;
}

function valueOf(label: string): string | null | undefined {
  return screen.getByText(label).nextElementSibling?.textContent;
}

describe("the import preview and placement", () => {
  beforeEach(() => fake.reset());
  afterEach(() => {
    cleanup();
    localStorage.removeItem("bv-lang");
  });

  it("counts the defaults and rules of the file, or says it carries none", async () => {
    file.summary = summary({});
    await pickFile();
    expect(valueOf("Placement defaults")).toBe("3");
    expect(valueOf("Copy rules")).toBe("not in the file, stays as it is");
  });

  it("counts the storage places of the file", async () => {
    file.summary = summary({});
    await pickFile();
    expect(valueOf("Storage places")).toBe("2");
  });

  it("says an older file without places has them built from its settings", async () => {
    file.summary = summary({ places: null });
    await pickFile();
    expect(valueOf("Storage places")).toBe("not in the file, built from the imported settings");
  });

  it.each([
    ["included, with 1 credential set", 1],
    ["included, with 3 credential sets", 3],
    ["included", 0],
  ])("says %j for a credential-set count of %i", async (text, sets) => {
    file.summary = summary({ credentials: withCreds(sets) });
    await pickFile();
    expect(valueOf("Credentials")).toBe(text);
  });

  it("counts the credential sets in German too", async () => {
    localStorage.setItem("bv-lang", "de");
    file.summary = summary({ credentials: withCreds(1) });
    await pickFile();
    expect(valueOf("Zugangsdaten")).toBe("enthalten, mit 1 Zugangsdaten-Satz");
    cleanup();
    file.summary = summary({ credentials: withCreds(2) });
    await pickFile();
    expect(valueOf("Zugangsdaten")).toBe("enthalten, mit 2 Zugangsdaten-Sätzen");
  });

  it("asks what each new target receives and writes the ticked exclusions after the import", async () => {
    file.summary = summary({ newTargets: [hetzner] });
    const seen = vi.fn();
    const off = subscribePlacement(seen);
    const applyImport = await pickFile();
    const block = screen.getByText("New target Hetzner for VMs").parentElement as HTMLElement;
    expect(block.textContent).toContain("Items: 15");
    expect(block.textContent).not.toContain("project folders");
    fireEvent.click(within(block).getByRole("switch", { name: "Leave these out here too" }));
    fireEvent.click(screen.getByRole("button", { name: "Replace settings" }));
    await waitFor(() =>
      expect(fake.callsTo("excludeFromTarget")).toEqual([
        [{ domain: "vms", targetId: "t-new", identities: ["vm:win11"], default: false }],
      ])
    );
    expect(applyImport).toHaveBeenCalledTimes(1);
    expect(seen).toHaveBeenCalledTimes(1);
    off();
  });

  it("writes no exclusion nobody ticked", async () => {
    file.summary = summary({ newTargets: [hetzner], placementDefaults: null });
    const applyImport = await pickFile();
    fireEvent.click(screen.getByRole("button", { name: "Replace settings" }));
    await waitFor(() => expect(applyImport).toHaveBeenCalledTimes(1));
    expect(fake.callsTo("excludeFromTarget")).toEqual([]);
  });
});
