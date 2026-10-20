// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { CloudCredSetInfo } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

const SETS: CloudCredSetInfo[] = [
  {
    id: "hetzner", name: "Hetzner S3", s3KeyId: "H3TZ", s3Region: "fsn1", restUser: "", s3StorageClass: "",
    s3SecretSet: true, restPasswordSet: false, default: true, usedBy: [{ id: "destination:h", name: "Hetzner S3" }],
  },
  {
    id: "wasabi", name: "Wasabi", s3KeyId: "WSB4", s3Region: "", restUser: "", s3StorageClass: "",
    s3SecretSet: true, restPasswordSet: false, usedBy: [{ id: "destination:w", name: "Wasabi box" }, { id: "target:w2", name: "Wasabi too" }],
  },
  { id: "nas", name: "Old NAS", s3KeyId: "", s3Region: "", restUser: "backup", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true, usedBy: [] },
  { id: "kept", name: "NAS direct", keptFor: "r1", s3KeyId: "", s3Region: "", restUser: "bv", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true },
];

let sets = SETS;

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => sets,
  credSetsChanged: () => {},
}));
vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getCloudCredSets: vi.fn(),
  setCloudCredSets: vi.fn(),
  makeDefaultSignIn: vi.fn(),
  dismissSignInsNote: vi.fn(),
  testSignIn: vi.fn(),
}));

const api = await import("../../lib/api");
const getCloudCredSets = vi.mocked(api.getCloudCredSets);
const setCloudCredSets = vi.mocked(api.setCloudCredSets);
const makeDefaultSignIn = vi.mocked(api.makeDefaultSignIn);
const dismissSignInsNote = vi.mocked(api.dismissSignInsNote);
const testSignIn = vi.mocked(api.testSignIn);
const { SignInsCard } = await import("./SignInsCard");

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <SignInsCard hueIndex={0} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

function row(name: string): HTMLElement {
  return screen.getByText(name).closest("li") as HTMLElement;
}

function removeButton(name: string): HTMLButtonElement {
  return within(row(name)).getByRole("button", { name: /^Remove/ }) as HTMLButtonElement;
}

async function edit(name: string) {
  await act(async () => {
    fireEvent.click(within(row(name)).getByRole("button", { name: "Edit" }));
  });
  return screen.getByRole("dialog");
}

