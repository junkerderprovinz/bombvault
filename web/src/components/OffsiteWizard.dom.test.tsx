// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

const updateTargetCalls: { id: string; credsRef: string }[] = [];
let listTargetCalls = 0;
let saveCalls = 0;

// The domain's primary off-site destination (sortOrder 0), whose credsRef the
// wizard's credential selector edits.
const PRIMARY_TARGET = {
  id: "tgt-primary",
  domain: "containers",
  name: "Primary",
  repo: "rest:http://192.168.20.199:8000/containers",
  credsRef: "",
  storageClass: "",
  immutable: false,
  schedule: "",
  retentionKeepLast: 0,
  retentionKeepDaily: 0,
  retentionKeepWeekly: 0,
  retentionKeepMonthly: 0,
  limitUpload: 0,
  limitDownload: 0,
  growthBudgetGb: 0,
  enabled: true,
  createdAt: 0,
  sortOrder: 0,
};

vi.mock("../lib/useCloudCredSets", () => ({
  useCloudCredSets: () => [{ id: "set-a", name: "Backblaze" }],
}));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getCloud: () =>
      Promise.resolve({
        ok: true,
        s3KeyId: "AKIAEXAMPLE",
        s3Region: "eu-central-1",
        restUser: "bombvault",
        s3StorageClass: "",
        s3SecretSet: false,
        restPasswordSet: false,
      }),
    setCloud: () => Promise.resolve({ ok: true }),
    getNewTargetPreview: () =>
      Promise.resolve({
        ok: true,
        preview: { items: 3, formerlyExcluded: [], defaultExcludes: false, snapshots: 40, bytes: null, unreadable: [] },
      }),
    listOffsiteTargets: () => {
      listTargetCalls++;
      return Promise.resolve({ ok: true, targets: [PRIMARY_TARGET] });
    },
    updateOffsiteTarget: (id: string, target: { credsRef: string }) => {
      updateTargetCalls.push({ id, credsRef: target.credsRef });
      return Promise.resolve({ ok: true, target: { ...PRIMARY_TARGET, credsRef: target.credsRef } });
    },
  };
});

const { OffsiteWizard } = await import("./OffsiteWizard");

// The credential-set selector renders for every remote backend, so the repo
// URL is a parameter.
const REST_REPO = "rest:http://192.168.20.199:8000/containers";

function settingsWith(repo: string = REST_REPO): Settings {
  return {
    containersOffsite: repo,
    containersOffsiteImmutable: false,
    containersPath: "backups/containers",
    offsiteGrowthBudgetGB: 0,
  } as unknown as Settings;
}

// The page keeps the settings and a save writes the field back into them, which
// is what makes a saved repo URL reach the wizard again.
function Harness({ repo }: { repo: string }) {
  const { t } = useT();
  const [settings, setSettings] = useState<Settings | null>(settingsWith(repo));
  return (
    <OffsiteWizard
      domain="containers"
      settings={settings as Settings}
      setSettings={setSettings}
      save={(patch) => {
        saveCalls++;
        setSettings((prev) => (prev ? { ...prev, ...patch } : prev));
        return Promise.resolve(true);
      }}
      t={t}
    />
  );
}

// Self-backup ("config") keeps its repo in configOffsite.
function SelfBackupHarness() {
  const { t } = useT();
  return (
    <OffsiteWizard
      domain="config"
      settings={{
        configOffsite: "s3:https://offsite.example/selfbackup",
        configOffsiteImmutable: false,
        configPath: "user/bombvault/config",
        offsiteGrowthBudgetGB: 0,
      } as unknown as Settings}
      setSettings={() => {}}
      save={() => { saveCalls++; return Promise.resolve(true); }}
      t={t}
    />
  );
}

async function renderWizard(repo: string = REST_REPO) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness repo={repo} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  updateTargetCalls.length = 0;
  listTargetCalls = 0;
  saveCalls = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// The wizard chooses credentials per destination and never edits the shared
