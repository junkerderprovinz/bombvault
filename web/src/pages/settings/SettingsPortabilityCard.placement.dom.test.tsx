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
    credentials: { present: false, cloud: false, rclone: false, notify: false },
    settingsGroups: [],
    placementDefaults: 3,
    copyRules: null,
    newTargets: [],
    ...over,
  };
}

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
  afterEach(cleanup);

  it("counts the defaults and rules of the file, or says it carries none", async () => {
    file.summary = summary({});
    await pickFile();
    expect(valueOf("Placement defaults")).toBe("3");
    expect(valueOf("Copy rules")).toBe("not in the file, stays as it is");
  });

  it("asks what each new target receives and writes the ticked exclusions after the import", async () => {
    file.summary = summary({ newTargets: [hetzner] });
    const seen = vi.fn();
    const off = subscribePlacement(seen);
    const applyImport = await pickFile();
    const block = screen.getByText("New target Hetzner for VMs").parentElement as HTMLElement;
    expect(block.textContent).toContain("Items and project folders: 15");
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
