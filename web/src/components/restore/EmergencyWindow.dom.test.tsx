// @vitest-environment jsdom
/**
 * The emergency window: the key of the install that is gone, BombVault's own
 * settings, the choice of what comes back, and the restore.
 *
 * The key is shown with the recovery kit, through the group or typed, tried
 * on every place and held in the server's memory. Nothing is stored before it is taken over, and
 * a takeover can be undone until the first backup.
 *
 * A repository that holds a database only as dumps holds the only copy of
 * it, so those containers are listed apart and never swept into restoring
 * everything, which would bring them back with an empty database.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import type {
  CloudCreds,
  Container,
  DBDumpView,
  EmergencyPlace,
  EmergencyRefusal,
  EmergencySession,
  EmergencyState,
  FileSetView,
  OffsiteEntries,
  Run,
  VM,
} from "../../lib/api";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";
import type { EmergencyStep, EmergencyWay } from "../../lib/restorePlan";
import { RESTORE_NO_RUNTIME } from "../../lib/runReason";
import { ToastProvider } from "../../lib/toast";
import { SETTINGS, containerView, fileSetView, vmView } from "../../pages/backupEntry.testsupport";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

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

let containersOnServer: Container[] = [];
let vmsOnServer: VM[] = [];
let setsOnServer: FileSetView[] = [];
let dumpsOnServer: DBDumpView[] = [];
let runsOnServer: Run[] = [];
// Containers whose restore Docker refuses until it is asked without a runtime.
let needsRuntime: string[] = [];

function finishedRun(kind: string, domain: string, target: string, error = ""): Run {
  return {
    id: `${kind}-${runsOnServer.length + 1}`,
    targetId: "t1",
    kind,
    status: error ? "failed" : "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_010,
    snapshotId: "s1",
    bytes: 0,
    error,
    acknowledged: false,
    target,
    domain,
  };
}

// Every start runs detached and is watched through the recorded runs, so each
// one hands its finished run to the next poll.
const restore = vi.fn((name: string, _snapshot: string, _confirm: boolean, _source?: string, _stopped?: boolean, bare?: boolean) => {
  const refused = needsRuntime.includes(name) && !bare;
  runsOnServer = [...runsOnServer, finishedRun("restore", "container", name, refused ? RESTORE_NO_RUNTIME : "")];
  return Promise.resolve({ ok: true, started: true });
});
const restoreVM = vi.fn((name: string) => {
  runsOnServer = [...runsOnServer, finishedRun("restore", "vm", name)];
  return Promise.resolve({ ok: true, started: true });
});
const restoreFileSet = vi.fn((id: string) => {
  const set = setsOnServer.find((s) => s.id === id);
  runsOnServer = [...runsOnServer, finishedRun("restore", "files", set?.name ?? id)];
  return Promise.resolve({ ok: true, started: true });
});
const fileSetSnapshots = vi.fn((id: string, source?: string) => {
  void id;
  void source;
  return Promise.resolve({ ok: true, snapshots: [{ id: "f-old", time: "2026-09-01T03:00:00Z" }, { id: "f-new", time: "2026-09-20T03:00:00Z" }] });
});
// What the off-site copies hold, by entry.
let offsiteOnServer: OffsiteEntries = { containers: {}, vms: {}, fileSets: {} };
const importDbDump = vi.fn((name: string) => {
  runsOnServer = [...runsOnServer, finishedRun("dbimport", "container", name)];
  return Promise.resolve({ ok: true, started: true });
});
const putSettings = vi.fn((settings: unknown) => {
  void settings;
  return Promise.resolve({ ok: true });
});
const restoreConfig = vi.fn((snapshot: string, source?: string, emergency?: string) => {
  void snapshot;
  void source;
  void emergency;
  return Promise.resolve({ ok: true, staged: true, autoRestart: false });
});
const discoverAll = vi.fn(() =>
  Promise.resolve({
    containers: containersOnServer.length,
    vms: vmsOnServer.length,
    files: setsOnServer.length,
    zfs: 0,
    skipped: [],
    skippedNeedsAction: false,
    paused: [],
    leftOpen: [],
    directRepos: [],
  })
);

// The emergency as the server keeps it: the key it backs up under and, while
// one is shown, the session that tried it.
let emergencyOnServer: EmergencyState = { backupKey: "own", undoable: false };

const KIT_PLACES: EmergencyPlace[] = [
  { domain: "containers", kind: "local", location: "/mnt/user/old/containers", folder: "old/containers", state: "readable" },
  { domain: "config", kind: "local", location: "/mnt/user/old/config", folder: "old/config", state: "readable" },
  { domain: "vms", kind: "offsite", location: "rclone:box:bombvault/vms", state: "sign-in" },
  { domain: "files", kind: "local", location: "/mnt/disks/usb/files", state: "missing" },
];

const SESSION = "sess-1";

function session(over: Partial<EmergencySession> = {}): EmergencySession {
  return {
    id: SESSION,
    source: "kit",
    kit: { server: "tower", writtenAt: 1_789_000_000 },
    replaces: { cloudSignIn: false, rcloneConfig: false },
    encrypted: true,
    places: KIT_PLACES,
    expiresAt: Math.floor(Date.now() / 1000) + 1800,
    cloudSignIn: true,
    rcloneConfig: false,
    ...over,
  };
}

const answer = () => Promise.resolve({ ok: true, emergency: emergencyOnServer });
const refused = (code: EmergencyRefusal) => Promise.resolve({ ok: false, code, error: `server said ${code}` });

const SEALED = "-----BEGIN AGE ENCRYPTED FILE-----\nYWdl\n-----END AGE ENCRYPTED FILE-----\n";
const IDENTITY = "AGE-SECRET-KEY-1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ";

const readEmergencyKit = vi.fn((kit: string, identity?: string) => {
  if (kit === SEALED && !identity) return refused("kit-sealed");
  if (kit === SEALED && identity !== IDENTITY) return refused("kit-not-for-identity");
  emergencyOnServer = { ...emergencyOnServer, session: session() };
  return answer();
});
const readEmergencyKey = vi.fn((key: string, places: string[], creds?: CloudCreds) => {
  void creds;
  if (!/^[0-9a-f]{64}$/.test(key)) return refused("emergency-key");
  emergencyOnServer = {
    ...emergencyOnServer,
    session: session({
      source: "typed",
      cloudSignIn: false,
      places: places.map((location) => ({ kind: "local", location, folder: location, state: "readable" })),
    }),
  };
  return answer();
});
const adoptEmergencyKey = vi.fn((id: string) => {
  const current = emergencyOnServer.session;
  if (!current || id !== current.id) return refused("emergency-ended");
  emergencyOnServer = {
    backupKey: "adopted",
    source: current.source,
    adoptedAt: 1_790_000_000,
    undoable: true,
    session: { ...current, refusal: "adopt-same-key" },
  };
  return answer();
});
// The kit's sign-ins reach the place that needed one, which the server tries
// again. Over this server's own it asks first.
const storeEmergencySignIns = vi.fn((id: string, replace = false) => {
  const current = emergencyOnServer.session;
  if (!current || id !== current.id) return refused("emergency-ended");
  if ((current.replaces.cloudSignIn || current.replaces.rcloneConfig) && !replace) return refused("sign-ins-exist");
  const places = current.places.map((place) => (place.state === "sign-in" ? { ...place, state: "readable" as const } : place));
  emergencyOnServer = { ...emergencyOnServer, session: { ...current, places } };
  return answer();
});
const finishEmergency = vi.fn((id: string) => {
  void id;
  emergencyOnServer = { ...emergencyOnServer, session: undefined, lost: undefined };
  return answer();
});
const cancelEmergency = vi.fn((id: string) => {
  void id;
  emergencyOnServer = { ...emergencyOnServer, session: undefined };
  return answer();
});
// Whether this install is in a group, and what the group answers a request
// for a deposit with.
let inGroup = false;
const emergencyFromGroup = vi.fn((memberId: string, ownerId: string, code: string, ask: boolean) => {
  void memberId;
  void ownerId;
  void code;
  void ask;
  emergencyOnServer = { ...emergencyOnServer, session: session({ source: "group" }) };
  return Promise.resolve({ ok: true, release: "allowed", emergency: emergencyOnServer });
});

const undoEmergency = vi.fn(() => {
  if (!emergencyOnServer.undoable) return refused("adopt-in-use");
  emergencyOnServer = { backupKey: "own", undoable: false };
  return answer();
});

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: {
          ...SETTINGS,
          configPath: "backups/config",
          configOffsite: "",
          containersPath: "backups/containers",
          vmsPath: "backups/vms",
          flashPath: "backups/flash",
          filesPath: "backups/files",
          zfsPath: "backups/zfs",
          containersOffsite: "",
          vmsOffsite: "",
          flashOffsite: "",
          filesOffsite: "",
          zfsOffsite: "",
          encryptionEnabled: true,
        },
        hostMountRoot: "/host/user",
      }),
    putSettings: (s: unknown) => putSettings(s),
    getEmergency: () => answer(),
    readEmergencyKit: (...a: unknown[]) => readEmergencyKit(...(a as [string, string?])),
    readEmergencyKey: (...a: unknown[]) => readEmergencyKey(...(a as [string, string[], CloudCreds?])),
    adoptEmergencyKey: (id: string) => adoptEmergencyKey(id),
    storeEmergencySignIns: (id: string, replace?: boolean) => storeEmergencySignIns(id, replace),
    finishEmergency: (id: string) => finishEmergency(id),
    cancelEmergency: (id: string) => cancelEmergency(id),
    undoEmergency: () => undoEmergency(),
    getGroup: () => Promise.resolve({ ok: true, active: inGroup, members: [] }),
    listOfferedDeposits: () =>
      Promise.resolve({
        ok: true,
        deposits: [{ memberId: "m-nas", memberName: "nas", ownerId: "o-tower", ownerName: "tower", updatedAt: "2026-09-14T03:00:00Z", newest: true }],
        silent: [],
      }),
    emergencyFromGroup: (...a: unknown[]) => emergencyFromGroup(...(a as [string, string, string, boolean])),
    discoverAll: () => discoverAll(),
    listContainers: () => Promise.resolve({ ok: true, containers: containersOnServer }),
    listVMs: () => Promise.resolve({ ok: true, vms: vmsOnServer }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: setsOnServer }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: runsOnServer }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower" }),
    getRclone: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
    restore: (...a: unknown[]) => restore(...(a as [string, string, boolean])),
    restoreVM: (...a: unknown[]) => restoreVM(...(a as [string])),
    restoreFileSet: (...a: unknown[]) => restoreFileSet(...(a as [string])),
    restoreConfig: (snapshot: string, source?: string, emergency?: string) => restoreConfig(snapshot, source, emergency),
    checkRestore: () => Promise.resolve({ ok: true, ready: true, checks: [], plan: null }),
    fileSetSnapshots: (...a: unknown[]) => fileSetSnapshots(...(a as [string, string?])),
    emergencyOffsiteEntries: () => Promise.resolve({ ok: true, entries: offsiteOnServer }),
    listDbDumps: () => Promise.resolve({ ok: true, dumps: dumpsOnServer }),
    importDbDump: (...a: unknown[]) => importDbDump(...(a as [string])),
    downloadRecoveryKit: () => Promise.resolve(null),
  };
});

const { EmergencyWindow } = await import("./EmergencyWindow");

function gone(over: Partial<Container> = {}): Container {
  return containerView({ name: "sonarr", state: "not-installed", installed: false, lastBackup: 1_700_000_000, ...over });
}

const onClose = vi.fn();
const onOldFolder = vi.fn();
const onPairing = vi.fn();
const OLD_KEY = "ab".repeat(32);

async function open(start: EmergencyStep = "access", way?: EmergencyWay) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <EmergencyWindow start={start} way={way} onClose={onClose} onOldFolder={onOldFolder} onPairing={onPairing} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;
const picks = () => screen.queryAllByTestId("emergency-pick");
const pick = (name: string) => screen.getByRole("switch", { name }) as HTMLElement;
const isOn = (name: string) => pick(name).getAttribute("aria-checked") === "true";

beforeEach(() => {
  containersOnServer = [];
  vmsOnServer = [];
  setsOnServer = [];
  dumpsOnServer = [];
  runsOnServer = [];
  needsRuntime = [];
  offsiteOnServer = { containers: {}, vms: {}, fileSets: {} };
  emergencyOnServer = { backupKey: "own", undoable: false };
  inGroup = false;
  localStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
  vi.clearAllMocks();
});

const way = (key: "emergency.way.kit" | "emergency.way.group" | "emergency.way.key") =>
  screen.getByRole("radio", { name: new RegExp(`^${en[key]}`) });
const places = () => screen.queryAllByTestId("emergency-place");
const next = () => button(en["zfs.replica.add.next"]);
const identityField = () => document.querySelector<HTMLInputElement>('input[type="password"]');

async function upload(text: string, name = "bombvault-recovery-kit.md") {
  await press(way("emergency.way.kit"));
  await act(async () => {
    fireEvent.change(screen.getByTestId("kit-file"), { target: { files: [new File([text], name, { type: "text/markdown" })] } });
  });
  await waitFor(() => expect(readEmergencyKit).toHaveBeenCalled());
}

/** Shows the kit and waits for the places it was tried on. */
async function readKit() {
  await upload("# BombVault encryption-key recovery kit");
  await waitFor(() => expect(places().length).toBeGreaterThan(0));
}

