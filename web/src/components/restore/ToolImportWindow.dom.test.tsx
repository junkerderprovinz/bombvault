// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ToolImportChoice, ToolImportInput, ToolImportJob, ToolImportResult } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";

let jobs: ToolImportJob[] = [];
let results: ToolImportResult[] = [];
const preview = vi.fn(async (_input: ToolImportInput) => ({ ok: true, jobs }));
const apply = vi.fn(async (_input: ToolImportInput, _jobs: ToolImportChoice[]) => ({ ok: true, jobs: results }));
vi.mock("../../lib/api", () => ({
  previewToolImport: (input: ToolImportInput) => preview(input),
  applyToolImport: (input: ToolImportInput, choices: ToolImportChoice[]) => apply(input, choices),
}));
const push = vi.fn();
vi.mock("../../lib/toast", () => ({ useToast: () => ({ push }) }));
vi.mock("../FolderBrowser", () => ({
  FolderBrowser: ({ label, onChange }: { label: string; onChange: (v: string) => void }) => (
    <input aria-label={label} onChange={(e) => onChange(e.target.value)} />
  ),
}));

import { ToolImportWindow } from "./ToolImportWindow";

window.matchMedia = ((query: string) => ({
  matches: true,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const onClose = vi.fn();
const view = () => (
  <I18nProvider>
    <ToolImportWindow hostMountRoot="/mnt" onClose={onClose} />
  </I18nProvider>
);
const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;
const box = (name: string) => screen.getByRole("checkbox", { name }) as HTMLButtonElement;

function photosJob(): ToolImportJob {
  return {
    key: "0",
    tool: "duplicati",
    name: "Family photos",
    enabled: true,
    schedule: "daily 02:30",
    sets: [
      { key: "0", name: "Family-photos-photos", source: "/source/photos", path: "user/photos", excludes: ["*.tmp"] },
      { key: "1", name: "Family-photos-videos", source: "/source/videos", path: "", excludes: [] },
      { key: "2", name: "Family-photos-music", source: "/source/music", path: "user/music", taken: "music", excludes: [] },
    ],
    dest: { kind: "s3", location: "s3:https://s3.amazonaws.com/photos/bombvault", signIn: "carried" },
    notes: [{ code: "retention" }, { code: "filter", detail: "{TemporaryFiles}" }],
  };
}

beforeEach(() => {
  preview.mockClear();
  apply.mockClear();
  push.mockClear();
  jobs = [photosJob()];
  results = [{ key: "0", sets: ["Family-photos-photos"], location: "Family photos" }];
});
afterEach(cleanup);

async function readDuplicatiFile() {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  const file = new File(['{"Backup":{}}'], "photos.json", { type: "application/json" });
  fireEvent.change(input, { target: { files: [file] } });
  await screen.findByText("photos.json");
  fireEvent.click(button(en["toolImport.read"]));
  await screen.findByText("Family photos");
}

it("shows what each job would become before anything is created", async () => {
  render(view());
  await readDuplicatiFile();
  expect(preview).toHaveBeenCalledWith({ tool: "duplicati", files: [{ name: "photos.json", text: '{"Backup":{}}' }] });
  expect(screen.getByText(en["toolImport.note.retention"])).toBeTruthy();
  expect(screen.getByText(en["toolImport.note.filter"].replace("{detail}", "{TemporaryFiles}"))).toBeTruthy();
  expect(screen.getByText(en["toolImport.excludes"].replace("{list}", "*.tmp"))).toBeTruthy();
  expect(screen.getByText(en["toolImport.signInCarried"])).toBeTruthy();
  expect(apply).not.toHaveBeenCalled();
});

it("ticks only the folders it found that no set backs up yet", async () => {
  render(view());
  await readDuplicatiFile();
  expect(box("Family-photos-photos").getAttribute("aria-checked")).toBe("true");
  expect(box("Family-photos-videos").getAttribute("aria-checked")).toBe("false");
  expect(box("Family-photos-music").getAttribute("aria-checked")).toBe("false");
  expect(screen.getByText(en["toolImport.taken"].replace("{name}", "music"))).toBeTruthy();

  fireEvent.click(button(en["toolImport.import"]));
  await waitFor(() => expect(apply).toHaveBeenCalled());
  expect(apply.mock.calls[0][1]).toEqual([{ key: "0", sets: [{ key: "0" }] }]);
  expect(await screen.findByText(en["toolImport.created"].replace("{list}", "Family-photos-photos"))).toBeTruthy();
});

it("takes a folder picked for a source that was not found", async () => {
  render(view());
  await readDuplicatiFile();
  fireEvent.change(screen.getByLabelText(en["toolImport.pickFolder"].replace("{name}", "Family-photos-videos")), { target: { value: "user/videos" } });
  expect(box("Family-photos-videos").getAttribute("aria-checked")).toBe("true");
  fireEvent.click(button(en["toolImport.import"]));
  await waitFor(() => expect(apply).toHaveBeenCalled());
  expect(apply.mock.calls[0][1][0].sets).toEqual([{ key: "0" }, { key: "1", path: "user/videos" }]);
});

it("asks for the address and the sign-in a B2 job lacks", async () => {
  jobs = [
    {
      ...photosJob(),
      sets: [photosJob().sets[0]],
      dest: { kind: "b2", location: "", b2Path: "docs/bombvault", signIn: "needed" },
    },
  ];
  render(view());
  await readDuplicatiFile();
  expect(button(en["toolImport.import"]).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText(en["toolImport.b2Endpoint"]), { target: { value: "s3.eu-central-003.backblazeb2.com" } });
  fireEvent.change(screen.getByLabelText(en["toolImport.keyId"]), { target: { value: "003abc" } });
  expect(button(en["toolImport.import"]).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText(en["toolImport.secret"]), { target: { value: "K003secret" } });
  fireEvent.click(button(en["toolImport.import"]));
  await waitFor(() => expect(apply).toHaveBeenCalled());
  expect(apply.mock.calls[0][1]).toEqual([
    {
      key: "0",
      sets: [{ key: "0" }],
      location: "s3:https://s3.eu-central-003.backblazeb2.com/docs/bombvault",
      keyId: "003abc",
      secret: "K003secret",
    },
  ]);
});

it("can leave the storage location out", async () => {
  render(view());
  await readDuplicatiFile();
  fireEvent.click(box(en["toolImport.useLocation"]));
  fireEvent.click(button(en["toolImport.import"]));
  await waitFor(() => expect(apply).toHaveBeenCalled());
  expect(apply.mock.calls[0][1]).toEqual([{ key: "0", sets: [{ key: "0" }], location: "" }]);
});

it("reads a Duplicacy folder and says what went wrong per job", async () => {
  jobs = [{ ...photosJob(), tool: "duplicacy-web", name: "documents" }];
  results = [{ key: "0", sets: [], location: "", error: "Family-photos-photos: a file set with this name already exists" }];
  render(view());
  fireEvent.click(screen.getByRole("radio", { name: new RegExp(en["toolImport.fromDuplicacy"]) }));
  fireEvent.change(screen.getByLabelText(en["toolImport.folder"]), { target: { value: "user/appdata/duplicacy" } });
  fireEvent.click(button(en["toolImport.read"]));
  await screen.findByText("documents");
  expect(preview).toHaveBeenCalledWith({ tool: "duplicacy", path: "user/appdata/duplicacy" });
  fireEvent.click(button(en["toolImport.import"]));
  expect(await screen.findByText("Family-photos-photos: a file set with this name already exists")).toBeTruthy();
});

it("says so when the settings cannot be read", async () => {
  preview.mockResolvedValueOnce({ ok: false, error: "the export is encrypted", jobs: [] } as never);
  render(view());
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  fireEvent.change(input, { target: { files: [new File(["AES"], "x.json")] } });
  await screen.findByText("x.json");
  fireEvent.click(button(en["toolImport.read"]));
  await waitFor(() => expect(push).toHaveBeenCalledWith("the export is encrypted", "fail"));
});
