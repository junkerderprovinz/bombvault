// A failed call says why in the person's language: in a sentence of its own
// where this client can explain it, else in a general one that keeps what the
// server said apart. Never as a bare status or in the server's English.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "./i18n";

const fetchMock = vi.fn();

function answers(status: number, body: unknown, statusText = "Error") {
  fetchMock.mockImplementation(() =>
    Promise.resolve({
      ok: status >= 200 && status < 300,
      status,
      statusText,
      json: () => (body === undefined ? Promise.reject(new SyntaxError("not JSON")) : Promise.resolve(body)),
    }),
  );
}

beforeEach(() => {
  vi.resetModules();
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("a refused settings change", () => {
  it("tells a phone's session to sign in at the instance itself", async () => {
    answers(403, { ok: false, error: "sign in at this instance's web interface", code: "own-login-needed" });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(api.ApiError);
    expect((err as InstanceType<typeof api.ApiError>).message).toBe(en["auth.ownLoginNeeded"]);
    expect((err as InstanceType<typeof api.ApiError>).code).toBe("own-login-needed");
    expect(api.explainedError(err)).toBe(en["auth.ownLoginNeeded"]);
  });

  it("tells a page opened through the relay to go to the instance's own address", async () => {
    const api = await import("./api");
    answers(200, { ok: true, enabled: true, authed: true, relayed: true });
    await api.getAuth();
    expect(api.openedThroughRelay()).toBe(true);
    answers(403, { ok: false, error: "sign in at this instance's web interface", code: "own-login-needed" });
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["auth.relayLoginNeeded"]);
  });

  it("keeps what the server said apart for a refusal this client does not know", async () => {
    answers(409, { ok: false, error: "a restore is running", code: "busy" });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.refused"]);
    expect((err as InstanceType<typeof api.ApiError>).detail).toBe("a restore is running");
    expect(api.explainedError(err)).toBeUndefined();
  });

  it.each([
    ["domain-busy", "prune is running on containers", en["apiError.domainBusy"]],
    ["offsite-busy", "offsite is running on containers", en["apiError.offsiteBusy"]],
    ["batch-busy", "a batch backup is already running", en["containers.batchAlreadyRunning"]],
  ])("says in its own words why a start was refused as %s", async (code, said, sentence) => {
    answers(409, { ok: false, error: said, code });
    const api = await import("./api");
    const err = await api.backupNow("plex").catch((e: unknown) => e);
    expect((err as Error).message).toBe(sentence);
    expect(api.explainedError(err)).toBe(sentence);
  });

  it("says a replica run holds a ZFS entry that only replicates", async () => {
    answers(409, { ok: false, error: "a replica run or restore of this item is in progress", code: "replica-busy" });
    const api = await import("./api");
    const err = await api.backupZFSDataset("tank").catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.replicaBusy"]);
  });

  it("says why a ZFS start was refused with a ZFS reason", async () => {
    answers(409, { ok: false, error: "the replica server is switched off", code: "server-disabled" });
    const api = await import("./api");
    const err = await api.backupZFSDataset("tank").catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["zfs.code.server-disabled"]);
    expect((err as InstanceType<typeof api.ApiError>).code).toBe("server-disabled");
  });

  it("says a check already runs on a received repository", async () => {
    answers(409, { ok: false, error: "a check is already running on this repository", code: "check-busy" });
    const api = await import("./api");
    const err = await api.checkReceivedRepo("r1").catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.checkBusy"]);
  });

  it("calls a failure on the server one, whatever it said", async () => {
    answers(500, { ok: false, error: "database is locked" });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.serverFailed"]);
    expect((err as InstanceType<typeof api.ApiError>).detail).toBe("database is locked");
  });

  it.each([
    [502, en["apiError.gateway"]],
    [503, en["apiError.unavailable"]],
    [504, en["apiError.timeout"]],
  ])("names a proxy's %i without JSON for what it is", async (status, sentence) => {
    answers(status, undefined, "Bad Gateway");
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(sentence);
    expect(api.explainedError(err)).toBe(sentence);
  });

  it("tells a session that ran out to sign in again", async () => {
    answers(401, { ok: false, error: "authentication required" });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.signedOut"]);
  });

  it.each([
    [401, "session-ended", en["apiError.sessionEnded"]],
    [403, "phone-pairing-off", en["apiError.phonePairingOff"]],
    [413, "relay-refused", en["apiError.relayTooLarge"]],
    [403, "relay-refused", en["apiError.relayRefused"]],
  ])("says what the relay's %i %s means", async (status, code, sentence) => {
    answers(status, { ok: false, error: "the relay said no", code });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(sentence);
  });

  it("says the server cannot be reached when the call never got there", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(api.ApiError);
    expect((err as Error).message).toBe(en["apiError.offline"]);
    expect((err as InstanceType<typeof api.ApiError>).detail).toBe("Failed to fetch");
  });

  it("says in the person's language why a backup of a section with no place is refused", async () => {
    answers(409, { ok: false, error: "every storage location of this section is switched off", code: "no-place" });
    const api = await import("./api");
    const err = await api.backupFlashNow().catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.noPlace"]);
    expect(api.explainedError(err)).toBe(en["apiError.noPlace"]);
  });

  it("says in the person's language why a backup of a section with no storage location is refused", async () => {
    answers(409, { ok: false, error: "ZFS datasets has no storage location yet.", code: "no-location" });
    const api = await import("./api");
    const err = await api.backupFlashNow().catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.noLocation"]);
  });

  it("says in the person's language that another task holds the section", async () => {
    answers(409, { ok: false, error: "prune is running on containers", code: "domain-busy" });
    const api = await import("./api");
    const err = await api.backupAll(["plex"]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["apiError.domainBusy"]);
  });

  it("names the section and what runs there when the refusal says", async () => {
    answers(409, { ok: false, error: "prune is running on containers", code: "domain-busy", domain: "containers", busyWith: "prune" });
    const api = await import("./api");
    const err = await api.backupAll(["plex"]).catch((e: unknown) => e);
    expect((err as Error).message).toBe("Old backups of Containers are being cleared away right now. Try again when that has finished.");
  });

  it("leaves a call its caller stopped as the AbortError it expects", async () => {
    fetchMock.mockRejectedValue(new DOMException("aborted", "AbortError"));
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).name).toBe("AbortError");
  });
});