/** Takes the key of the kit over, answering the question with yes. */
async function takeOver() {
  await readKit();
  await press(button(en["emergency.adopt"]));
  await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
  await screen.findByText(new RegExp(en["emergency.adopted"]));
}

describe("the ways in", () => {
  it("are the recovery kit, the group and the typed key, and none is needed to step on", async () => {
    await open();
    for (const key of ["emergency.way.kit", "emergency.way.group", "emergency.way.key"] as const) {
      expect(way(key).getAttribute("aria-checked")).toBe("false");
    }
    expect(within(screen.getByRole("radiogroup", { name: en["emergency.access"] })).getAllByRole("radio")).toHaveLength(3);
    expect(within(way("emergency.way.group")).getByText(en["emergency.way.groupSub"])).toBeTruthy();
    expect(next().disabled).toBe(false);
  });

  it("read an uploaded kit, say what each place answered and store nothing", async () => {
    await open();
    await upload("# BombVault encryption-key recovery kit");
    expect(readEmergencyKit).toHaveBeenCalledWith("# BombVault encryption-key recovery kit");
    await waitFor(() => expect(places()).toHaveLength(4));

    expect(screen.getByText(en["emergency.readKit"].replace("{n}", "2").replace("{total}", "4"))).toBeTruthy();
    expect(screen.getByText(new RegExp(en["emergency.notSaved"]))).toBeTruthy();
    expect(screen.getByText(new RegExp(countText(en["emergency.timeLeft"], "en", 30).replace("{n}", "30")))).toBeTruthy();
    expect(within(places()[0]).getByText(en["emergency.place.readable"])).toBeTruthy();
    expect(within(places()[2]).getByText(en["emergency.place.sign-in"])).toBeTruthy();
    expect(within(places()[2]).getByText(en["emergency.place.sign-inDo"])).toBeTruthy();
    expect(within(places()[3]).getByText(en["emergency.place.missingOutside"])).toBeTruthy();
    expect(screen.getByText(en["recovery.encEncrypted"])).toBeTruthy();

    expect(screen.queryByRole("button", { name: en["zfs.replica.add.next"] })).toBeNull();
    expect(putSettings).not.toHaveBeenCalled();
    expect(adoptEmergencyKey).not.toHaveBeenCalled();
    expect(storeEmergencySignIns).not.toHaveBeenCalled();
  });

  it("name the folder BombVault looked for when a place is missing below the mount", async () => {
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = {
        ...emergencyOnServer,
        session: session({ places: [{ domain: "vms", kind: "local", location: "/mnt/user/old/vms", folder: "old/vms", state: "missing" }] }),
      };
      return answer();
    });
    await open();
    await upload("kit");
    expect(await screen.findByText(en["emergency.place.missingDo"].replace("{folder}", "old/vms"))).toBeTruthy();
  });

  it("ask for the age identity only once the server calls the kit sealed, and keep it nowhere", async () => {
    await open();
    await press(way("emergency.way.kit"));
    expect(identityField()).toBeNull();
    await upload(SEALED, "bombvault-recovery-kit.md.age");
    await waitFor(() => expect(identityField()).not.toBeNull());
    expect(screen.getByText(en["emergency.code.kit-sealed"])).toBeTruthy();
    expect(places()).toHaveLength(0);

    fireEvent.change(identityField()!, { target: { value: "AGE-SECRET-KEY-1WRONG" } });
    await press(button(en["kit.open"]));
    expect(readEmergencyKit).toHaveBeenLastCalledWith(SEALED, "AGE-SECRET-KEY-1WRONG");
    expect(await screen.findByText(en["emergency.code.kit-not-for-identity"])).toBeTruthy();
    expect(identityField()!.value).toBe("");

    fireEvent.change(identityField()!, { target: { value: ` ${IDENTITY} ` } });
    await press(button(en["kit.open"]));
    expect(readEmergencyKit).toHaveBeenLastCalledWith(SEALED, IDENTITY);
    await waitFor(() => expect(places()).toHaveLength(4));
    expect(identityField()).toBeNull();
    expect(document.body.innerHTML).not.toContain("AGE-SECRET-KEY-1");
  });

  it.each(["not-a-kit", "kit-key-broken", "kit-no-key", "kit-identity"] as const)("say in a sentence why the kit was refused as %s", async (code) => {
    readEmergencyKit.mockImplementationOnce(() => refused(code));
    await open();
    await upload("something else");
    expect(await screen.findByText(en[`emergency.code.${code}`])).toBeTruthy();
    expect(screen.queryByText(new RegExp(`server said ${code}`))).toBeNull();
    expect(places()).toHaveLength(0);
  });

  it("turn away a file that is too large to be a kit without sending it", async () => {
    await open();
    await press(way("emergency.way.kit"));
    await act(async () => {
      fireEvent.change(screen.getByTestId("kit-file"), { target: { files: [new File([new Uint8Array(1024 * 1024 + 1)], "big.md")] } });
    });
    expect(screen.getByText(en["kit.tooLarge"])).toBeTruthy();
    expect(readEmergencyKit).not.toHaveBeenCalled();
  });

  it("try a typed key on the places that were named", async () => {
    await open();
    await press(way("emergency.way.key"));
    const connect = button(en["emergency.connect"]);
    expect(connect.disabled).toBe(true);
    fireEvent.change(screen.getByLabelText(en["emergency.key"]), { target: { value: "nonsense" } });
    await press(connect);
    expect(await screen.findByText(en["emergency.code.emergency-key"])).toBeTruthy();

    fireEvent.change(screen.getByLabelText(en["emergency.key"]), { target: { value: OLD_KEY } });
    await press(button(en["emergency.connect"]));
    expect(readEmergencyKey).toHaveBeenLastCalledWith(
      OLD_KEY,
      ["backups/containers", "backups/vms", "backups/flash", "backups/files", "backups/zfs"],
      undefined
    );
    await waitFor(() => expect(places()).toHaveLength(5));
    expect(screen.getByText(en["emergency.readKey"].replace("{n}", "5").replace("{total}", "5"))).toBeTruthy();
  });

  it("send the sign-in typed for a remote place along with the key", async () => {
    await open();
    await press(way("emergency.way.key"));
    fireEvent.change(screen.getByLabelText(en["emergency.key"]), { target: { value: OLD_KEY } });
    fireEvent.change(screen.getByLabelText(en["recovery.foreignS3KeyId"]), { target: { value: " AKIA1 " } });
    fireEvent.change(screen.getByLabelText(en["recovery.foreignS3Secret"]), { target: { value: "s3cret" } });
    await press(button(en["emergency.connect"]));
    expect(readEmergencyKey.mock.calls.at(-1)?.[2]).toMatchObject({ s3KeyId: "AKIA1", s3Secret: "s3cret", restUser: "" });
  });

  it("say that the key no longer opens when the server calls it locked", async () => {
    emergencyOnServer = { backupKey: "locked", source: "kit", adoptedAt: 1_790_000_000, undoable: true };
    await open();
    expect(await screen.findByText(en["emergency.code.backup-key-locked"])).toBeTruthy();
  });
});

