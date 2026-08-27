// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// OffsiteWizard — the second copy of the auto-saving secret field.
//
// Step 3's RESTIC_REST_PASSWORD writes through the same cloud-credential
// endpoint CloudCard uses, with the same write-only "blank = keep the stored
// one" contract, and it inherited the same post-save blanking from the "Save
// credentials" button the Speichern-Button sweep deleted. On a debounce that
// blanking is destructive: the timer fires 800 ms into a pause mid-password,
// the field is emptied under the cursor, the rest is typed into an empty field
// and overwrites the stored credential with a fragment.
//
// Same defect, same fix, its own test — the two components share no code, so
// only a test each keeps them from drifting apart again. See
// Settings.cloudCard.dom.test.tsx for the full story.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

const setCloudCalls: Record<string, string>[] = [];
const updateTargetCalls: { id: string; credsRef: string }[] = [];

// The domain's PRIMARY off-site destination: the row whose credsRef the
// wizard's credential selector edits (#176). sortOrder 0 is what marks it.
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
    setCloud: (c: Record<string, string>) => {
      setCloudCalls.push({ ...c });
      return Promise.resolve({ ok: true });
    },
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [PRIMARY_TARGET] }),
    updateOffsiteTarget: (id: string, target: { credsRef: string }) => {
      updateTargetCalls.push({ id, credsRef: target.credsRef });
      return Promise.resolve({ ok: true, target: { ...PRIMARY_TARGET, credsRef: target.credsRef } });
    },
  };
});

const { OffsiteWizard } = await import("./OffsiteWizard");

// The credentials block renders only for a rest: repo (rclone/s3 carry their
// own auth), so the domain's off-site repo has to be one.
function settingsWith(): Settings {
  return {
    containersOffsite: "rest:http://192.168.20.199:8000/containers",
    containersOffsiteImmutable: false,
    containersPath: "backups/containers",
    offsiteGrowthBudgetGB: 0,
  } as unknown as Settings;
}

function Harness() {
  const { t } = useT();
  return (
    <OffsiteWizard
      domain="containers"
      settings={settingsWith()}
      setSettings={() => {}}
      save={() => Promise.resolve(true)}
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
}

function typeMore(field: HTMLInputElement, chunk: string) {
  fireEvent.change(field, { target: { value: field.value + chunk } });
}

async function pauseTyping() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(900);
  });
}

function lastSave(): Record<string, string> {
  const call = setCloudCalls.at(-1);
  if (!call) throw new Error("setCloud was never called");
  return call;
}

beforeEach(() => {
  vi.useFakeTimers();
  setCloudCalls.length = 0;
  updateTargetCalls.length = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

it("stores the whole REST password when typing pauses part-way through", async () => {
  await renderWizard();
  const field = screen.getByLabelText(/RESTIC_REST_PASSWORD/) as HTMLInputElement;

  typeMore(field, "9f3a1c");
  await pauseTyping();
  expect(lastSave().restPassword).toBe("9f3a1c");
  // If the field were blanked here, the rest of the password below would land
  // in an empty field and be saved over the real one.
  expect(field.value).toBe("9f3a1c");

  typeMore(field, "8e2b7d4a");
  await pauseTyping();

  expect(field.value).toBe("9f3a1c8e2b7d4a");
  expect(lastSave().restPassword).toBe("9f3a1c8e2b7d4a");
});

// ---------------------------------------------------------------------------
// #176 (kramttocs): the credentials step is opened per domain, but its fields
// write the SHARED cloud credentials, so filling them in under Containers also
// rewrote every other domain's. The plumbing for per-destination credentials
// already existed (each destination carries a credsRef the replication path
// resolves, and the primary destination is such a row) — the wizard simply had
// no control that set it. These two tests pin the control and, just as
// importantly, that choosing a set STOPS the shared fields from being offered,
// since leaving them visible is what made the two look like the same thing.
// ---------------------------------------------------------------------------
it("writes the chosen credential set onto this domain's primary destination", async () => {
  await renderWizard();

  const select = screen.getByLabelText(/Credentials|Zugangsdaten/) as HTMLSelectElement;
  expect(select.value).toBe(""); // shared by default, as before

  await act(async () => {
    fireEvent.change(select, { target: { value: "set-a" } });
  });

  expect(updateTargetCalls).toEqual([{ id: "tgt-primary", credsRef: "set-a" }]);
});

it("hides the shared credential fields once a set is chosen", async () => {
  await renderWizard();

  // Shared by default: the fields are the shared ones and are offered.
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).not.toBeNull();

  await act(async () => {
    fireEvent.change(screen.getByLabelText(/Credentials|Zugangsdaten/), {
      target: { value: "set-a" },
    });
  });

  // With a set selected they must be gone: editing them would silently change
  // every other destination that still uses the shared set.
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_PASSWORD/)).toBeNull();
});
