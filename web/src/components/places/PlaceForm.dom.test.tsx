// @vitest-environment jsdom
// Nothing is added that was not tested: the probe runs on the typed fields,
// any later edit makes it stale, and Add sends what the probe completed.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { CatalogProvider, ProbeRequest, ProbeResult } from "../../lib/places";
import type { OkEnvelope } from "../../lib/api";

const probes: ProbeRequest[] = [];
let probeAnswer: OkEnvelope & ProbeResult = { ok: true };

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    probePlace: (req: ProbeRequest) => {
      probes.push(structuredClone(req));
      return Promise.resolve(probeAnswer);
    },
  };
});

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getVMSSH: () => Promise.resolve({ ok: true, publicKey: "ssh-ed25519 AAAA bombvault" }),
    browse: (path = "") => Promise.resolve({ ok: true, path, dirs: [] }),
  };
});

const { PlaceForm } = await import("./PlaceForm");

const WASABI: CatalogProvider = {
  id: "wasabi",
  group: "cloud",
  kind: "s3",
  offPremises: true,
  fields: [
    { key: "keyId" },
    { key: "secret", secret: true },
    { key: "region", placeholder: "eu-central-1" },
    { key: "bucket", optional: true, placeholder: "backups" },
    { key: "path", optional: true, placeholder: "bombvault" },
  ],
};
const SYNOLOGY: CatalogProvider = {
  id: "synology",
  group: "here",
  kind: "local",
  pickRoots: ["remotes"],
  fields: [{ key: "path", placeholder: "remotes/nas/bombvault" }],
};
const SFTP: CatalogProvider = {
  id: "sftp",
  group: "self",
  kind: "sftp",
  defaultPort: 22,
  fields: [{ key: "host" }, { key: "port", optional: true }, { key: "user" }, { key: "path", optional: true }],
};
const AZURE: CatalogProvider = {
  id: "azure",
  group: "cloud",
  kind: "azure",
  offPremises: true,
  fields: [{ key: "account" }, { key: "secret", secret: true }, { key: "container", optional: true }, { key: "path", optional: true }],
};

beforeEach(() => {
  probes.length = 0;
  probeAnswer = { ok: true, base: "s3:https://s3.eu-central-1.wasabisys.com/bv/bombvault", folders: { containers: "empty" } };
});
afterEach(cleanup);

async function form(provider: CatalogProvider) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <PlaceForm provider={provider} hostMountRoot="/mnt" onBack={vi.fn()} onCancel={vi.fn()} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

// An optional field's label carries the word "optional" after its name.
const field = (label: string) => screen.getByLabelText(new RegExp(`^${label}`));
const type = (label: string, value: string) => fireEvent.change(field(label), { target: { value } });
const button = (key: keyof typeof en) => screen.getByRole("button", { name: en[key] });
async function testConnection() {
  await act(async () => {
    fireEvent.click(button("places.form.test"));
  });
}

describe("PlaceForm fields", () => {
  it("asks for every field of the provider, with secrets behind the eye", async () => {
    await form(WASABI);
    expect(screen.getByLabelText(en["places.field.keyId"])).toHaveProperty("type", "text");
    expect(screen.getByLabelText(en["places.field.secret"])).toHaveProperty("type", "password");
    expect(screen.getByRole("button", { name: en["common.showValue"] })).toBeTruthy();
    expect(screen.getByLabelText(en["places.field.region"]).getAttribute("placeholder")).toBe("eu-central-1");
    expect(screen.getAllByText(en["places.form.optional"])).toHaveLength(2);
  });

  it("picks the folder of a local place in the folder browser", async () => {
    await form(SYNOLOGY);
    expect(button("folder.browseTitle")).toBeTruthy();
    expect(screen.queryByLabelText(en["places.field.path"])).toBeNull();
  });

  it("shows the public key an SFTP server needs", async () => {
    await form(SFTP);
    expect(await screen.findByText("ssh-ed25519 AAAA bombvault")).toBeTruthy();
    expect(field(en["places.field.port"]).getAttribute("placeholder")).toBe("22");
  });

  it("names each field the way its kind does", async () => {
    await form({
      id: "nextcloud",
      group: "self",
      kind: "webdav",
      fields: [{ key: "url" }, { key: "user" }, { key: "password", secret: true }, { key: "path", optional: true }],
    });
    expect(screen.getByLabelText(en["places.field.url"])).toHaveProperty("type", "text");
    expect(screen.getByLabelText(en["places.field.appPassword"])).toHaveProperty("type", "password");
    cleanup();
    await form(AZURE);
    expect(screen.getByLabelText(en["places.field.storageAccount"])).toHaveProperty("type", "text");
    expect(screen.getByLabelText(en["places.field.accessKey"])).toHaveProperty("type", "password");
    expect(field(en["places.field.container"])).toHaveProperty("type", "text");
  });
});