describe("the way through the group", () => {
  const deposit = () => screen.getByRole("button", { name: en["emergency.group.choose"] });

  it("sends an install that is in no group to the pairing", async () => {
    await open();
    await press(way("emergency.way.group"));
    expect(await screen.findByText(en["emergency.group.noGroup"])).toBeTruthy();
    await press(button(en["emergency.group.toPairing"]));
    expect(onPairing).toHaveBeenCalledTimes(1);
  });

  it("is where a link from the pairing opens the window", async () => {
    inGroup = true;
    await open("access", "group");
    expect(way("emergency.way.group").getAttribute("aria-checked")).toBe("true");
    expect(await screen.findByText("tower")).toBeTruthy();
  });

  it("opens a deposit with the recovery code and goes on as with the kit", async () => {
    inGroup = true;
    await open();
    await press(way("emergency.way.group"));
    await waitFor(() => expect(deposit()).toBeTruthy());
    await press(deposit());
    fireEvent.change(screen.getByLabelText(en["emergency.group.code"]), { target: { value: "abcde fghjk mnpqr stvwx" } });
    await press(button(en["emergency.group.ask"]));
    expect(emergencyFromGroup).toHaveBeenCalledWith("m-nas", "o-tower", "ABCDE-FGHJK-MNPQR-STVWX", true);

    await waitFor(() => expect(places()).toHaveLength(4));
    expect(screen.getByText(en["emergency.readGroup"].replace("{n}", "2").replace("{total}", "4"))).toBeTruthy();
    expect(document.body.innerHTML).not.toContain("ABCDE");
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    await screen.findByText(new RegExp(en["emergency.adopted"]));
    await press(next());
    await press(button(en["emergency.settingsGo"]));
    await waitFor(() => expect(restoreConfig).toHaveBeenCalled());
    expect(putSettings.mock.calls[0][0]).toMatchObject({ configPath: "old/config" });
  });

  it("shows on its card a request that still waits, and opens on it", async () => {
    inGroup = true;
    localStorage.setItem("bv-emergency-asked", JSON.stringify({ memberId: "m-nas", memberName: "nas", ownerId: "o-tower", ownerName: "tower" }));
    await open();
    expect(within(way("emergency.way.group")).getByText(en["emergency.group.pending"].replace("{keeper}", "nas"))).toBeTruthy();
    expect(way("emergency.way.group").getAttribute("aria-checked")).toBe("true");
    expect(await screen.findByText(en["emergency.group.resumeText"].replace("{keeper}", "nas"))).toBeTruthy();
  });
});

