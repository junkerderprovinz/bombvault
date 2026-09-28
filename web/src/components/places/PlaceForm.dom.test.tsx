// @vitest-environment jsdom
// Nothing is added that was not tested: the probe runs on the typed fields,
// any later edit makes it stale, and Add sends what the probe completed.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { CatalogProvider, CreatePlaceBody, ProbeRequest, ProbeResult } from "../../lib/places";
import type { OkEnvelope } from "../../lib/api";

const probes: ProbeRequest[] = [];
const creates: CreatePlaceBody[] = [];
let probeAnswer: OkEnvelope & ProbeResult = { ok: true };
let createAnswer: OkEnvelope & { place?: unknown; code?: string } = { ok: true };
let copyWorks = true;

vi.mock("../../lib/clipboard", () => ({ copyText: () => Promise.resolve(copyWorks) }));

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    probePlace: (req: ProbeRequest) => {
      probes.push(structuredClone(req));
      return Promise.resolve(probeAnswer);
    },
    createPlace: (body: CreatePlaceBody) => {
      creates.push(structuredClone(body));
      return Promise.resolve(createAnswer);
    },
    restServerRecipe: () =>
      Promise.resolve({
        ok: true,
        snippet: { user: "tower", password: "Xy12", htpasswd: "tower:$2a$12$h", dockerRun: "docker run", compose: "services:", unraid: "<Container/>" },
      }),
  };
});

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getVMSSH: () => Promise.resolve({ ok: true, publicKey: "ssh-ed25519 AAAA bombvault" }),
    browse: (path = "") => Promise.resolve({ ok: true, path, dirs: [] }),
    getRclone: () => Promise.resolve({ ok: true, remotes: ["b2", "gdrive"] }),
    setRclone: () => Promise.resolve({ ok: true }),
  };
});

// The stub is built inside its factory, which vitest hoists above this file's imports.
vi.mock("./MeshOffers", async () => {
  const { createElement } = await import("react");
  return {
    MeshOffers: ({ onAccepted }: { onAccepted: (place: { id: string; name: string }) => void }) =>
      createElement("button", { type: "button", onClick: () => onAccepted({ id: "p9", name: "mesh: tower" }) }, "accept an offer"),
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
const UNRAID: CatalogProvider = { ...SYNOLOGY, id: "unraid-folder", pickRoots: ["user", ""], offPremises: false };
const SFTP: CatalogProvider = {
  id: "sftp",
  group: "self",
  kind: "sftp",
  defaultPort: 22,
  fields: [{ key: "host" }, { key: "port", optional: true }, { key: "user" }, { key: "path", optional: true }],
};
const NEXTCLOUD: CatalogProvider = {
  id: "nextcloud",
  group: "self",
  kind: "webdav",
  fields: [{ key: "url" }, { key: "user" }, { key: "password", secret: true }, { key: "path", optional: true }],
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
  creates.length = 0;
  probeAnswer = { ok: true, base: "s3:https://s3.eu-central-1.wasabisys.com/bv/bombvault", folders: { containers: "empty" } };
  createAnswer = { ok: true, place: { id: "p1", name: "Wasabi" } };
  copyWorks = true;
});
afterEach(cleanup);

async function form(provider: CatalogProvider, onAdded = vi.fn(), onAccepted = vi.fn()) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <PlaceForm
            provider={provider}
            hostMountRoot="/mnt"
            onBack={vi.fn()}
            onCancel={vi.fn()}
            onAdded={onAdded}
            onAccepted={onAccepted}
          />
        </ToastProvider>
      </I18nProvider>
    );
  });
  return onAdded;
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
// A disabled button takes no hover, so its tip opens on the box around it.
function tipOf(el: HTMLElement): string | null | undefined {
  const target = (el as HTMLButtonElement).disabled ? el.parentElement! : el;
  fireEvent.mouseEnter(target);
  const text = document.querySelector(".glim-bubble")?.textContent;
  fireEvent.mouseLeave(target);
  return text;
}

