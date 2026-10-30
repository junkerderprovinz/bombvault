// The mapping is keyed off translation keys so the same verb gets the same
// symbol wherever it appears, and a specific rule wins over a general one.
import { describe, expect, it } from "vitest";
import { glyphFor } from "./glyphFor";

describe("glyphFor", () => {
  it("gives one verb one symbol across different pages", () => {
    const a = glyphFor("containers.backupNow");
    const b = glyphFor("containers.backupSelected");
    const c = glyphFor("files.backupAll");
    expect(a).toEqual(b);
    expect(a).toEqual(c);
  });

  it("keeps distinct verbs distinct", () => {
    expect(glyphFor("snapshots.delete")).not.toEqual(glyphFor("snapshots.restore"));
    expect(glyphFor("offsite.test")).not.toEqual(glyphFor("files.addSet"));
  });

  it("lets a specific rule win over a general one", () => {
    // "backupSelected" is a backup, not a selection.
    expect(glyphFor("containers.backupSelected")).toEqual(glyphFor("containers.backupNow"));
    // "unlock" is not "lock"/"key".
    expect(glyphFor("integrity.unlock")).not.toEqual(glyphFor("signIns.add"));
  });

  it("gives every Got it the check of an agreement", () => {
    expect(glyphFor("common.gotIt")).toEqual(glyphFor("common.confirm"));
  });

  it("gives a confirmation the symbol of what it confirms", () => {
    expect(glyphFor("fleet.confirmRemove")).toEqual(glyphFor("fleet.remove"));
    expect(glyphFor("common.confirm")).toEqual(glyphFor("fleet.mesh.accept"));
    expect(glyphFor("common.confirm")).not.toEqual(glyphFor("common.save"));
  });

  it("points the jump to the newest log line down, not forward", () => {
    expect(glyphFor("activityLog.jumpToLatest")).toBeDefined();
    expect(glyphFor("activityLog.jumpToLatest")).not.toEqual(glyphFor("wizard.next"));
  });

  it("returns undefined rather than a meaningless symbol", () => {
    // A key nothing sensible matches keeps its text in glyph mode, which is
    // the better failure: a wrong symbol is worse than a word.
    expect(glyphFor("zzz.somethingWithNoVerb")).toBeUndefined();
  });
});

describe("glyphFor on the About card", () => {
  it("gives the mail button a symbol", () => {
    expect(glyphFor("about.mail")).toBeDefined();
  });

  it("gives the coffee button no glyph", () => {
    // The only coffee mark is Buy Me a Coffee's own, which the button carries
    // as its vendor artwork. A pattern on "coffee" or "donate" would put that
    // brand on anything mentioning either word.
    expect(glyphFor("about.coffeeButton")).toBeUndefined();
    expect(glyphFor("about.donateAppeal")).toBeUndefined();
  });

  it("gives the repository button no glyph", () => {
    // It wears GitHub's own mark, passed at its one call site. A pattern on
    // "repo" would put that logo on repository settings that have nothing to
    // do with GitHub.
    expect(glyphFor("about.repo")).toBeUndefined();
    expect(glyphFor("offsite.repoUrl")).toBeUndefined();
  });

  it("does not let the mail rule swallow unrelated keys", () => {
    // The rule is anchored on a dot so it takes `about.mail` and
    // `settings.mailFrom`, not every key that happens to contain the letters.
    expect(glyphFor("recovery.emailless")).toBeUndefined();
  });
});

describe("glyphFor on credentials", () => {
  it("marks revoking a key apart from switching two-factor off", () => {
    expect(glyphFor("mcp.revoke")).toBeDefined();
    expect(glyphFor("mcp.revoke")).not.toEqual(glyphFor("auth.twoFactorDisable"));
    expect(glyphFor("mcp.revoke")).not.toEqual(glyphFor("mcp.rotate"));
  });

  it("gives removing a passkey the same mark as revoking a key", () => {
    expect(glyphFor("auth.passkeyRemove")).toEqual(glyphFor("mcp.revoke"));
  });

  it("gives undo a mark of its own, apart from restoring a backup", () => {
    expect(glyphFor("emergency.undo")).toEqual(glyphFor("takeover.undo"));
    expect(glyphFor("emergency.undo")).not.toEqual(glyphFor("snapshots.restore"));
  });

  it("marks a forgotten password apart from setting one", () => {
    expect(glyphFor("auth.forgotPassword")).toBeDefined();
    expect(glyphFor("auth.forgotPassword")).not.toEqual(glyphFor("auth.setPassword"));
  });

  it("marks turning the login off like turning two-factor off", () => {
    expect(glyphFor("auth.loginDisable")).toEqual(glyphFor("auth.twoFactorDisable"));
  });
});

describe("glyphFor on close meanings", () => {
  it("tells trying again from going back to the default", () => {
    expect(glyphFor("anomaly.retry")).toEqual(glyphFor("pairing.checkAgain"));
    expect(glyphFor("anomaly.retry")).toEqual(glyphFor("restoreCheck.recheck"));
    expect(glyphFor("placement.reset")).toEqual(glyphFor("backupOrder.reset"));
    expect(glyphFor("anomaly.retry")).not.toEqual(glyphFor("placement.reset"));
  });

  it("tells reading in and writing out from moving a file", () => {
    expect(glyphFor("settingsIO.importButton")).not.toEqual(glyphFor("fleet.mesh.send"));
    expect(glyphFor("settingsIO.exportButton")).toEqual(glyphFor("entry.export.action"));
    expect(glyphFor("settingsIO.exportButton")).not.toEqual(glyphFor("dbdump.download"));
  });

  it("gives Done, Confirm, Got it and Allow one mark, apart from a test", () => {
    for (const key of ["common.gotIt", "common.confirm", "zfs.receive.allow", "anomaly.action.acknowledge"]) {
      expect(glyphFor(key), key).toEqual(glyphFor("common.done"));
    }
    expect(glyphFor("common.done")).not.toEqual(glyphFor("offsite.test"));
  });

  it("takes the Done mark where the key also names an add", () => {
    expect(glyphFor("zfs.add.done")).toEqual(glyphFor("common.done"));
  });

  it("refuses with a mark apart from closing", () => {
    expect(glyphFor("fleet.mesh.decline")).toEqual(glyphFor("oauth.decline"));
    expect(glyphFor("fleet.mesh.decline")).not.toEqual(glyphFor("common.close"));
  });

  it("applies the one shown apart from saving", () => {
    expect(glyphFor("folder.use")).toEqual(glyphFor("excludes.presetApply"));
    expect(glyphFor("folder.use")).toEqual(glyphFor("zfs.replica.takeOver"));
    expect(glyphFor("folder.use")).not.toEqual(glyphFor("settings.save"));
    expect(glyphFor("takeover.decline")).not.toEqual(glyphFor("takeover.accept"));
  });

  it("sends a test message apart from testing a connection", () => {
    expect(glyphFor("notify.test")).toBeDefined();
    expect(glyphFor("notify.test")).not.toEqual(glyphFor("offsite.test"));
  });

  it("shows a restore check's files as a view", () => {
    expect(glyphFor("restoreCheck.plan.showFiles")).toEqual(glyphFor("snapshots.show"));
  });
});