describe("taking the key over", () => {
  it("asks first, says what it does and does nothing on a no", async () => {
    await open();
    await readKit();
    await press(button(en["emergency.adopt"]));
    const question = screen.getAllByRole("dialog").at(-1)!;
    expect(within(question).getByText(en["emergency.adoptAsk"])).toBeTruthy();
    await press(within(question).getByRole("button", { name: en["common.cancel"] }));
    expect(adoptEmergencyKey).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: en["zfs.replica.add.next"] })).toBeNull();
  });

  it("stores the key on a yes and says until when that can be undone", async () => {
    await open();
    await takeOver();
    expect(adoptEmergencyKey).toHaveBeenCalledTimes(1);
    expect(screen.getByText(new RegExp(en["emergency.undoable"]))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["emergency.adopt"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["emergency.another"] })).toBeNull();
    expect(next().disabled).toBe(false);
  });

  it("is refused for a server with backups of its own and leads to the old backup folder", async () => {
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ refusal: "adopt-own-backups" }) };
      return answer();
    });
    await open();
    await upload("kit");
    expect(await screen.findByText(en["emergency.code.adopt-own-backups"])).toBeTruthy();
    expect(button(en["emergency.adopt"]).disabled).toBe(true);
    await press(button(en["restorePage.oldFolder"]));
    expect(cancelEmergency).toHaveBeenCalledWith(SESSION);
    expect(finishEmergency).not.toHaveBeenCalled();
    expect(onOldFolder).toHaveBeenCalledTimes(1);
  });

  it("is not offered for a key that opens none of the places", async () => {
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = {
        ...emergencyOnServer,
        session: session({ refusal: "adopt-nothing-readable", places: [{ ...KIT_PLACES[0], state: "wrong-key" }] }),
      };
      return answer();
    });
    await open();
    await upload("kit");
    expect(await screen.findByText(en["emergency.code.adopt-nothing-readable"])).toBeTruthy();
    expect(within(places()[0]).getByText(en["emergency.place.wrong-keyDo"])).toBeTruthy();
    expect(button(en["emergency.adopt"]).disabled).toBe(true);
  });

  it("is skipped when the server already backs up under that key", async () => {
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ refusal: "adopt-same-key" }) };
      return answer();
    });
    await open();
    await upload("kit");
    expect(await screen.findByText(en["emergency.code.adopt-same-key"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["emergency.adopt"] })).toBeNull();
    expect(next().disabled).toBe(false);
  });

  it("saves the sign-ins of the kit only when asked", async () => {
    await open();
    await takeOver();
    expect(storeEmergencySignIns).not.toHaveBeenCalled();
    await press(button(en["emergency.signInsSave"]));
    expect(storeEmergencySignIns).toHaveBeenCalledTimes(1);
    expect(await screen.findByText(en["emergency.signInsSaved"])).toBeTruthy();
  });

  it("offers no sign-ins a kit does not carry, and the forms for a place that lacks one", async () => {
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ cloudSignIn: false }) };
      return answer();
    });
    await open();
    await upload("kit");
    await waitFor(() => expect(places()).toHaveLength(4));
    expect(screen.queryByText(en["emergency.signInByHand"])).toBeNull();
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    expect(await screen.findByText(en["emergency.signInByHand"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["emergency.signInsSave"] })).toBeNull();
  });
});

