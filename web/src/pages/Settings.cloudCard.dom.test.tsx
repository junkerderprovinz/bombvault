// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// CloudCard — a secret typed into an auto-saving field must arrive whole.
//
// The two fields here (AWS_SECRET_ACCESS_KEY, RESTIC_REST_PASSWORD) are
// write-only: the GET never echoes them, so blank means "keep the stored one".
// They used to be committed by a Save button, which blanked them afterwards so
// the placeholder could switch to "already set". Typing was finished by then,
// so that was correct.
//
// The Speichern-Button sweep replaced the click with an 800 ms debounce and
// carried the blanking across unchanged. That turns the field into a shredder:
// pause for a moment mid-secret (glancing at the key on another screen is the
// normal way to enter a 40-character one), the timer fires, the input is wiped
// under the cursor, the rest of the secret goes into an empty field, and 800 ms
// later THAT fragment is saved over the real credential. Nothing reports an
// error — "blank = keep" cannot tell a fragment from a whole key — so the next
// off-site backup simply fails to authenticate.
//
// The test types the way a person does: append to whatever the field currently
// holds. That is what makes the blanking visible at all; a test that always
// sets the whole value would pass either way.
//
// jsdom opted in explicitly (controlled inputs + timers) — see
// Selector.dom.test.tsx's header for this repo's naming convention.
// ---------------------------------------------------------------------------
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

// Imported AFTER vi.mock so the component picks up the mocked client.
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

/** Types `chunk` at the END of whatever the field already holds, exactly as a
 *  keyboard does — never replacing the field's contents. */
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

    // The field must still hold what was typed — this is the whole bug: if it
    // was blanked, the rest of the key below lands in an empty field.
    expect(field.value).toBe("wJalrXUtnFEMI");

    // …and the rest of the key.
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

  it("keeps the stored secret on the wire when a LATER edit touches another field", async () => {
    await renderCard();
    const secret = screen.getByLabelText(/AWS_SECRET_ACCESS_KEY/) as HTMLInputElement;
    typeMore(secret, "wJalrXUtnFEMI/K7MDENG");
    await pauseTyping();

    const region = screen.getByLabelText(/AWS_DEFAULT_REGION/) as HTMLInputElement;
    fireEvent.change(region, { target: { value: "us-east-1" } });
    await pauseTyping();

    // The region save carries the secret as typed. (A blank here would be
    // harmless on its own — the backend keeps the stored one — but it would
    // mean the field had been emptied, which is what breaks the case above.)
    expect(lastSave().s3Region).toBe("us-east-1");
    expect(lastSave().s3Secret).toBe("wJalrXUtnFEMI/K7MDENG");
  });

  // setCloud is a FULL REPLACE. Before the loaded gate, a non-ok GET left the
  // card at its empty initial state with nothing on screen to say so, and the
  // next edit posted that emptiness as the new truth: stored AWS key id,
  // region, REST user and storage class gone, with a "saved" toast on top.
  // The same shape exists in OffsiteWizard, where the guard has been there all
  // along as `cloudLoaded`.
  it("saves nothing when the current config could not be loaded, and says so", async () => {
    cloudLoadFails = true;
    await renderCard();

    const region = screen.getByLabelText(/AWS_DEFAULT_REGION/) as HTMLInputElement;
    fireEvent.change(region, { target: { value: "us-east-1" } });
    await pauseTyping();

    expect(setCloudCalls).toHaveLength(0);
    // And the failure is visible rather than a silently dead card. It shows
    // twice on purpose: a standing line in the card (the card stays refusing)
    // and a toast for the click that was just refused.
    expect(screen.getAllByText(/could not be loaded/i).length).toBeGreaterThan(0);
  });
});
