// @vitest-environment jsdom
/**
 * Servers somebody runs: an SFTP host pinned to the key it showed at the
 * test, an SMB server whose first level is its shares, and the yes-or-no
 * settings of their backends.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, renderHook, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import type { Backend, BackendOption, Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getProviders: vi.fn(),
  getVMSSH: vi.fn(),
  checkDraft: vi.fn(),
  draftFolders: vi.fn(),
  createDestination: vi.fn(),
}));

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => [],
}));

const api = await import("../../lib/api");
const checkDraft = vi.mocked(api.checkDraft);
const draftFolders = vi.mocked(api.draftFolders);
const createDestination = vi.mocked(api.createDestination);
const { useDestinationDraft, formFields } = await import("./useDestinationDraft");
const { DraftField, FolderBody, SignInBody } = await import("./steps");
type DraftState = ReturnType<typeof useDestinationDraft>;

function option(name: string, over: Partial<BackendOption> = {}): BackendOption {
  return { name, help: "", required: false, secret: false, password: false, advanced: false, essential: false, default: "", ...over };
}

const SFTP: Backend = {
  name: "sftp",
  description: "",
  options: [option("host", { required: true }), option("pass", { password: true }), option("key_pem", { secret: true }), option("ssh")],
};
const SMB: Backend = { name: "smb", description: "", options: [option("host", { required: true }), option("use_kerberos", { type: "bool" })] };

const SFTP_PROVIDER: Provider = {
  id: "sftp", name: "SFTP", backend: "sftp", group: "server", route: "rclone", auth: "sshkey", unasked: ["key_pem", "ssh"],
};
const SMB_PROVIDER: Provider = { id: "smb", name: "SMB share", backend: "smb", group: "server", route: "rclone", auth: "login", shares: true };
const FTP_PROVIDER: Provider = { id: "ftp", name: "FTP", backend: "ftp", group: "server", route: "rclone", auth: "login" };

const PIN = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB";
const FINGERPRINT = "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU";

function wrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider>
      <ToastProvider>{children}</ToastProvider>
    </I18nProvider>
  );
}

async function draftFor(provider: Provider) {
  const hook = renderHook(() => useDestinationDraft(), { wrapper });
  await act(async () => {});
  await act(async () => hook.result.current.pick(provider));
  return hook;
}

beforeEach(() => {
  vi.mocked(api.getProviders).mockResolvedValue({ ok: true, providers: [SFTP_PROVIDER, SMB_PROVIDER], backends: { sftp: SFTP, smb: SMB } });
  vi.mocked(api.getVMSSH).mockResolvedValue({ ok: true, publicKey: "ssh-ed25519 AAAA bombvault" } as never);
  checkDraft.mockResolvedValue({ ok: true, reachable: true, initialized: false });
  draftFolders.mockResolvedValue({ ok: true, folders: [] });
  createDestination.mockResolvedValue({ ok: true, destination: { id: "new", name: "SFTP" } as never });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("an SFTP server", () => {
  it("asks for no key or ssh command that would replace BombVault's own key", () => {
    const { main, more } = formFields(SFTP_PROVIDER, SFTP);
    const names = [...main, ...more].map((o) => o.name);
    expect(names).toEqual(["host", "pass"]);
  });

  it("is saved pinned to the key it showed at the test", async () => {
    checkDraft.mockResolvedValue({ ok: true, reachable: true, initialized: false, hostKey: { pin: PIN, fingerprint: FINGERPRINT } });
    const { result } = await draftFor(SFTP_PROVIDER);
    await act(async () => result.current.setSetting("host", "box.example"));

    await act(async () => {
      await result.current.runTest();
    });
    expect(result.current.hostKey?.fingerprint).toBe(FINGERPRINT);
    await act(async () => {
      await result.current.save();
    });

    expect(createDestination).toHaveBeenCalledWith(expect.objectContaining({ settings: { host: "box.example", host_keys: PIN } }));
  });

  it("forgets the key once a field names another server", async () => {
    checkDraft.mockResolvedValue({ ok: true, reachable: true, initialized: false, hostKey: { pin: PIN, fingerprint: FINGERPRINT } });
    const { result } = await draftFor(SFTP_PROVIDER);
    await act(async () => {
      await result.current.runTest();
    });

    await act(async () => result.current.setSetting("host", "other.example"));

    expect(result.current.hostKey).toBeNull();
  });

  it("shows the fingerprint after a test that passed", async () => {
    const draft = {
      picked: SFTP_PROVIDER,
      fields: { main: [], more: [] },
      settings: {},
      name: "SFTP",
      setName: () => {},
      credsRef: "",
      publicKey: "",
      reached: true,
      hostKey: { pin: PIN, fingerprint: FINGERPRINT },
      test: { verdict: { ok: true } },
    } as unknown as DraftState;
    render(
      <I18nProvider>
        <SignInBody draft={draft} providerLabel="SFTP" />
      </I18nProvider>
    );

    expect(screen.getByText("Server key")).toBeTruthy();
    expect(screen.getByText(FINGERPRINT)).toBeTruthy();
  });
});

describe("an SMB server", () => {
  it("is tested at the share named for a server that does not list its shares", async () => {
    const { result } = await draftFor(SMB_PROVIDER);
    await act(async () => result.current.setShare("backups"));

    await act(async () => {
      await result.current.runTest();
    });

    expect(checkDraft).toHaveBeenCalledWith(expect.objectContaining({ provider: "smb", dir: "backups" }));
    expect(draftFolders).toHaveBeenLastCalledWith(expect.objectContaining({ dir: "backups" }));
    expect(result.current.folderOpen).toBe(false);
  });

  it("cannot be saved on its list of shares", async () => {
    const { result } = await draftFor(SMB_PROVIDER);
    await act(async () => {
      await result.current.runTest();
    });

    expect(result.current.dir).toEqual([]);
    expect(result.current.needsBucket).toBe(true);
    expect(result.current.folderOpen).toBe(true);
  });

  it("offers no new folder among its shares", () => {
    const draft = {
      picked: SMB_PROVIDER,
      browsable: true,
      dir: [],
      folders: ["media"],
      free: null,
      newFolder: null,
      needsBucket: true,
      location: "rclone:smb:",
      loadFolders: async () => {},
    } as unknown as DraftState;
    render(
      <I18nProvider>
        <FolderBody draft={draft} />
      </I18nProvider>
    );

    expect(screen.queryByRole("button", { name: "New folder" })).toBeNull();
    expect(screen.getByText(/Open a share first/)).toBeTruthy();
  });
});

describe("a yes-or-no setting", () => {
  function field(provider: Provider, o: BackendOption, settings: Record<string, string> = {}) {
    const setSetting = vi.fn();
    render(
      <I18nProvider>
        <DraftField draft={{ picked: provider, settings, setSetting }} option={o} />
      </I18nProvider>
    );
    return setSetting;
  }

  it("is a switch that writes the word rclone reads", async () => {
    const setSetting = field(SMB_PROVIDER, option("use_kerberos", { type: "bool" }));

    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: "use_kerberos" }));
    });

    expect(setSetting).toHaveBeenCalledWith("use_kerberos", "true");
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("makes FTP's two TLS switches one choice and moves implicit TLS to its port", async () => {
    const setSetting = field(FTP_PROVIDER, option("tls", { type: "bool" }), { port: "21" });

    await act(async () => {
      fireEvent.click(screen.getByRole("combobox", { name: "Encryption" }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("option", { name: "Implicit TLS (FTPS, port 990)" }));
    });

    expect(setSetting).toHaveBeenCalledWith("tls", "true");
    expect(setSetting).toHaveBeenCalledWith("explicit_tls", "false");
    expect(setSetting).toHaveBeenCalledWith("port", "990");
  });

  it("leaves FTP's explicit TLS switch to that choice", () => {
    field(FTP_PROVIDER, option("explicit_tls", { type: "bool" }));

    expect(screen.queryByRole("switch")).toBeNull();
  });
});