describe("what the kit says about itself", () => {
  it("names the server it was written for and when, before anything is taken over", async () => {
    await open();
    await readKit();
    const when = formatRecent(1_789_000_000, "en");
    expect(screen.getByTestId("emergency-kit").textContent).toBe(en["emergency.kitOf"].replace("{server}", "tower").replace("{when}", when));
  });

  it("names a member that keeps a newer deposit and leads to asking that one", async () => {
    inGroup = true;
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ source: "group", kit: { ownerId: "o-tower", server: "tower", writtenAt: 1_789_000_000, newerWith: "attic" } }) };
      return answer();
    });
    await open();
    await upload("kit");
    expect(await screen.findByText(en["emergency.newerWith"].replaceAll("{member}", "attic").replace("{server}", "tower"))).toBeTruthy();
    await press(button(en["emergency.askNewer"].replace("{member}", "attic")));
    expect(cancelEmergency).toHaveBeenCalledWith(SESSION);
    expect(adoptEmergencyKey).not.toHaveBeenCalled();
    expect(way("emergency.way.group").getAttribute("aria-checked")).toBe("true");
    expect(await screen.findByText("tower")).toBeTruthy();
  });

  it("says nothing of itself for a typed key", async () => {
    await open();
    await press(way("emergency.way.key"));
    readEmergencyKey.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ source: "typed", kit: {} }) };
      return answer();
    });
    fireEvent.change(screen.getByLabelText(en["emergency.key"]), { target: { value: OLD_KEY } });
    await press(button(en["emergency.connect"]));
    await waitFor(() => expect(places().length).toBeGreaterThan(0));
    expect(screen.queryByTestId("emergency-kit")).toBeNull();
  });
});

describe("the sign-ins of the kit", () => {
  it("are tried on the places again once saved, which then say what they answered", async () => {
    await open();
    await takeOver();
    expect(within(places()[2]).getByText(en["emergency.place.sign-in"])).toBeTruthy();
    await press(button(en["emergency.signInsSave"]));
    expect(storeEmergencySignIns).toHaveBeenCalledWith(SESSION, false);
    expect(await screen.findByText(en["emergency.signInsSaved"])).toBeTruthy();
    expect(within(places()[2]).getByText(en["emergency.place.readable"])).toBeTruthy();
  });

  it("replace this server's own only after the person agrees, naming which", async () => {
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ rcloneConfig: true, replaces: { cloudSignIn: true, rcloneConfig: true } }) };
      return answer();
    });
    await open();
    await upload("kit");
    await waitFor(() => expect(places()).toHaveLength(4));
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    await screen.findByText(new RegExp(en["emergency.adopted"]));
    const list = `${en["emergency.replaces.cloud"]} and ${en["emergency.replaces.rclone"]}`;
    expect(screen.getByText(en["emergency.signInsReplaces"].replace("{list}", list))).toBeTruthy();

    await press(button(en["emergency.signInsSave"]));
    expect(storeEmergencySignIns).toHaveBeenCalledTimes(1);
    const ask = screen.getAllByRole("dialog").at(-1)!;
    expect(ask.textContent).toContain(en["emergency.signInsReplaceAsk"].replace("{list}", list));
    expect(screen.queryByText(en["emergency.code.sign-ins-exist"])).toBeNull();
    await press(within(ask).getByRole("button", { name: en["common.cancel"] }));
    expect(storeEmergencySignIns).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(en["emergency.signInsSaved"])).toBeNull();

    await press(button(en["emergency.signInsSave"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.signInsReplace"] }));
    expect(storeEmergencySignIns).toHaveBeenLastCalledWith(SESSION, true);
    expect(await screen.findByText(en["emergency.signInsSaved"])).toBeTruthy();
  });
});

