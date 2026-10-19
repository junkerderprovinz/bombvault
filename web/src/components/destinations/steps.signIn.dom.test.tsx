// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { BackendOption, CloudCredSetInfo, Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import type { DestinationDraftState } from "./useDestinationDraft";

const SETS: CloudCredSetInfo[] = [
  { id: "nas", name: "NAS", s3KeyId: "", s3Region: "", restUser: "backup", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true },
  { id: "s3only", name: "Wasabi", s3KeyId: "WSB", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false },
];

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => SETS,
}));

const { SignInBody } = await import("./steps");

const plain = (name: string, secret = false): BackendOption => ({
  name, help: "", required: true, secret, password: secret, advanced: false, essential: true, default: "",
});

function draft(credsRef: string, setCredsRef: (id: string) => void): DestinationDraftState {
  return {
    picked: { id: "rest", name: "rest-server", route: "rest", group: "server" } as Provider,
    fields: { main: [plain("url"), plain("user"), plain("pass", true)], more: [] },
    settings: {},
    setSetting: () => {},
    name: "NAS",
    setName: () => {},
    credsRef,
    setCredsRef,
    test: { verdict: null },
  } as unknown as DestinationDraftState;
}

async function show(credsRef: string, setCredsRef = vi.fn()) {
  await act(async () => {
    render(
      <I18nProvider>
        <SignInBody draft={draft(credsRef, setCredsRef)} providerLabel="rest-server" />
      </I18nProvider>
    );
  });
  return setCredsRef;
}

afterEach(cleanup);

it("offers the saved sign-ins of the provider's kind instead of new keys", async () => {
  const setCredsRef = await show("");
  expect(screen.getByLabelText("User name")).toBeTruthy();

  await act(async () => {
    fireEvent.click(screen.getByRole("tab", { name: "Saved sign-in" }));
  });
  expect(setCredsRef).toHaveBeenCalledWith("nas");
});

it("hides the key fields while a saved sign-in is picked", async () => {
  await show("nas");
  expect(screen.queryByLabelText("User name")).toBeNull();
  expect(screen.queryByLabelText("Password")).toBeNull();
  expect(screen.getByLabelText("Address")).toBeTruthy();
  const picker = screen.getByRole("combobox", { name: "Saved sign-in" });
  expect(picker.textContent).toContain("NAS");
  expect(picker.textContent).not.toContain("Wasabi");
  expect(screen.getByText("Signs in as: backup")).toBeTruthy();
});
