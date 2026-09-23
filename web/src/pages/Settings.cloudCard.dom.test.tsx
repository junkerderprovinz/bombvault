// @vitest-environment jsdom
// The write-only secrets in CloudCard (AWS_SECRET_ACCESS_KEY,
// RESTIC_REST_PASSWORD) save on an 800 ms debounce, and blank means "keep the
// stored one". If the field were cleared after a save, a pause mid-secret
// would send the rest into an empty field and store only that fragment, with
// no error anywhere. The tests append to the field the way a keyboard does,
// because a test that sets the whole value passes either way.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

const setCloudCalls: Record<string, string>[] = [];
// Flipped by the load-failure test below; the mock reads it on every call.
let cloudLoadFails = false;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getCloud: () =>
      Promise.resolve({
        ok: !cloudLoadFails,
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

// Imported after vi.mock so the component picks up the mocked client.
const { CloudCard } = await import("./settings/CloudCard");

function Harness() {
  const { t } = useT();
  return <CloudCard t={t} hueIndex={0} />;
}

async function renderCard() {
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

/** Appends `chunk` to what the field holds, the way a keyboard does. */
function typeMore(field: HTMLInputElement, chunk: string) {
  fireEvent.change(field, { target: { value: field.value + chunk } });
}

/** Lets the 800 ms debounce fire and its save promise settle. */
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
  cloudLoadFails = false;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("CloudCard secret entry", () => {
  it("stores the whole S3 secret when typing pauses part-way through", async () => {
    await renderCard();
    const field = screen.getByLabelText(/AWS_SECRET_ACCESS_KEY/) as HTMLInputElement;

    // Half the key, then a pause long enough for the debounce to save it.
    typeMore(field, "wJalrXUtnFEMI");
    await pauseTyping();
    expect(lastSave().s3Secret).toBe("wJalrXUtnFEMI");

    // Blanked here, the rest of the key would land in an empty field.
    expect(field.value).toBe("wJalrXUtnFEMI");

    // Then the rest of the key.
    typeMore(field, "/K7MDENG/bPxRfiCYEXAMPLEKEY");
    await pauseTyping();

    expect(field.value).toBe("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY");
    expect(lastSave().s3Secret).toBe("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY");
  });

  it("stores the whole REST password when typing pauses part-way through", async () => {
    await renderCard();
    const field = screen.getByLabelText(/RESTIC_REST_PASSWORD/) as HTMLInputElement;

    typeMore(field, "correct-horse");
    await pauseTyping();
    expect(lastSave().restPassword).toBe("correct-horse");
    expect(field.value).toBe("correct-horse");

    typeMore(field, "-battery-staple");
    await pauseTyping();

    expect(field.value).toBe("correct-horse-battery-staple");
    expect(lastSave().restPassword).toBe("correct-horse-battery-staple");
  });

  it("keeps the stored secret on the wire when a later edit touches another field", async () => {
    await renderCard();
    const secret = screen.getByLabelText(/AWS_SECRET_ACCESS_KEY/) as HTMLInputElement;
    typeMore(secret, "wJalrXUtnFEMI/K7MDENG");
    await pauseTyping();

    const region = screen.getByLabelText(/AWS_DEFAULT_REGION/) as HTMLInputElement;
    fireEvent.change(region, { target: { value: "us-east-1" } });
    await pauseTyping();

    // A blank secret would be harmless on its own, since the backend keeps the
    // stored one, but it would mean the field had been emptied.
    expect(lastSave().s3Region).toBe("us-east-1");
    expect(lastSave().s3Secret).toBe("wJalrXUtnFEMI/K7MDENG");
  });

  // setCloud replaces the whole config, so an edit on a card that failed to
  // load would post its empty fields over the stored ones. OffsiteWizard
  // guards the same way with cloudLoaded.
  it("saves nothing when the current config could not be loaded, and says so", async () => {
    cloudLoadFails = true;
    await renderCard();

    const region = screen.getByLabelText(/AWS_DEFAULT_REGION/) as HTMLInputElement;
    fireEvent.change(region, { target: { value: "us-east-1" } });
    await pauseTyping();

    expect(setCloudCalls).toHaveLength(0);
    // Shown twice: a standing line in the card and a toast for the refused
    // edit.
    expect(screen.getAllByText(/could not be loaded/i).length).toBeGreaterThan(0);
  });
});
