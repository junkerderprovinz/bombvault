// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { DraftQuestion, Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import type { DestinationDraftState } from "./useDestinationDraft";

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => [],
}));

const { SignInBody } = await import("./steps");

const cloud = (id: string, backend: string, auth: Provider["auth"]): Provider =>
  ({ id, name: id, backend, route: "rclone", group: "cloud", auth }) as Provider;

function draft(picked: Provider, settings: Record<string, string> = {}, question: DraftQuestion | null = null, setSetting = vi.fn()) {
  return {
    picked,
    fields: { main: [], more: [] },
    settings,
    setSetting,
    name: picked.name,
    setName: () => {},
    credsRef: "",
    setCredsRef: () => {},
    question,
    test: { verdict: null },
  } as unknown as DestinationDraftState;
}

async function show(d: DestinationDraftState) {
  await act(async () => {
    render(
      <I18nProvider>
        <SignInBody draft={d} providerLabel={d.picked?.name ?? ""} />
      </I18nProvider>
    );
  });
}

afterEach(cleanup);

it("gives Zoho its region in the command, once there is one", async () => {
  const zoho = cloud("zoho", "zoho", "token");
  await show(draft(zoho));
  expect(screen.queryByText(/rclone authorize/)).toBeNull();
  expect(screen.getByText(/Choose the region below first/)).toBeTruthy();

  cleanup();
  await show(draft(zoho, { region: "eu" }));
  expect(screen.getByText(`rclone authorize "zoho" "${btoa('{"region":"eu"}').replace(/=+$/, "")}"`)).toBeTruthy();
});

it("asks rclone for ShareFile's address along with the token", async () => {
  await show(draft(cloud("sharefile", "sharefile", "token")));
  expect(screen.getByText('rclone authorize "sharefile" "e30"')).toBeTruthy();
});

it("sends Jottacloud to its login token page instead of rclone authorize", async () => {
  await show(draft(cloud("jottacloud", "jottacloud", "logintoken")));
  expect(screen.queryByText(/rclone authorize/)).toBeNull();
  expect(screen.getByRole("link", { name: "https://www.jottacloud.com/web/secure" })).toBeTruthy();
});

it("says how iCloud Drive gets its trust token", async () => {
  await show(draft(cloud("icloud", "iclouddrive", "login")));
  expect(screen.getByText(/copy trust_token and cookies/)).toBeTruthy();
});

it("lets the drive rclone's setup asks about be picked", async () => {
  const setSetting = vi.fn();
  const question = { name: "config_driveid", choices: [{ value: "a", help: "OneDrive (personal)" }, { value: "b", help: "Team (business)" }] };
  await show(draft(cloud("onedrive", "onedrive", "token"), {}, question, setSetting));

  await act(async () => {
    fireEvent.click(screen.getByRole("combobox", { name: "Drive" }));
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("option", { name: "Team (business)" }));
  });
  expect(setSetting).toHaveBeenCalledWith("config_driveid", "b");
});