describe("a step the phone app's session may not take", () => {
  it("asks for a sign-in at this instance instead of saying it was refused", async () => {
    adoptEmergencyKey.mockImplementationOnce(() => refused("own-login-needed"));
    await open();
    await readKit();
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    const ask = screen.getAllByRole("dialog").at(-1)!;
    expect(ask.textContent).toContain(en["secrets.ownLoginNeeded"]);
    expect(within(ask).getByRole("button", { name: en["secrets.signInHere"] })).toBeTruthy();
    expect(screen.queryByText(/server said own-login-needed/)).toBeNull();
  });

  it("says that a backup or restore has to end first", async () => {
    adoptEmergencyKey.mockImplementationOnce(() => refused("adopt-busy"));
    await open();
    await readKit();
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    expect(await screen.findByText(en["emergency.code.adopt-busy"])).toBeTruthy();
  });
});

describe("the time an emergency has", () => {
  it("is counted down, and the window goes back to the start when it is over", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    readEmergencyKit.mockImplementationOnce(() => {
      emergencyOnServer = { ...emergencyOnServer, session: session({ expiresAt: Math.floor(Date.now() / 1000) + 120 }) };
      return answer();
    });
    await open();
    await upload("kit");
    await waitFor(() => expect(places()).toHaveLength(4));
    expect(screen.getByText(new RegExp(countText(en["emergency.timeLeft"], "en", 2).replace("{n}", "2")))).toBeTruthy();

    emergencyOnServer = { backupKey: "own", undoable: false };
    await act(() => vi.advanceTimersByTimeAsync(121_000));
    expect(await screen.findByText(en["emergency.code.emergency-ended"])).toBeTruthy();
    expect(places()).toHaveLength(0);
    expect(way("emergency.way.kit")).toBeTruthy();
  });

  it("ends a step that comes too late, keeping the places that were typed and not the key", async () => {
    await open();
    await press(way("emergency.way.key"));
    fireEvent.change(screen.getByLabelText(en["emergency.key"]), { target: { value: OLD_KEY } });
    fireEvent.change(screen.getByLabelText(en["recovery.foreignS3Secret"]), { target: { value: "s3cret" } });
    fireEvent.change(screen.getByDisplayValue("backups/containers"), { target: { value: "old/containers" } });
    await press(button(en["emergency.connect"]));
    await waitFor(() => expect(places()).toHaveLength(5));

    adoptEmergencyKey.mockImplementationOnce(() => {
      emergencyOnServer = { backupKey: "own", undoable: false };
      return refused("emergency-ended");
    });
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    expect(await screen.findByText(en["emergency.code.emergency-ended"])).toBeTruthy();
    expect((screen.getByLabelText(en["emergency.key"]) as HTMLInputElement).value).toBe("");
    expect((screen.getByLabelText(en["recovery.foreignS3Secret"]) as HTMLInputElement).value).toBe("");
    expect(screen.getByDisplayValue("old/containers")).toBeTruthy();
  });
});