describe("PlaceForm fields", () => {
  it("starts in the first field", async () => {
    await form(WASABI);
    expect(document.activeElement).toBe(screen.getByLabelText(en["places.field.keyId"]));
    cleanup();
    await form(SYNOLOGY);
    expect(document.activeElement).toBe(screen.getByPlaceholderText("remotes/nas/bombvault"));
  });

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

  it("says so and shakes the copy button when the public key cannot be copied", async () => {
    copyWorks = false;
    await form(SFTP);
    await screen.findByText("ssh-ed25519 AAAA bombvault");
    await act(async () => {
      fireEvent.click(button("common.copy"));
    });
    expect(screen.getByText(en["vm.ssh.copyFailed"])).toBeTruthy();
    expect(button("common.copy").className).toContain("glim-shake");
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

  it("says in words why a domain's folder could not be read", async () => {
    // A failed folder fails the test as a whole and leaves the top-level
    // code and error empty, as places_probe.go answers.
    probeAnswer = {
      ok: false,
      base: "s3:https://s3.eu-central-1.wasabisys.com/bv",
      folders: { containers: "empty", vms: "error" },
      errors: { vms: { code: "direct-access-denied", error: "403 Forbidden" } },
    };
    await form(WASABI);
    await testConnection();
    const reason = en["placementCode.directAccessDenied"];
    expect(screen.getByText(reason)).toBeTruthy();
    expect(screen.getByText(en["places.error.probeFailed"].replace("{reason}", `${en["nav.vms"]}: ${reason}`))).toBeTruthy();
    expect(screen.queryByText("403 Forbidden")).toBeNull();
    expect(screen.queryByText(en["places.form.name"])).toBeNull();
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

  it("keeps a typed bucket that the listing does not hold as the one chosen", async () => {
    probeAnswer = {
      ok: true,
      base: "s3:https://s3.eu-central-1.wasabisys.com/fresh",
      buckets: ["alpha", "beta"],
      facts: [{ key: "places.probe.bucketNew", params: { bucket: "fresh" } }],
    };
    await form(WASABI);
    type(en["places.field.bucket"], "fresh");
    await testConnection();
    fireEvent.click(screen.getByRole("combobox", { name: en["places.field.bucket"] }));
    expect(screen.getByRole("option", { name: "fresh" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("option", { name: "alpha" })).toBeTruthy();
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

describe("PlaceForm add", () => {
  it("adds only what was tested", async () => {
    await form(WASABI);
    expect(button("places.form.add")).toHaveProperty("disabled", true);
    expect(screen.queryByLabelText(en["places.form.name"])).toBeNull();
    type(en["places.field.keyId"], "AKIA1");
    await testConnection();
    expect(button("places.form.add")).toHaveProperty("disabled", false);
    type(en["places.field.keyId"], "AKIA2");
    expect(button("places.form.add")).toHaveProperty("disabled", true);
  });

  it("gives the accent to Test until it passes, then to Add", async () => {
    await form(WASABI);
    expect(button("places.form.test").className).toContain("bg-accent");
    expect(button("places.form.add").className).not.toContain("bg-accent");
    await testConnection();
    expect(button("places.form.test").className).not.toContain("bg-accent");
    expect(button("places.form.add").className).toContain("bg-accent");
  });

  it("says what Add still waits for", async () => {
    probeAnswer = { ok: true, base: "remotes/syno/bombvault" };
    await form(SYNOLOGY);
    expect(tipOf(button("places.form.add"))).toBe(en["places.form.testFirst"]);
    await testConnection();
    expect(tipOf(button("places.form.add"))).toBe(en["places.form.whereFirst"]);
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.here"] }));
    type(en["places.form.name"], " ");
    expect(tipOf(button("places.form.add"))).toBe(en["places.form.nameFirst"]);
    type(en["places.form.name"], "NAS");
    expect(button("places.form.add")).toHaveProperty("disabled", false);
  });

  it("names the place after its provider and sends what the probe completed", async () => {
    probeAnswer = {
      ok: true,
      base: "s3:https://s3.eu-central-1.wasabisys.com/bv/bombvault",
      fields: { keyId: "AKIA1", region: "eu-central-1", bucket: "bv", path: "bombvault" },
    };
    const onAdded = await form(WASABI);
    type(en["places.field.keyId"], "AKIA1");
    type(en["places.field.secret"], "s3cret");
    await testConnection();
    expect(field(en["places.form.name"])).toHaveProperty("value", "Wasabi");
    // A cloud provider always stands at another site, so nothing asks.
    expect(screen.queryByRole("tablist", { name: en["places.form.where"] })).toBeNull();
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates).toEqual([
      {
        provider: "wasabi",
        fields: { keyId: "AKIA1", secret: "s3cret", region: "eu-central-1", bucket: "bv", path: "bombvault" },
        name: "Wasabi",
      },
    ]);
    expect(onAdded).toHaveBeenCalledWith({ id: "p1", name: "Wasabi" });
  });

  it("asks a device where it stands before it can be added", async () => {
    probeAnswer = { ok: true, base: "remotes/syno/bombvault" };
    await form(SYNOLOGY);
    fireEvent.change(screen.getByPlaceholderText("remotes/nas/bombvault"), { target: { value: "remotes/syno/bombvault" } });
    await testConnection();
    const where = screen.getByRole("tablist", { name: en["places.form.where"] });
    expect(button("places.form.add")).toHaveProperty("disabled", true);
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.away"] }));
    expect(where.querySelector('[aria-selected="true"]')?.textContent).toBe(en["places.form.away"]);
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates[0]).toMatchObject({ provider: "synology", name: "Synology", offPremises: true });
  });

  it("does not ask about a folder on this server", async () => {
    probeAnswer = { ok: true, base: "user/bombvault" };
    await form(UNRAID);
    await testConnection();
    expect(screen.queryByRole("tablist", { name: en["places.form.where"] })).toBeNull();
    expect(button("places.form.add")).toHaveProperty("disabled", false);
  });

  it("makes an address that already holds a repository a place that is one", async () => {
    probeAnswer = { ok: true, base: "rest:https://nas:8000/tower", repoIds: { "": "r1" }, facts: [{ key: "places.probe.baseIsRepository" }] };
    await form(WASABI);
    await testConnection();
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates[0]!.folders).toEqual({ containers: "", vms: "", flash: "", config: "", files: "", zfs: "" });
  });

  it("adds a Nextcloud whose probe names no address yet", async () => {
    probeAnswer = { ok: true, folders: { containers: "empty" } };
    await form(NEXTCLOUD);
    await testConnection();
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.away"] }));
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates[0]).toMatchObject({ provider: "nextcloud", name: "Nextcloud" });
  });

  it("adds a Nextcloud folder that is a repository, with no address named", async () => {
    probeAnswer = { ok: true, repoIds: { "": "r1" }, facts: [{ key: "places.probe.baseIsRepository" }] };
    await form(NEXTCLOUD);
    await testConnection();
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.away"] }));
    expect(button("places.form.add")).toHaveProperty("disabled", false);
  });

  it("says why an add was refused and shakes the button", async () => {
    createAnswer = { ok: false, code: "place-name-taken", error: "place name taken" };
    const onAdded = await form(WASABI);
    await testConnection();
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(await screen.findByText(en["places.error.nameTaken"])).toBeTruthy();
    expect(button("places.form.add").className).toContain("glim-shake");
    expect(onAdded).not.toHaveBeenCalled();
  });
});

const RCLONE: CatalogProvider = {
  id: "rclone",
  group: "self",
  kind: "rclone",
  fields: [{ key: "remote" }, { key: "path", optional: true, placeholder: "bombvault" }],
};

describe("PlaceForm rclone", () => {
  it("offers the remotes of BombVault's rclone config", async () => {
    await form(RCLONE);
    fireEvent.click(screen.getByRole("combobox", { name: en["places.field.remote"] }));
    expect(screen.getByRole("option", { name: en["places.form.chooseRemote"] })).toBeTruthy();
    fireEvent.click(screen.getByRole("option", { name: "gdrive" }));
    await testConnection();
    expect(probes[0]!.fields.remote).toBe("gdrive");
    expect(screen.getByLabelText(en["places.rclone.config"])).toBeTruthy();
  });

  it("starts in the remote picker the stored remotes turn the field into", async () => {
    await form(RCLONE);
    expect(document.activeElement).toBe(screen.getByRole("combobox", { name: en["places.field.remote"] }));
  });

  it("wants a new test once a changed config is saved", async () => {
    await form(RCLONE);
    fireEvent.click(screen.getByRole("combobox", { name: en["places.field.remote"] }));
    fireEvent.click(screen.getByRole("option", { name: "gdrive" }));
    await testConnection();
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.away"] }));
    expect(button("places.form.add")).toHaveProperty("disabled", false);
    fireEvent.change(screen.getByLabelText(en["places.rclone.config"]), { target: { value: "[gdrive]\ntype = drive" } });
    await act(async () => {
      fireEvent.click(button("places.rclone.save"));
    });
    expect(button("places.form.add")).toHaveProperty("disabled", true);
  });
});

const REST: CatalogProvider = {
  id: "rest-server",
  group: "self",
  kind: "rest",
  fields: [{ key: "url" }, { key: "user" }, { key: "password", secret: true }, { key: "path", optional: true }],
};

describe("PlaceForm rest-server", () => {
  it("fills in the login of the recipe it shows, and tests it like a typed one", async () => {
    await form(REST);
    await act(async () => {
      fireEvent.click(button("places.recipe.show"));
    });
    expect(field(en["places.field.user"])).toHaveProperty("value", "tower");
    type(en["places.field.url"], "http://nas:8000");
    await testConnection();
    expect(probes[0]!.fields).toMatchObject({ url: "http://nas:8000", user: "tower", password: "Xy12" });
  });

  it("starts a place set up from the recipe append-only, as the recipe runs the server", async () => {
    probeAnswer = { ok: true, base: "rest:http://nas:8000/tower" };
    await form(REST);
    await act(async () => {
      fireEvent.click(button("places.recipe.show"));
    });
    type(en["places.field.url"], "http://nas:8000");
    await testConnection();
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.here"] }));
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates[0]).toMatchObject({ provider: "rest-server", immutable: true });
  });

  it("leaves append-only to the details for a server it did not set up", async () => {
    probeAnswer = { ok: true, base: "rest:http://nas:8000/tower" };
    await form(REST);
    type(en["places.field.url"], "http://nas:8000");
    type(en["places.field.user"], "tower");
    type(en["places.field.password"], "pw");
    await testConnection();
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.here"] }));
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates[0]!.immutable).toBeUndefined();
  });

  it("leaves append-only to the details once the recipe's login is typed over", async () => {
    probeAnswer = { ok: true, base: "rest:http://nas:8000/backup" };
    await form(REST);
    await act(async () => {
      fireEvent.click(button("places.recipe.show"));
    });
    type(en["places.field.url"], "http://nas:8000");
    type(en["places.field.user"], "backup");
    type(en["places.field.password"], "pw");
    await testConnection();
    fireEvent.click(screen.getByRole("tab", { name: en["places.form.here"] }));
    await act(async () => {
      fireEvent.click(button("places.form.add"));
    });
    expect(creates[0]!.immutable).toBeUndefined();
  });
});

const BOMBVAULT: CatalogProvider = { ...REST, id: "bombvault" };

describe("PlaceForm Another BombVault", () => {
  it("hands the place an accepted offer made to onAccepted, not onAdded", async () => {
    const onAccepted = vi.fn();
    const onAdded = await form(BOMBVAULT, vi.fn(), onAccepted);
    fireEvent.click(screen.getByRole("button", { name: "accept an offer" }));
    expect(onAccepted).toHaveBeenCalledWith({ id: "p9", name: "mesh: tower" });
    expect(onAdded).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: en["places.recipe.show"] })).toBeNull();
  });

  it("lists no offers under another provider", async () => {
    await form(REST);
    expect(screen.queryByRole("button", { name: "accept an offer" })).toBeNull();
  });
});
