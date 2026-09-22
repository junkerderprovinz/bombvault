// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { NamedRepo, OffsiteTarget, Settings } from "../lib/api";

const FIELD_TARGET: OffsiteTarget = {
  id: "t-f", domain: "containers", name: "B2", repo: "b2:bkt:containers", credsRef: "", storageClass: "",
  immutable: true, schedule: "", retentionKeepLast: 0, retentionKeepDaily: 0, retentionKeepWeekly: 0,
  retentionKeepMonthly: 0, limitUpload: 0, limitDownload: 0, growthBudgetGb: 0, enabled: true, createdAt: 1, sortOrder: 0,
};
const DIRECT: NamedRepo = {
  id: "d1", name: "B2 direct", repo: "b2:bkt:containers-direct", credsRef: "", storageClass: "", limitUpload: 0,
  limitDownload: 0, immutable: true, enabled: true, offPremises: false, inUse: 2, companionOf: "t-f", companionLost: false,
};

vi.mock("../lib/useCloudCredSets", () => ({ useCloudCredSets: () => [] }));
vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getCloud: () =>
    Promise.resolve({
      ok: true, s3KeyId: "", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: false, restPasswordSet: false,
    }),
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [FIELD_TARGET] }),
  listRepos: () => Promise.resolve({ ok: true, repos: [DIRECT] }),
}));

const { OffsiteWizard } = await import("./OffsiteWizard");

const saves: Partial<Settings>[] = [];

function Harness() {
  const { t } = useT();
  return (
    <OffsiteWizard
      domain="containers"
      settings={
        {
          containersOffsite: FIELD_TARGET.repo,
          containersOffsiteImmutable: true,
          containersPath: "backups/containers",
          offsiteGrowthBudgetGB: 0,
        } as unknown as Settings
      }
      setSettings={() => {}}
      save={(patch) => {
        saves.push(patch);
        return Promise.resolve(true);
      }}
      t={t}
    />
  );
}

async function renderWizard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    );
  });
  // The target and repository lists load after mount; the question needs both.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

async function clickAppendOnly() {
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: en["offsite.immutable"] }));
  });
}

beforeEach(() => {
  saves.length = 0;
});
afterEach(cleanup);

it("asks before append-only goes off while items keep their only copy in the direct repository", async () => {
  await renderWizard();
  await clickAppendOnly();
  expect((await screen.findByRole("dialog")).textContent).toContain(
    "Items whose only copy is in B2 direct: 2. Without append-only this box may delete from it. Save anyway?"
  );
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
  });
  expect(saves).toEqual([]);

  await clickAppendOnly();
  await act(async () => {
    fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
  });
  expect(saves).toEqual([{ containersOffsiteImmutable: false }]);
});
