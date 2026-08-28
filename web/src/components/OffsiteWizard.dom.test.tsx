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

const updateTargetCalls: { id: string; credsRef: string }[] = [];
let listTargetCalls = 0;
let saveCalls = 0;

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
    setCloud: () => Promise.resolve({ ok: true }),
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

// The REST username/password FIELDS render only for a rest: repo (s3/rclone
// carry their own auth), but since #182 the credential-SET selector renders for
// every remote backend — so the repo URL is a parameter here.
const REST_REPO = "rest:http://192.168.20.199:8000/containers";

function settingsWith(repo: string = REST_REPO): Settings {
  return {
    containersOffsite: repo,
    containersOffsiteImmutable: false,
    containersPath: "backups/containers",
    offsiteGrowthBudgetGB: 0,
  } as unknown as Settings;
}

function Harness({ repo }: { repo: string }) {
  const { t } = useT();
  return (
    <OffsiteWizard
      domain="containers"
      settings={settingsWith(repo)}
      setSettings={() => {}}
      save={() => { saveCalls++; return Promise.resolve(true); }}
      t={t}
    />
  );
}

// Self-backup ("config"): the domain whose settings key is configOffsite, not
// containersOffsite, and which the wizard used to have no map entry for.
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

// The REST password debounce test that used to live here is gone with the
// fields it covered: the wizard no longer edits the shared credentials at all
// (#176, kramttocs). The same defect and the same fix are still pinned for the
// component that DOES own them, in Settings.cloudCard.dom.test.tsx.

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

it("never shows credential FIELDS, only the selector, whichever set is chosen", async () => {
  await renderWizard();

  // Shared is the default, and even then the shared username and password are
  // not editable here: the wizard picks credentials, it does not define them.
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

// Saving a repo for the first time is what CREATES the destination row the
// selector binds to, and the keystroke that triggered the save happened before
// it existed. Without a re-read after the save the selector stays hidden until
// the wizard is next opened - seen live during a first-time setup.
it("re-reads the destination after saving the repository", async () => {
  await renderWizard();

  const repo = screen.getByLabelText(/Off-site-Repository-URL|Off-site repository URL/) as HTMLInputElement;
  await act(async () => {
    fireEvent.change(repo, { target: { value: "rest:http://192.0.2.99:8000/new" } });
  });
  // Typing alone does not re-read here (this harness keeps settings constant),
  // so pin the count now and require the SAVE itself to produce the read.
  const beforeSave = listTargetCalls;

  await act(async () => {
    await vi.advanceTimersByTimeAsync(900);
  });
  // The re-read hangs off the save promise, so let the microtasks settle.
  await act(async () => {
    await Promise.resolve();
  });

  expect(saveCalls).toBe(1);
  expect(listTargetCalls).toBe(beforeSave + 1);
});

// ---------------------------------------------------------------------------
// #182 (manilx): "I can't set s3 credentials when i use s3 as main path and
// offsite". The whole credentials block, selector included, used to be gated on
// urlBackend === "rest", on the reasoning that only REST needs a username and
// password here. True of the FIELDS, wrong for the SELECTOR: a credential set
// carries S3 keys too, and the replication path resolves whichever set a
// destination names. So an S3 user was never offered the choice at all and
// could only ever run on the shared credentials.
// ---------------------------------------------------------------------------

it("offers the credential selector for an S3 destination, not just a REST one", async () => {
  await renderWizard("s3:https://s3.eu-central-1.example/bucket");

  const select = screen.getByLabelText(/Credentials|Zugangsdaten/) as HTMLSelectElement;
  expect(select).toBeTruthy();
  expect([...select.options].map((o) => o.textContent)).toContain("Backblaze");

  // The REST-only fields must stay away: S3 keys live in the credential set or
  // the shared cloud credentials, never in this block (#131).
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_PASSWORD/)).toBeNull();
});

it("shows no credentials block at all for a local path", async () => {
  await renderWizard("backups/containers-copy");

  expect(screen.queryByLabelText(/Credentials|Zugangsdaten/)).toBeNull();
  expect(screen.queryByLabelText(/RESTIC_REST_USERNAME/)).toBeNull();
});

// ---------------------------------------------------------------------------
// #182 (manilx), reported against v8.3.0: opening the wizard for self-backup
// rendered a blank page and needed a browser refresh. #176 had added the domain
// to the off-site tab, but this file still carried its own four-domain copy of
// the type plus an `as` cast asserting that off-site mode "never receives
// config". So REPO_KEY had no entry, repoKey was undefined, settings[undefined]
// was undefined, and inferBackend called .trim() on it during the first render.
//
// The cast is gone (a missing map entry is a compile error now), but the render
// itself is pinned here: a type-level guarantee does not prove the component
// mounts.
// ---------------------------------------------------------------------------

it("renders for the self-backup domain instead of blanking the page", async () => {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SelfBackupHarness />
        </ToastProvider>
      </I18nProvider>
    );
  });

  // Reaching step 3 at all means the first render survived: this is exactly
  // where the crash used to happen, on inferBackend(settings[repoKey]).
  expect(screen.getByLabelText(/Credentials|Zugangsdaten/)).toBeTruthy();
});
