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
    s3SecretSet: true, restPasswordSet: false, usedBy: [{ id: "destination:w", name: "Wasabi box" }],
  },
  { id: "nas", name: "Old NAS", s3KeyId: "", s3Region: "", restUser: "backup", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true, usedBy: [] },
  { id: "kept", name: "NAS direct", keptFor: "r1", s3KeyId: "", s3Region: "", restUser: "bv", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true },
];

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => SETS,
  credSetsChanged: () => {},
}));
vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getCloudCredSets: vi.fn(),
  setCloudCredSets: vi.fn(),
  makeDefaultSignIn: vi.fn(),
  dismissSignInsNote: vi.fn(),
}));

const api = await import("../../lib/api");
const getCloudCredSets = vi.mocked(api.getCloudCredSets);
const setCloudCredSets = vi.mocked(api.setCloudCredSets);
const makeDefaultSignIn = vi.mocked(api.makeDefaultSignIn);
const dismissSignInsNote = vi.mocked(api.dismissSignInsNote);
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

beforeEach(() => {
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
  expect(screen.getByText("Used by Wasabi box")).toBeTruthy();
  expect(row("Old NAS").textContent).toContain("Not used by any storage location");
  expect(screen.getByText("NAS direct (kept credentials)")).toBeTruthy();
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

it("keeps the default and a sign-in a storage location uses", async () => {
  await renderCard();
  await act(async () => {
    fireEvent.click(screen.getAllByRole("button", { name: "Remove" })[0]);
    fireEvent.click(screen.getAllByRole("button", { name: "Remove" })[1]);
  });
  expect(await screen.findByText("This is the default sign-in. Make another one the default first.")).toBeTruthy();
  expect(await screen.findByText("Wasabi box still signs in with it. Pick another sign-in there first.")).toBeTruthy();
  expect(setCloudCredSets).not.toHaveBeenCalled();
});

it("removes an unused sign-in on the second click", async () => {
  await renderCard();
  await act(async () => {
    fireEvent.click(screen.getAllByRole("button", { name: "Remove" })[2]);
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Confirm remove" }));
  });
  expect(setCloudCredSets).toHaveBeenCalledWith(expect.not.arrayContaining([expect.objectContaining({ id: "nas" })]));
});

it("moves the default mark to a sign-in saved as the default", async () => {
  await renderCard();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Add sign-in" }));
  });
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Garage" } });
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: "Default sign-in" }));
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
  });

  const sent = setCloudCredSets.mock.calls[0][0];
  expect(sent.filter((s) => s.default).map((s) => s.name)).toEqual(["Garage"]);
  expect(sent.find((s) => s.id === "hetzner")?.default).toBe(false);
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