describe("PlaceForm connection test", () => {
  it("probes the typed fields and shows what it found", async () => {
    probeAnswer = {
      ok: true,
      base: "s3:https://s3.eu-central-1.wasabisys.com/bv",
      facts: [{ key: "places.probe.bucketNew", params: { bucket: "bv" } }],
      folders: { containers: "empty", vms: "repository" },
    };
    await form(WASABI);
    type(en["places.field.keyId"], "AKIA1");
    type(en["places.field.secret"], "s3cret");
    type(en["places.field.bucket"], "bv");
    await testConnection();
    expect(probes).toEqual([
      { provider: "wasabi", fields: { keyId: "AKIA1", secret: "s3cret", region: "", bucket: "bv", path: "" } },
    ]);
    expect(screen.getByText("s3:https://s3.eu-central-1.wasabisys.com/bv")).toBeTruthy();
    expect(screen.getByText(en["places.probe.bucketNew"].replace("{bucket}", "bv"))).toBeTruthy();
    expect(screen.getByText(en["places.folderState.repository"])).toBeTruthy();
  });

  it("offers the buckets a key may see, and tests again with the one chosen", async () => {
    probeAnswer = { ok: true, buckets: ["alpha", "beta"] };
    await form(WASABI);
    await testConnection();
    const pick = screen.getByRole("combobox", { name: en["places.field.bucket"] });
    fireEvent.click(pick);
    fireEvent.click(screen.getByRole("option", { name: "beta" }));
    probeAnswer = { ok: true, base: "s3:https://s3.eu-central-1.wasabisys.com/beta" };
    await testConnection();
    expect(probes[1]!.fields.bucket).toBe("beta");
    expect(screen.getByText("s3:https://s3.eu-central-1.wasabisys.com/beta")).toBeTruthy();
  });

  it("shows no result while a bucket is still to be chosen", async () => {
    probeAnswer = { ok: true, buckets: ["alpha", "beta"] };
    await form(WASABI);
    await testConnection();
    expect(screen.getByRole("combobox", { name: en["places.field.bucket"] })).toBeTruthy();
    expect(document.querySelector('[aria-live="polite"]')).toBeNull();
  });

  it("offers the containers of an Azure account, and tests again with the one chosen", async () => {
    probeAnswer = { ok: true, buckets: ["backups", "photos"] };
    await form(AZURE);
    await testConnection();
    fireEvent.click(screen.getByRole("combobox", { name: en["places.field.container"] }));
    expect(screen.getByRole("option", { name: en["places.form.chooseContainer"] })).toBeTruthy();
    fireEvent.click(screen.getByRole("option", { name: "photos" }));
    await testConnection();
    expect(probes[1]!.fields.container).toBe("photos");
  });

  it("drops what it found once a field changes", async () => {
    await form(WASABI);
    type(en["places.field.keyId"], "AKIA1");
    await testConnection();
    expect(screen.getByText("s3:https://s3.eu-central-1.wasabisys.com/bv/bombvault")).toBeTruthy();
    type(en["places.field.keyId"], "AKIA2");
    expect(screen.queryByText("s3:https://s3.eu-central-1.wasabisys.com/bv/bombvault")).toBeNull();
  });

  it("says why a probe failed, and adds nothing", async () => {
    probeAnswer = { ok: false, code: "place-probe-failed", error: "dial tcp: connection refused" };
    await form(WASABI);
    await testConnection();
    expect(await screen.findByText("The connection test failed: dial tcp: connection refused")).toBeTruthy();
    expect(screen.queryByText(en["places.form.address"])).toBeNull();
    expect(button("places.form.test").className).toContain("glim-shake");
  });
});