describe("leaving the window", () => {
  it("before the key is taken over drops it from memory and asks nothing", async () => {
    await open();
    await readKit();
    await press(button(en["common.cancel"]));
    expect(cancelEmergency).toHaveBeenCalledWith(SESSION);
    expect(finishEmergency).not.toHaveBeenCalled();
    expect(undoEmergency).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("with another way chosen drops the key and offers the ways again", async () => {
    await open();
    await readKit();
    await press(button(en["emergency.another"]));
    expect(cancelEmergency).toHaveBeenCalledWith(SESSION);
    await waitFor(() => expect(places()).toHaveLength(0));
    expect(way("emergency.way.kit").getAttribute("aria-checked")).toBe("false");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("after the key is taken over asks whether to keep it, and undoes it when told to", async () => {
    await open();
    await takeOver();
    await press(button(en["common.cancel"]));
    expect(screen.getByText(en["emergency.leaveAsk"])).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
    await press(button(en["emergency.undo"]));
    expect(undoEmergency).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(finishEmergency).not.toHaveBeenCalled();
  });

  it("keeps the key that was taken over when told to", async () => {
    await open();
    await takeOver();
    await press(button(en["common.cancel"]));
    await press(button(en["emergency.keep"]));
    expect(finishEmergency).toHaveBeenCalledWith(SESSION);
    expect(undoEmergency).not.toHaveBeenCalled();
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });

  it("says that the key stays once a backup was written with it", async () => {
    await open();
    await takeOver();
    await press(button(en["common.cancel"]));
    emergencyOnServer = { ...emergencyOnServer, undoable: false };
    await press(button(en["emergency.undo"]));
    expect((await screen.findAllByText(en["emergency.code.adopt-in-use"])).length).toBeGreaterThan(0);
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.queryByText(en["emergency.leaveAsk"])).toBeNull();
  });

  it("asks nothing where the takeover can no longer be undone", async () => {
    adoptEmergencyKey.mockImplementationOnce(() => {
      emergencyOnServer = {
        backupKey: "adopted",
        source: "kit",
        adoptedAt: 1_790_000_000,
        undoable: false,
        session: session({ refusal: "adopt-same-key" }),
      };
      return answer();
    });
    await open();
    await readKit();
    await press(button(en["emergency.adopt"]));
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["emergency.adopt"] }));
    expect(await screen.findByText(new RegExp(en["emergency.code.adopt-in-use"]))).toBeTruthy();
    await press(button(en["common.cancel"]));
    expect(finishEmergency).toHaveBeenCalledWith(SESSION);
    expect(cancelEmergency).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe("the settings", () => {
  it("are brought back from the place that was named, and say when the restart is left to the user", async () => {
    await open("settings");
    await press(button(en["emergency.settingsGo"]));
    await waitFor(() => expect(restoreConfig).toHaveBeenCalledWith("latest", undefined, undefined));
    expect(putSettings.mock.calls[0][0]).toMatchObject({ configPath: "backups/config" });
    expect(await screen.findByText(en["recovery.configManualRestart"])).toBeTruthy();
    expect(screen.queryByText(new RegExp(en["emergency.loginStays"]))).toBeNull();
  });

  it("come from where the kit says they are and leave this server's login alone", async () => {
    await open();
    await takeOver();
    await press(next());
    expect(screen.getByText(new RegExp(en["emergency.loginStays"]))).toBeTruthy();
    await press(button(en["emergency.settingsGo"]));
    await waitFor(() => expect(restoreConfig).toHaveBeenCalledWith("latest", undefined, SESSION));
    expect(putSettings.mock.calls[0][0]).toMatchObject({ configPath: "old/config" });
  });

  it("name the first step when they were sealed with another server's key", async () => {
    restoreConfig.mockResolvedValueOnce({ ok: false, code: "settings-other-key", error: "these settings were sealed with the APP_KEY of another server" } as never);
    await open("settings");
    await press(button(en["emergency.settingsGo"]));
    expect((await screen.findAllByText(en["emergency.code.settings-other-key"])).length).toBeGreaterThan(0);
  });

  it("can be skipped", async () => {
    await open("settings");
    await press(screen.getByRole("radio", { name: new RegExp(`^${en["recovery.configSkip"]}`) }));
    await press(button(en["zfs.replica.add.next"]));
    expect(restoreConfig).not.toHaveBeenCalled();
    expect(putSettings).not.toHaveBeenCalled();
    await waitFor(() => expect(discoverAll).toHaveBeenCalled());
  });

  it("when skipped point this server at the places of the kit before the search", async () => {
    await open();
    await takeOver();
    await press(next());
    await press(screen.getByRole("radio", { name: new RegExp(`^${en["recovery.configSkip"]}`) }));
    await press(next());
    await waitFor(() => expect(putSettings).toHaveBeenCalledTimes(1));
    expect(putSettings.mock.calls[0][0]).toMatchObject({
      containersPath: "old/containers",
      vmsOffsite: "rclone:box:bombvault/vms",
      filesPath: "backups/files",
      encryptionEnabled: true,
      restoreFolder: SETTINGS.restoreFolder,
    });
    await waitFor(() => expect(discoverAll).toHaveBeenCalled());
  });
});

describe("the selection", () => {
  it("looks through the backups when the step opens", async () => {
    containersOnServer = [gone(), gone({ name: "radarr" })];
    await open("selection");
    expect(discoverAll).toHaveBeenCalledTimes(1);
    expect(screen.getByText(en["recovery.foundCounts"].replace("{c}", "2").replace("{v}", "0"))).toBeTruthy();
    expect(picks()).toHaveLength(2);
  });

  it("looks again after the first step named other places", async () => {
    containersOnServer = [gone()];
    await open("selection");
    expect(discoverAll).toHaveBeenCalledTimes(1);
    await press(screen.getByRole("radio", { name: en["emergency.access"] }));
    await takeOver();
    await press(next());
    await press(screen.getByRole("radio", { name: new RegExp(`^${en["recovery.configSkip"]}`) }));
    await press(next());
    await waitFor(() => expect(discoverAll).toHaveBeenCalledTimes(2));
  });

  it("names what the settings restore could not bring over", async () => {
    emergencyOnServer = {
      backupKey: "adopted",
      source: "kit",
      adoptedAt: 1_790_000_000,
      undoable: true,
      lost: { count: 2, kinds: { cloud: 1, peers: 1 } },
    };
    await open("selection");
    const note = await screen.findByText(new RegExp(countText(en["emergency.lost"], "en", 2)));
    expect(note.textContent).toContain(en["emergency.lost.cloud"]);
    expect(note.textContent).toContain(en["emergency.lost.peers"]);
    expect(note.textContent).not.toContain(en["emergency.lost.rclone"]);
  });

  it("starts with what this server lacks switched on and what it has switched off", async () => {
    containersOnServer = [gone(), containerView({ name: "plex" })];
    vmsOnServer = [vmView({ state: "not-installed" })];
    await open("selection");
    expect(isOn("sonarr")).toBe(true);
    expect(isOn("plex")).toBe(false);
    expect(isOn("Windows 11")).toBe(true);
    expect(screen.getByText(new RegExp(countText(en["emergency.count.containers"], "en", 1)))).toBeTruthy();
  });

  it("says so when nothing was found", async () => {
    await open("selection");
    expect(screen.getByText(en["recovery.foundNone"])).toBeTruthy();
    expect(button(en["snapshots.restore"]).disabled).toBe(true);
  });

  it("lists the containers that exist as dumps alone apart from the others", async () => {
    containersOnServer = [gone(), gone({ name: "immich_postgres", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
    await open("selection");
    const apart = screen.getByText(en["recovery.dumpOnlyTitle"]).closest("section") as HTMLElement;
    expect(apart.textContent).toContain("immich_postgres");
    expect(apart.textContent).not.toContain("sonarr");
    expect(picks()).toHaveLength(1);
  });

  it("brings a dump-only container back with its data from its own row", async () => {
    containersOnServer = [gone({ name: "immich_postgres", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
    dumpsOnServer = [
      {
        id: "d1",
        time: "2026-09-23T19:10:19Z",
        engine: "postgres",
        image: "postgres:16",
        version: "16.4",
        databases: ["immich"],
        bytes: 5_000,
        damaged: false,
      },
    ];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["recovery.restoreAndImport"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls[0]).toEqual(["immich_postgres", "definition", true, undefined, undefined, false]);
    expect(importDbDump.mock.calls[0]).toEqual(["immich_postgres", "d1"]);
    expect(screen.getByText(en["dbdump.importDone"])).toBeTruthy();
  });

  it("offers a dump-only container again without GPU and runtime", async () => {
    containersOnServer = [gone({ name: "plex", dbTier: "curated", dbEngine: "postgres", dumpOnly: true })];
    needsRuntime = ["plex"];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["recovery.restoreAndImport"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(document.body.textContent).toContain(en["restore.noRuntime"].replace("{name}", "plex"));
    fireEvent.click(button(en["restore.withoutRuntime"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls.at(-1)?.[5]).toBe(true);
  });
});

describe("the restore", () => {
  it("brings back what is switched on, one after the other and left stopped", async () => {
    containersOnServer = [gone(), gone({ name: "immich_postgres", dumpOnly: true })];
    vmsOnServer = [vmView({ state: "not-installed" })];
    setsOnServer = [fileSetView({ path: "" })];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(restore.mock.calls.map((c) => c[0])).toEqual(["sonarr"]);
    expect(restore.mock.calls[0]).toEqual(["sonarr", "latest", true, undefined, true]);
    expect(restoreVM.mock.calls[0]).toEqual(["win11", "latest", true, undefined, true]);
    expect(restoreFileSet.mock.calls[0]).toEqual([setsOnServer[0].id, "f-new", true, "user/bombvault/restore/Photos", undefined]);
    expect(screen.getAllByTestId("emergency-run")).toHaveLength(3);
    expect(screen.getByText(en["emergency.result"].replace("{ok}", "3").replace("{fail}", "0"))).toBeTruthy();
    expect(button(en["recovery.kitDownload"])).toBeTruthy();

    expect(finishEmergency).not.toHaveBeenCalled();
    fireEvent.click(button(en["common.done"]));
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(finishEmergency).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("asks first when it would replace something this server still has", async () => {
    containersOnServer = [containerView({ name: "plex" })];
    await open("selection");
    await press(pick("plex"));
    await press(button(en["snapshots.restore"]));
    expect(restore).not.toHaveBeenCalled();
    expect(screen.getByText(new RegExp(en["backups.restore.alsoVMs"]))).toBeTruthy();
    await press(within(screen.getAllByRole("dialog").at(-1)!).getByRole("button", { name: en["common.cancel"] }));
    expect(restore).not.toHaveBeenCalled();
  });

  it("starts what came back when it is not left stopped", async () => {
    containersOnServer = [gone()];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(screen.getByRole("switch", { name: en["restoreWindow.leaveStopped"] }));
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls[0]).toEqual(["sonarr", "latest", true, undefined, false]);
  });

  it("lists and restores from the off-site copies when the local backups are gone", async () => {
    containersOnServer = [gone({ lastBackup: null })];
    vmsOnServer = [vmView({ state: "not-installed", lastBackup: null })];
    setsOnServer = [fileSetView({ path: "" })];
    offsiteOnServer = { containers: { sonarr: 1_790_000_000 }, vms: { win11: 1_790_000_000 }, fileSets: { Photos: 1_790_000_000 } };
    vi.useFakeTimers();
    await open("selection");
    expect(screen.queryByRole("switch", { name: "sonarr" })).toBeNull();

    fireEvent.click(screen.getByRole("tab", { name: en["source.offsite"] }));
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(isOn("sonarr")).toBe(true);
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(30_000));

    expect(restore.mock.calls[0]).toEqual(["sonarr", "latest", true, "offsite", true]);
    expect(restoreVM.mock.calls[0]).toEqual(["win11", "latest", true, "offsite", true]);
    expect(fileSetSnapshots.mock.calls[0]).toEqual([setsOnServer[0].id, "offsite"]);
    expect(restoreFileSet.mock.calls[0]).toEqual([setsOnServer[0].id, "f-new", true, "user/bombvault/restore/Photos", "offsite"]);
  });

  it("holds the page and the window while it runs, keeps what has not started, and can stop", async () => {
    containersOnServer = [gone(), gone({ name: "radarr" }), gone({ name: "lidarr" })];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(0));

    const leaving = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(leaving);
    expect(leaving.defaultPrevented).toBe(true);
    expect(JSON.parse(localStorage.getItem("bv-restore-many-left") ?? "{}").names).toEqual(["radarr", "lidarr"]);

    fireEvent.keyDown(document, { key: "Escape" });
    expect(document.body.textContent).toContain(en["restoreMany.held"].replace("{stop}", en["restoreMany.stop"]));
    expect(onClose).not.toHaveBeenCalled();

    await act(() => vi.advanceTimersByTimeAsync(1_000));
    fireEvent.click(button(en["restoreMany.stop"]));
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(restore.mock.calls.length).toBeLessThan(3);
    expect(document.body.textContent).toContain(countText(en["restoreMany.notStarted"], "en", 3 - restore.mock.calls.length).replace("{n}", String(3 - restore.mock.calls.length)));
    expect(localStorage.getItem("bv-restore-many-left")).toBeNull();
    const after = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(after);
    expect(after.defaultPrevented).toBe(false);
  });

  it("offers the containers Docker refused again without GPU and runtime", async () => {
    containersOnServer = [gone({ name: "plex" }), gone()];
    needsRuntime = ["plex"];
    vi.useFakeTimers();
    await open("selection");
    fireEvent.click(button(en["snapshots.restore"]));
    await act(() => vi.advanceTimersByTimeAsync(20_000));

    expect(screen.getByText(en["emergency.result"].replace("{ok}", "1").replace("{fail}", "1"))).toBeTruthy();
    expect(document.body.textContent).toContain(en["restore.noRuntimeList"].replace("{names}", "plex"));
    fireEvent.click(button(en["restore.withoutRuntime"]));
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(restore.mock.calls.filter((c) => c[5] === true).map((c) => c[0])).toEqual(["plex"]);
  });
});