beforeEach(() => {
  sets = SETS;
  getCloudCredSets.mockResolvedValue({ ok: true, sets: SETS, note: null });
  setCloudCredSets.mockResolvedValue({ ok: true });
  makeDefaultSignIn.mockResolvedValue({ ok: true });
  dismissSignInsNote.mockResolvedValue({ ok: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("marks the default and says who signs in with each sign-in", async () => {
  await renderCard();
  expect(row("Hetzner S3").textContent).toContain("Default");
  expect(row("Hetzner S3").textContent).toContain("Also used for addresses typed in by hand");
  expect(screen.getByText("Used by Wasabi box and Wasabi too")).toBeTruthy();
  expect(row("Old NAS").textContent).toContain("Not used by any storage location");
  expect(screen.getByText("NAS direct (kept credentials)")).toBeTruthy();
});

it("adds no sign-in of its own", async () => {
  await renderCard();
  expect(screen.queryByRole("button", { name: /Add sign-in/ })).toBeNull();
});

it("shows once what became of the shared credentials", async () => {
  getCloudCredSets.mockResolvedValue({ ok: true, sets: SETS, note: { setName: "Hetzner S3", merged: true, duplicates: 0, linked: [] } });
  await renderCard();

  const note = screen.getByRole("status");
  expect(note.textContent).toContain("The shared cloud credentials are a saved sign-in now.");
  expect(screen.getByLabelText(/They had the same keys as Hetzner S3, so the two became one, and Hetzner S3 is the default\./)).toBeTruthy();

  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Got it" }));
  });
  expect(dismissSignInsNote).toHaveBeenCalled();
  expect(screen.queryByRole("status")).toBeNull();
});

it("keeps a sign-in a storage location uses and says who uses it", async () => {
  await renderCard();
  expect(removeButton("Hetzner S3").disabled).toBe(true);
  expect(removeButton("Wasabi").disabled).toBe(true);
  expect(screen.getAllByLabelText("Wasabi box and Wasabi too still signs in with it. Pick another sign-in there first.").length).toBeGreaterThan(0);
  expect(removeButton("Old NAS").disabled).toBe(false);
});

it("keeps the default while another sign-in could take its place", async () => {
  sets = [{ ...SETS[0], usedBy: [] }, SETS[2]];
  await renderCard();
  expect(removeButton("Hetzner S3").disabled).toBe(true);
  expect(screen.getAllByLabelText("This is the default sign-in. Make another one the default first.").length).toBeGreaterThan(0);
});

it("lets the last sign-in go with the default mark", async () => {
  sets = [{ ...SETS[0], usedBy: [] }];
  await renderCard();
  await act(async () => {
    fireEvent.click(removeButton("Hetzner S3"));
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Confirm remove" }));
  });
  expect(setCloudCredSets).toHaveBeenCalledWith([]);
});

it("removes an unused sign-in on the second click", async () => {
  await renderCard();
  await act(async () => {
    fireEvent.click(removeButton("Old NAS"));
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Confirm remove" }));
  });
  expect(setCloudCredSets).toHaveBeenCalledWith(expect.not.arrayContaining([expect.objectContaining({ id: "nas" })]));
});

it("saves an edited sign-in from its window without a test", async () => {
  await renderCard();
  const window = await edit("Old NAS");
  fireEvent.change(within(window).getByLabelText("Name"), { target: { value: "New NAS" } });
  await act(async () => {
    fireEvent.click(within(window).getByRole("button", { name: "Save" }));
  });

  const sent = setCloudCredSets.mock.calls[0][0];
  expect(sent.find((s) => s.id === "nas")?.name).toBe("New NAS");
  expect(sent.map((s) => s.id).sort()).toEqual(["hetzner", "kept", "nas", "wasabi"]);
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("tests the values as typed at every storage location that uses the sign-in", async () => {
  testSignIn.mockResolvedValue({
    ok: true,
    locations: [
      { id: "destination:w", name: "Wasabi box", reachable: true, initialized: true },
      { id: "target:w2", name: "Wasabi too", reachable: false, initialized: false, error: "403 Forbidden" },
    ],
  });
  await renderCard();
  const window = await edit("Wasabi");
  fireEvent.change(within(window).getByLabelText("Access key ID"), { target: { value: "NEWKEY" } });
  await act(async () => {
    fireEvent.click(within(window).getByRole("button", { name: "Test" }));
  });

  expect(testSignIn).toHaveBeenCalledWith(expect.objectContaining({ id: "wasabi", s3KeyId: "NEWKEY" }));
  expect(within(window).getByText("Wasabi box").closest("li")?.textContent).toContain("Connected");
  expect(within(window).getByText("Wasabi too").closest("li")?.textContent).toContain("403 Forbidden");
  expect(setCloudCredSets).not.toHaveBeenCalled();
});

it("has nothing to test a sign-in no storage location uses against", async () => {
  await renderCard();
  const window = await edit("Old NAS");
  expect((within(window).getByRole("button", { name: /^Test/ }) as HTMLButtonElement).disabled).toBe(true);
  expect(within(window).getAllByLabelText("No storage location uses this sign-in, so there is nothing to test it against.").length).toBeGreaterThan(0);
});

it("makes another sign-in the default in one request", async () => {
  await renderCard();
  expect(within(row("Hetzner S3")).queryByRole("button", { name: "Make default" })).toBeNull();
  await act(async () => {
    fireEvent.click(within(row("Wasabi")).getByRole("button", { name: "Make default" }));
  });
  expect(makeDefaultSignIn).toHaveBeenCalledWith("wasabi");
  expect(setCloudCredSets).not.toHaveBeenCalled();
});