// ones.
it("writes the chosen credential set onto this domain's primary destination", async () => {
  await renderWizard();

  // The picker is the app's own listbox: it is opened and an option clicked.
  const picker = screen.getByRole("combobox", { name: /Credentials|Zugangsdaten/ });
  expect(picker.textContent).toContain("Shared");

  await act(async () => {
    fireEvent.click(picker);
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("option", { name: "Backblaze" }));
  });

  expect(updateTargetCalls).toEqual([{ id: "tgt-primary", credsRef: "set-a" }]);
});

it("shows only the selector, never credential fields, whichever set is chosen", async () => {
  await renderWizard();

  // Shared is the default, and even then its username and password are not
  // editable here.
  expect(screen.getByLabelText(/Credentials|Zugangsdaten/)).toBeTruthy();
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_PASSWORD/)).toBeNull();

  await act(async () => {
    fireEvent.change(screen.getByLabelText(/Credentials|Zugangsdaten/), {
      target: { value: "set-a" },
    });
  });

  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_PASSWORD/)).toBeNull();
});

// Saving a repo for the first time is what creates the destination row the
// selector binds to, so the save itself has to produce a read of it.
it("saves the repository only when told to, after the question, and reads the destination again", async () => {
  await renderWizard();

  const repo = screen.getByLabelText(/Off-site-Repository-URL|Off-site repository URL/) as HTMLInputElement;
  await act(async () => {
    fireEvent.change(repo, { target: { value: "rest:http://192.0.2.99:8000/new" } });
  });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(900);
  });
  expect(saveCalls).toBe(0);
  // This harness keeps settings constant, so typing alone does not re-read.
  const beforeSave = listTargetCalls;

  await act(async () => {
    fireEvent.keyDown(repo, { key: "Enter" });
  });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0);
  });
  expect(screen.getByRole("dialog").textContent).toContain("The new location of Primary receives the whole history.");

  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));
  });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0);
  });
  expect(saveCalls).toBe(1);
  expect(listTargetCalls).toBe(beforeSave + 1);
});

it("offers the credential selector for an S3 destination, not just a REST one", async () => {
  await renderWizard("s3:https://s3.eu-central-1.example/bucket");

  const picker = screen.getByRole("combobox", { name: /Credentials|Zugangsdaten/ });
  expect(picker).toBeTruthy();
  // Every option is carried in the trigger (that is what keeps its width from
  // moving), so the set is offered here whether the list is open or not.
  expect(picker.textContent).toContain("Backblaze");

  // S3 keys live in the credential set or the shared cloud credentials.
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_PASSWORD/)).toBeNull();
});

it("shows no credentials block at all for a local path", async () => {
  await renderWizard("backups/containers-copy");

  expect(screen.queryByLabelText(/Credentials|Zugangsdaten/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
});

it("renders for the self-backup domain", async () => {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SelfBackupHarness />
        </ToastProvider>
      </I18nProvider>
    );
  });

  // Step 3 is only reached if the first render, which runs
  // inferBackend(settings[repoKey]), got through.
  expect(screen.getByLabelText(/Credentials|Zugangsdaten/)).toBeTruthy();
});

// With --private-repos each htpasswd user only reaches the tree under its own
// name, so the URL's first path segment has to be the user.
it("says so when the URL's user segment is not the user it signs in as", async () => {
  // The mocked shared credentials sign in as "bombvault" (see getCloud above).
  await renderWizard("rest:http://192.168.20.199:8000/bombvault-containers/containers");

  const hint = screen.getByText(/bombvault-containers/);
  expect(hint.textContent).toContain("bombvault-containers");
  expect(hint.textContent).toContain("bombvault");
});

it("stays quiet when the two agree, and when there is no user segment", async () => {
  await renderWizard("rest:http://192.168.20.199:8000/bombvault/containers");
  expect(screen.queryByText(/--private-repos/)).toBeNull();
  cleanup();

  // One segment is an ordinary path on a server without --private-repos, where
  // a 401 means a wrong password: a hint here would be a wrong steer.
  await renderWizard("rest:http://192.168.20.199:8000/containers");
  expect(screen.queryByText(/--private-repos/)).toBeNull();
});
