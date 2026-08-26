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
