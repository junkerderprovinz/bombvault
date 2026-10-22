// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { BackendOption, Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import type { DestinationDraftState } from "./useDestinationDraft";

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => [],
}));

const { FolderBody, SignInBody } = await import("./steps");

const option = (name: string, help = ""): BackendOption => ({
  name, help, required: false, secret: false, password: false, advanced: false, essential: true, default: "",
  examples: name === "region" ? [{ value: "", help: "Use this if unsure.", provider: "Other" }] : undefined,
});

function draft(picked: Provider, extra: Partial<DestinationDraftState> = {}): DestinationDraftState {
  return {
    picked,
    fields: { main: [option("region", "Region to connect to."), option("endpoint")], more: [] },
    settings: {},
    setSetting: () => {},
    name: picked.name,
    setName: () => {},
    credsRef: "",
    setCredsRef: () => {},
    bucket: "",
    setBucket: () => {},
    test: { verdict: null },
    browsable: true,
    dir: ["bucket"],
    folders: [],
    free: null,
    newFolder: null,
    needsBucket: false,
    location: "s3:https://example/bucket",
    storageClass: "",
    setStorageClass: () => {},
    ...extra,
  } as unknown as DestinationDraftState;
}

const s3 = (id: string, provider: string, storageClasses?: string[]): Provider => ({
  id, name: id, backend: "s3", group: "storage", route: "s3", preset: { provider }, storageClasses,
});

async function show(node: React.ReactNode) {
  await act(async () => {
    render(<I18nProvider>{node}</I18nProvider>);
  });
}

afterEach(cleanup);

it("offers only the storage classes the provider takes", async () => {
  await show(<FolderBody draft={draft(s3("r2", "Cloudflare", ["STANDARD", "STANDARD_IA"]))} />);
  const picker = screen.getByRole("combobox", { name: "(provider default)" });
  expect(picker.textContent).toContain("STANDARD_IA");
  expect(picker.textContent).not.toContain("GLACIER_IR");
});

it("offers no storage class where the provider takes none", async () => {
  await show(<FolderBody draft={draft(s3("silo", "Minio", []))} />);
  expect(screen.queryByRole("combobox", { name: "(provider default)" })).toBeNull();
});

it("asks for a bucket on an S3 service, from the bucket of a pasted address", async () => {
  await show(<SignInBody draft={draft(s3("r2", "Cloudflare"), { settings: { endpoint: "https://acct.r2.cloudflarestorage.com/backups" } })} providerLabel="R2" />);
  const field = screen.getByLabelText("Bucket") as HTMLInputElement;
  expect(field.placeholder).toBe("backups");
});

it("tells a Garage user which region to type instead of offering an empty one", async () => {
  await show(<SignInBody draft={draft(s3("garage", "Other"))} providerLabel="Garage" />);
  expect(screen.getByRole("img", { name: /s3_region/ })).toBeTruthy();
  expect(screen.queryByText("Use this if unsure.")).toBeNull();
});

it("tells an IDrive e2 user where the endpoint is shown", async () => {
  await show(<SignInBody draft={draft(s3("idrivee2", "IDrive"))} providerLabel="IDrive e2" />);
  expect(screen.getByRole("img", { name: /Enabled regions/ })).toBeTruthy();
});
