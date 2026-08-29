import { useEffect, useRef, useState } from "react";
import type { Settings, DeploySnippetData, PrimaryRemoteConfig, PrimaryRemoteDomain, OffsiteDomain } from "../lib/api";
import {
  deploySnippet,
  tamperTest,
  testOffsite,
  getPrimaryRemote,
  setPrimaryRemote,
  testPrimaryRemote,
  primaryRemoteTamperTest,
  listOffsiteTargets,
  updateOffsiteTarget,
} from "../lib/api";
import type { OffsiteTarget } from "../lib/api";
import { useCloudCredSets } from "../lib/useCloudCredSets";
import { useT } from "../lib/i18n";
import { InfoBubble } from "./InfoBubble";
import { Toggle } from "./Toggle";
import { Badge } from "./Badge";
import { withLtrFragments, REPO_LOCAL_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { useToast } from "../lib/toast";
import { Button } from "./Button";

// ---------------------------------------------------------------------------
// OffsiteWizard — guided per-domain off-site setup.
//
// It does NOT own any new persistence: the repo URL/schedule + immutable flag +
// growth budget flow through the SAME `settings`/`setSettings`/`save` the Settings
// page already uses. Credentials are only SELECTED here (which named set a
// destination uses); editing them lives in Settings › Shared cloud credentials
// and in each set's own row. The wizard wraps those existing inputs in a
// step-by-step flow and adds the guided extras:
// backend choice, a rest-server deploy snippet, a connection test, an
// append-only tamper verdict, and a retention-strategy chooser.
// ---------------------------------------------------------------------------

// Both modes take all five domains. "config" (self-backup) used to be valid
// only in remote-primary mode, and this file kept its own four-domain copy of
// the type to say so. Since #176 the off-site tab lists self-backup too, and
// the stale copy is what let REPO_KEY silently miss an entry. Imported now, so
// the domain list has one definition (api.ts) rather than one per file.
type Domain = OffsiteDomain;
type T = ReturnType<typeof useT>["t"];
type SaveState = "idle" | "saving" | "saved" | "error";

// Per-domain Settings keys — off-site MODE binds to the exact same fields the
// off-site card and immutable flags already persist (no parallel state).
const REPO_KEY = {
  containers: "containersOffsite",
  vms: "vmsOffsite",
  flash: "flashOffsite",
  files: "filesOffsite",
  config: "configOffsite",
} as const;
// The off-site schedule is owned by Settings › Schedules — the wizard no longer
// edits it, so there is no SCHED_KEY map here.
const IMM_KEY = {
  containers: "containersOffsiteImmutable",
  vms: "vmsOffsiteImmutable",
  flash: "flashOffsiteImmutable",
  files: "filesOffsiteImmutable",
  config: "configOffsiteImmutable",
} as const;
// Every domain's backup PATH field — used only by remote-primary mode to read
// the LIVE path for display + backend inference; never written here (editing
// a domain's path happens on the Storage tab's FolderBrowser field itself).
const PATH_KEY: Record<Domain, keyof Settings> = {
  containers: "containersPath",
  vms: "vmsPath",
  flash: "flashPath",
  config: "configPath",
  files: "filesPath",
};

// "none" = empty URL (neutral prompt — no REST snippet, no caveat); "path" = a
// plain folder under the Host Data mount (a mounted NAS share, say — the option
// nothing in the UI used to mention, issue #138); "other" = a recognized non-REST
// scheme (sftp/b2/gs/azure) that must NOT get the REST deploy-snippet flow.
// "path" and "other" behave identically for every caveat below; they differ only
// in what Step 1 offers to explain.
type Backend = "rest" | "rclone" | "s3" | "path" | "other" | "none";

function inferBackend(url: string): Backend {
  const u = url.trim();
  if (u === "") return "none";
  if (u.startsWith("rclone:")) return "rclone";
  if (u.startsWith("s3:") || u.startsWith("s3://")) return "s3";
  if (u.startsWith("rest:") || u.startsWith("http://") || u.startsWith("https://")) return "rest";
  // No "<scheme>:" prefix at all → a local/mounted path, which restic and
  // resolveRepo both accept (relative to the Host Data mount).
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(u)) return "path";
  // Any other recognized scheme (sftp:, b2:, gs:, azure:, …) is "other": no REST
  // snippet, no rclone/s3 caveat — the wizard makes no false REST assumption.
  return "other";
}

// CopyBlock mirrors the VM-SSH card's copy pattern: a monospace <pre> with a copy
// button. Clipboard may be unavailable on a non-HTTPS origin — the text is
// selectable in that case.
//
// GlimStone follow-up pass (v8.0.0): the "copied" label-flip is now a toast,
// same shape as every other migrated copy button (UnraidTileSection/
// DashboardWidgetCard/FleetSettingsCard in Settings.tsx) — including turning
// the previously-silent clipboard failure into an explicit fail toast,
// matching those same sites.
function CopyBlock({ text, t }: { text: string; t: T }) {
  const { push } = useToast();
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      push(t("vm.ssh.copied"), "success");
    } catch {
      // clipboard unavailable (non-HTTPS) — the text is selectable in the box
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <Button
        label={t("vm.ssh.copy")}
        labelKey="vm.ssh.copy"
        tone="neutral"
        onClick={() => void copy()}
        className="shrink-0"
      />
    </div>
  );
}

export function OffsiteWizard({
  domain,
  settings,
  setSettings,
  save,
  t,
  primary = false,
  hueIndex,
}: {
  domain: Domain;
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setState: (s: SaveState) => void,
    setError: (e: string | null) => void
  ) => Promise<boolean>;
  t: T;
  /**
   * false (default) — the original off-site DESTINATION wizard: repo,
   * immutable flag and growth budget bind straight to the domain's off-site
   * Settings columns, exactly as before this prop existed.
   * true — remote-PRIMARY safety settings (issue #152): the domain's own
   * backup path (Settings.*Path, edited on the Storage tab) is itself a
   * restic remote, and this reuses the SAME dialog for its bandwidth limits/
   * append-only/growth-budget instead of duplicating the UI. The repo-URL
   * step becomes read-only (editing happens on the path field itself), the
   * retention-strategy chooser (far-side prune / maintenance window — both
   * assume a SEPARATE off-site copy standing behind the local one) is
   * replaced by a plain bandwidth+budget form, and repo/immutable/budget are
   * backed by the primary-remote-target API instead of Settings.
   */
  primary?: boolean;
  /** GlimStone follow-up pass audit fix: this domain's own enclosing Card's
   *  rainbow position (off-site tab: the SAME `hueIdx` Settings.tsx already
   *  threads into that Card's own TestConnectionButton/ReplicateNowButton/
   *  Einrichten toggle) — not a second independent value. Before this fix,
   *  the Step 3 connection-test button and Step 4 tamper-test button below
   *  never joined the colour engine at all, so opening the wizard visibly
   *  de-coloured those two actions in rainbow mode even though every OTHER
   *  clickable control for the same domain stayed hued. Optional (like
   *  TestConnectionButton's/ReplicateNowButton's own `hueIndex?`) — a caller
   *  with no single per-domain hue to offer (PathModeSwitch's remote-mode
   *  dialog, which shares one hue across five domains' worth of chrome that
   *  isn't itself hued yet) simply omits it, and both Badges below render
   *  their flat, un-rainbowed `tone="active"` look, same as any other
   *  singleton hue-eligible badge with no `hueIndex` passed. */
  hueIndex?: number;
}) {
  // Off-site mode DOES receive "config" since #176 gave self-backup the same
  // card as every other domain. It did not before, and this line used to cast
  // the domain to narrow it away, which turned the first render for
  // self-backup into a blank page: REPO_KEY had no "config" entry, so repoKey
  // was undefined, settings[undefined] was undefined, and inferBackend called
  // .trim() on it (manilx, on #182). No cast now, so leaving a domain out of
  // either map is a compile error rather than a crash in the browser.
  const offsiteDomain: OffsiteDomain = domain;
  const repoKey = REPO_KEY[offsiteDomain];
  const immKey = IMM_KEY[offsiteDomain];

  // Remote-primary mode: load the saved safety settings once (mirrors the
  // cloud-creds self-load below). primaryLoaded gates saves exactly like
  // cloudLoaded does and for the same reason — never PUT a config that was
  // not actually read from the server first (a blank round-trip would wipe
  // the stored limits/budget).
  const [primaryConfig, setPrimaryConfig] = useState<PrimaryRemoteConfig | null>(null);
  const [primaryLoaded, setPrimaryLoaded] = useState(false);
  const [primaryLoadErr, setPrimaryLoadErr] = useState<string | null>(null);
  const [pLimitUpload, setPLimitUpload] = useState(0);
  const [pLimitDownload, setPLimitDownload] = useState(0);
  const [pBudget, setPBudget] = useState(0);
  // #182: the primary path's own credential set. Carried through every
  // savePrimarySafety call below, because that PUT writes the FULL config — a
  // save that omitted this would silently clear the user's choice.
  const [pCredsRef, setPCredsRef] = useState("");

  useEffect(() => {
    if (!primary) return;
    let active = true;
    getPrimaryRemote(domain)
      .then((r) => {
        if (!active) return;
        if (!r.ok || !r.config) {
          setPrimaryLoadErr(t("offsite.wizard.credLoadError"));
          return;
        }
        setPrimaryConfig(r.config);
        setPLimitUpload(r.config.limitUpload);
        setPLimitDownload(r.config.limitDownload);
        setPBudget(r.config.growthBudgetGb);
        setPCredsRef(r.config.credsRef ?? "");
        setPrimaryLoaded(true);
      })
      .catch(() => {
        if (active) setPrimaryLoadErr(t("offsite.wizard.credLoadError"));
      });
    return () => {
      active = false;
    };
    // domain/primary are stable for a mounted dialog; t is stable for a given
    // language — the load runs once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [primary, domain]);

  // The LIVE backup path (primary mode, read-only here) vs. the persisted
  // off-site repo (off-site mode, editable in Step 3 below) — the shared
  // "repoURL" every step below reasons about (backend inference, caveats,
  // the connection test, the tamper-test's REST-only gate).
  const livePath = String(settings[PATH_KEY[domain]] ?? "");
  const repoURL = primary ? livePath : settings[repoKey];
  const immutable = primary ? (primaryConfig?.immutable ?? false) : settings[immKey];
  const [backend, setBackend] = useState<Backend>(() => inferBackend(repoURL));

  const { push } = useToast();

  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): this wizard's own repo-URL/credentials/bandwidth-budget
  // fields used to batch into their own per-step Save buttons. None of them
  // are a "draft not meant to take effect until applied" the way
  // CloudCredSetsCard's/OffsiteTargetsSection's own add-or-edit forms are
  // (both kept as genuine exceptions — see their own header comments): every
  // field here already binds straight to a real persisted value (the SAME
  // shared `settings` object in off-site mode, or the primary-remote config
  // fetched by getPrimaryRemote below), with no separate "Close = discard"
  // affordance to protect. Local debounce mirrors FlashZipExportCard's own
  // mechanism in Settings.tsx — this component has no access to
  // SettingsPage's shared debouncedSave.
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // Step 2 — rest-server deploy snippet (generated on demand, never persisted).
  const [snippet, setSnippet] = useState<DeploySnippetData | null>(null);
  // GlimStone follow-up pass (v8.0.0): the "error" resting state below is gone
  // — a failed generate/regenerate now pushes a toast and resets straight to
  // idle (see genSnippet), so only the busy/idle distinction is left to track.
  const [snipBusy, setSnipBusy] = useState(false);

  // The wizard no longer holds any shared-credential state. It used to load,
  // edit and save the shared cloud credentials inline whenever the backend was
  // REST; since #176 it only SELECTS which credentials a destination uses, and
  // editing them belongs to Settings › Shared cloud credentials and to each
  // set's own row. That removes the second editor for one pair of values.

  // Step 3 — which credentials this domain's PRIMARY off-site destination uses
  // (#176, kramttocs). The fields below write the SHARED cloud credentials, so
  // editing them from one domain's wizard changed every other domain's too —
  // exactly the "setting it in any of the places updates all of the others"
  // the issue describes. Every off-site destination already carries its own
  // credential-set selector, the replication path already resolves it per
  // destination (offsiteModeForTarget), and the primary destination IS such a
  // row whose creds_ref syncPrimaryOffsiteTarget deliberately preserves — the
  // only thing missing was a control that sets it, which is what this is.
  //
  // Loaded through the shared hook rather than a private copy, for the reason
  // OffsiteTargetsSection documents: the card that creates these sets lives on
  // this same page, so a fetched-once copy goes stale the moment one is added.
  const credSets = useCloudCredSets();
  const [primaryTarget, setPrimaryTarget] = useState<OffsiteTarget | null>(null);
  // The wizard edits either a domain's PRIMARY path or one of its off-site
  // destinations, and both can now name their own credential set (#182). The
  // two keep their state in different places, so the selector reads whichever
  // mode is active rather than being duplicated into two near-identical blocks.
  const credsRef = primary ? pCredsRef : (primaryTarget?.credsRef ?? "");
  const selectedCredSet = credSets.find((c) => c.id === credsRef);
  // Off-site: the destination row only exists once a repo has been saved.
  // Primary: the safety row is created on demand by the PUT, so the only thing
  // to wait for is the initial read that tells us the current value.
  const canPickCredSet = primary ? primaryLoaded : primaryTarget !== null;

  // Step 3 — connection test verdict. GlimStone follow-up pass (v8.0.0): the
  // ok/uninit/fail verdict below is now a toast, the exact same migration
  // Settings.tsx's TestConnectionButton already got for this exact
  // ok/uninit/fail shape (see runTest below) — so only busy/idle is left.
  const [testBusy, setTestBusy] = useState(false);

  // Repo URL save state. Full-page Speichern-Button sweep: the "Save
  // repository" button is gone (see patchRepo below) — only the setter
  // survives, threaded into the SHARED `save` prop (Settings.tsx's own
  // save()), which still requires a `(s: SaveState) => void` callback even
  // though save() itself never actually produces "saved"/"error" (see its
  // own comment in Settings.tsx).
  const [, setRepoState] = useState<SaveState>("idle");

  // Step 4 — immutable flag + tamper verdict. `immState` is SHARED by both
  // modes below: off-site mode routes through the same already-toast-
  // migrated shared `save` prop as repoState above (hence the SaveState type,
  // for the same signature-compatibility reason); primary mode's own
  // toggleImmutable further down now pushes its own toast too (GlimStone
  // follow-up pass, v8.0.0), so neither mode's "saved"/"error" render (dead
  // for one mode already, now dead for both) is needed — removed below.
  const [immState, setImmState] = useState<SaveState>("idle");
  // GlimStone standing rule (jdp, live review, emphatic, system-wide: "Wenn
  // etwas fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!") —
  // audit fix: toggleImmutable already rolls the optimistic flip back and
  // pushes a fail toast in BOTH modes below, but never bumped a shake nonce,
  // unlike every other copy of this exact optimistic-flip+revert shape in the
  // app (Settings.tsx's toggleDomainEnabled/autoSaveField, VMs.tsx's various
  // toggles) — despite toggleDomainEnabled explicitly documenting itself as
  // mirroring THIS function. A genuinely new value forces the Toggle below to
  // remount (passed as its `key`), so `.glim-shake` replays from its first
  // frame even for the same domain failing twice in a row — same mechanism as
  // ToggleRow's own shakeNonce (Settings.tsx).
  const [immShake, setImmShake] = useState(0);
  const [tamperState, setTamperState] = useState<"idle" | "busy" | "done">("idle");
  const [verdict, setVerdict] = useState<{ testable: boolean; protected: boolean; detail: string } | null>(null);
  // Audit fix: a FAILURE to even run the tamper test (runTamper's own two
  // catch-all branches below, the exact "toast+shake standard" target named
  // by IntegrityCard's own runTamperFor comment in Settings.tsx) pushed a
  // fail toast but never shook the triggering button — same nonce/key/
  // className shape as immShake above.
  const [tamperShake, setTamperShake] = useState(0);

  // #176 (kramttocs): this step used to be three radio buttons headed
  // "Retention strategy", which read as a setting and was not one. Only
  // `offsiteGrowthBudgetGB` was ever persisted; the choice itself lived in
  // local state, was reconstructed from that one number every time the
  // wizard mounted, and had NO influence on whether anything got pruned.
  // What actually decides that is service.go's copyToOffsiteTarget: an
  // immutable target is never pruned from here, otherwise the shared
  // off-site keep values apply. So the step now REPORTS that state instead
  // of offering a choice that decided nothing.
  const keepTotal =
    settings.offsiteRetentionKeepLast +
    settings.offsiteRetentionKeepDaily +
    settings.offsiteRetentionKeepWeekly +
    settings.offsiteRetentionKeepMonthly;
  const pruneMode: "farside" | "policy" | "none" = immutable
    ? "farside"
    : keepTotal > 0
      ? "policy"
      : "none";
  // Colour follows the same reading as the tamper verdict above: green is
  // the protected, fully-handled case. "farside" is green because
  // append-only is on AND the far side is told how to prune; "policy" is
  // plain text because it is ordinary working behaviour, not an
  // achievement; "none" warns because the repository grows without limit
  // and nothing in the app will ever say so on its own.
  const pruneColor =
    pruneMode === "farside" ? "text-statusOk" : pruneMode === "none" ? "text-statusWarn" : "text-carbon-text";
  const pruneText =
    pruneMode === "farside"
      ? t("offsite.prune.stateFarSide")
      : pruneMode === "policy"
        ? t("offsite.prune.statePolicy")
        : t("offsite.prune.stateNone");
  // `budgetState` is SHARED the same way `immState` is above: off-site mode's
  // "grow" branch routes through the shared `save` prop, primary mode's own
  // branch through savePrimarySafety — both toast their own outcome. Full-
  // page Speichern-Button sweep: both modes' Save buttons are gone (each
  // number field now debounce-auto-saves itself below), so only the setter
  // survives.
  const [, setBudgetState] = useState<SaveState>("idle");

  // Load this domain's primary off-site destination so its credential-set
  // selector below has something to bind to. Failures are silent on purpose:
  // the row only exists once an off-site repo has been saved, so "not there
  // yet" is an ordinary state during first-time setup, not an error worth
  // showing. The selector simply stays hidden until it exists.
  function refreshPrimaryTarget() {
    if (primary) return; // remote-primary mode has no off-site destination row
    listOffsiteTargets(offsiteDomain)
      .then((r) => {
        if (!r.ok) return;
        setPrimaryTarget((r.targets ?? []).find((x) => x.sortOrder === 0) ?? null);
      })
      .catch(() => {});
  }

  useEffect(() => {
    refreshPrimaryTarget();
    // Re-reads when the repo URL changes, because saving a repo for the first
    // time is what CREATES the row this selector edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [offsiteDomain, primary, repoURL]);

  // Persist the credential-set choice onto the primary destination. The row is
  // rewritten from Settings on every settings save, but syncPrimaryOffsiteTarget
  // explicitly carries creds_ref across that rewrite, so this survives.
  async function pickCredSet(ref: string) {
    const row = primaryTarget;
    if (!row) return;
    const next = { ...row, credsRef: ref };
    setPrimaryTarget(next); // optimistic: the select must not snap back while saving
    try {
      const r = await updateOffsiteTarget(row.id, next);
      if (!r.ok) {
        setPrimaryTarget(row);
        push(r.error ?? t("common.actionFailed"), "fail");
        return;
      }
      if (r.target) setPrimaryTarget(r.target);
    } catch {
      setPrimaryTarget(row);
      push(t("common.actionFailed"), "fail");
    }
  }

  // Full-page Speichern-Button sweep: patchRepo used to only update local
  // state, relying on the "Save repository" button (now gone) to persist it.
  // It now debounce-auto-saves through the SAME shared `save` prop the off-
  // site Card's own repo-URL field (Settings.tsx) already converted to.
  function patchRepo(v: string) {
    setSettings((prev) => (prev ? { ...prev, [repoKey]: v } : prev));
    debounced(String(repoKey), () => {
      // Re-read the destination AFTER the save completes, not only when repoURL
      // changes. Saving a repo for the first time is what CREATES the row the
      // credential selector binds to, and the keystroke that triggered this save
      // happened while it did not exist yet — so without this the selector stays
      // hidden until the wizard is next opened. Seen live on a first-time setup.
      void Promise.resolve(
        save({ [repoKey]: v } as Partial<Settings>, setRepoState, () => undefined)
      ).then(() => refreshPrimaryTarget());
    });
  }

  async function genSnippet() {
    setSnipBusy(true);
    try {
      // Never called for domain "config" — deploySnippet's backend route does
      // not accept it (see the Step 2 render gate below), so this cast is safe.
      const r = await deploySnippet(domain as OffsiteDomain);
      if (r.ok && r.snippet) {
        setSnippet(r.snippet);
      } else {
        push(r.error ?? t("offsite.wizard.snippetError"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.wizard.snippetError"), "fail");
    } finally {
      setSnipBusy(false);
    }
  }

  // GlimStone follow-up pass (v8.0.0): the ok/uninit/fail verdict below is now
  // a toast — the exact same ok/uninit/fail shape Settings.tsx's
  // TestConnectionButton already migrated, reused here verbatim (same i18n
  // keys, same severities).
  async function runTest() {
    setTestBusy(true);
    try {
      const r = primary ? await testPrimaryRemote(domain as PrimaryRemoteDomain) : await testOffsite(domain as OffsiteDomain);
      if (r.ok && r.reachable && r.initialized) {
        push(t("offsite.testOk"), "success");
      } else if (r.ok && r.reachable) {
        push(t("offsite.testUninitialized"), "warn");
      } else {
        push(r.error ?? t("offsite.testFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.testFailed"), "fail");
    } finally {
      setTestBusy(false);
    }
  }

  // GlimStone follow-up pass (v8.0.0): a FAILURE to even run the test is now a
  // one-shot toast (same shape as every other migrated test/save action). The
  // verdict itself, on success, stays exactly as it was — see the render
  // below (near verdictText) for why.
  async function runTamper() {
    setTamperState("busy");
    try {
      const r = primary
        ? await primaryRemoteTamperTest(domain as PrimaryRemoteDomain)
        : await tamperTest(domain as OffsiteDomain);
      if (r.ok) {
        setVerdict({ testable: !!r.testable, protected: !!r.protected, detail: r.detail ?? "" });
        setTamperState("done");
      } else {
        setTamperState("idle");
        push(r.error ?? t("offsite.tamperError"), "fail");
        setTamperShake((n) => n + 1);
      }
    } catch (e) {
      setTamperState("idle");
      push(e instanceof Error ? e.message : t("offsite.tamperError"), "fail");
      setTamperShake((n) => n + 1);
    }
  }

  // savePrimarySafety PUTs the FULL remote-primary safety config (the API has
  // no partial-patch form, unlike the off-site path's Settings-column save) —
  // used by both toggleImmutable (immutable + the CURRENT limits/budget) and
  // the bandwidth/budget form's own Save button (limits/budget + the CURRENT
  // immutable flag), so neither one clobbers the field the other owns.
  async function savePrimarySafety(patch: { immutable?: boolean; limitUpload?: number; limitDownload?: number; growthBudgetGb?: number; credsRef?: string }) {
    return setPrimaryRemote(domain, {
      immutable: patch.immutable ?? immutable,
      limitUpload: patch.limitUpload ?? pLimitUpload,
      limitDownload: patch.limitDownload ?? pLimitDownload,
      growthBudgetGb: patch.growthBudgetGb ?? pBudget,
      credsRef: patch.credsRef ?? pCredsRef,
    });
  }

  // #182: pick the primary path's credential set. Mirrors pickCredSet's
  // optimistic-then-revert shape, but goes through savePrimarySafety so the
  // limits and immutable flag ride along untouched.
  async function pickPrimaryCredSet(ref: string) {
    const prev = pCredsRef;
    setPCredsRef(ref); // optimistic: the select must not snap back while saving
    try {
      const r = await savePrimarySafety({ credsRef: ref });
      if (!r.ok) {
        setPCredsRef(prev);
        push(r.error ?? t("common.actionFailed"), "fail");
      }
    } catch {
      setPCredsRef(prev);
      push(t("common.actionFailed"), "fail");
    }
  }

  // persistPrimarySafety — the bandwidth-limits/growth-budget form's own
  // debounced auto-save (full-page Speichern-Button sweep; was a single
  // click handler inline on the now-removed Save button). Same
  // primaryLoaded guard the button's own `disabled` used to enforce: never
  // PUT before the real config was actually read, or a blank round-trip
  // would wipe the stored limits/budget.
  async function persistPrimarySafety(patch: { limitUpload?: number; limitDownload?: number; growthBudgetGb?: number }) {
    if (!primaryLoaded) return;
    setBudgetState("saving");
    try {
      const r = await savePrimarySafety(patch);
      if (r.ok) {
        push(t("settings.saved"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    } finally {
      setBudgetState("idle");
    }
  }

  // Toggling immutable ON persists the flag AND — only after a CONFIRMED save —
  // proves it with a tamper test (the verdict is shown verbatim). A failed save
  // rolls the optimistic flip back and surfaces the error, so a green "protected"
  // verdict can never appear while the server flag actually stayed OFF.
  //
  // GlimStone follow-up pass (v8.0.0): primary mode's own "saved"/"error" flash
  // (below) is now a toast — the off-site branch (via the shared `save` prop)
  // already got this for free from the prior pass; this brings primary mode to
  // the same behaviour rather than leaving the two modes inconsistent.
  //
  // Audit fix: BOTH branches below now also bump immShake on a failed save —
  // see that state's own doc comment above for why this was a gap despite
  // being the toast+revert pattern's own named origin.
  async function toggleImmutable(next: boolean) {
    if (primary) {
      if (!primaryLoaded) return; // never save before the config was actually read (mirrors cloudLoaded)
      setPrimaryConfig((prev) => (prev ? { ...prev, immutable: next } : prev));
      setImmState("saving");
      try {
        const r = await savePrimarySafety({ immutable: next });
        if (!r.ok) {
          setPrimaryConfig((prev) => (prev ? { ...prev, immutable: !next } : prev));
          push(r.error ?? t("settings.error"), "fail");
          setImmShake((n) => n + 1);
          return;
        }
        push(t("settings.saved"), "success");
      } catch (e) {
        setPrimaryConfig((prev) => (prev ? { ...prev, immutable: !next } : prev));
        push(e instanceof Error ? e.message : t("settings.error"), "fail");
        setImmShake((n) => n + 1);
        return;
      } finally {
        setImmState("idle");
      }
      if (next) void runTamper();
      else {
        setVerdict(null);
        setTamperState("idle");
      }
      return;
    }
    setSettings((prev) => (prev ? { ...prev, [immKey]: next } : prev));
    const ok = await save({ [immKey]: next } as Partial<Settings>, setImmState, () => undefined);
    if (!ok) {
      // Roll back the optimistic toggle; save() already pushed the reason.
      setSettings((prev) => (prev ? { ...prev, [immKey]: !next } : prev));
      setImmShake((n) => n + 1);
      return;
    }
    if (next) void runTamper();
    else {
      setVerdict(null);
      setTamperState("idle");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const stepTitle = "text-xs font-semibold text-carbon-textSub uppercase tracking-widest";

  // Backend caveats key off the ACTUAL repo URL (live), not the Step-1 radio — so
  // a saved/edited rclone: or s3: URL always shows its warning, and a REST/empty
  // URL never shows a spurious one.
  const urlBackend = inferBackend(repoURL);

  // Far-side prune cron hint (includes --keep-within 14d + a snapshot-count note).
  // REST has an actual "storage box" to SSH into with a local repo path; rclone/s3/
  // other don't — the prune must run from any SEPARATE machine with the same remote
  // configured, using the real repo URL, not a fabricated local path (#131).
  const cronHint =
    urlBackend === "rest"
      ? `# Run on the storage box itself — BombVault stays append-only:
0 4 * * 0 restic -r /path/on/storage-box/restic/bombvault-${domain}/${domain} forget \\
  --keep-within 14d --keep-weekly 8 --keep-monthly 12 --prune
# note: watch for a sudden snapshot-count drop (retention-policy timestamp attack)`
      : `# Run from a SEPARATE machine with this remote configured — BombVault itself
# never prunes an immutable off-site repo:
0 4 * * 0 restic -r ${repoURL || "<repo-url>"} forget \\
  --keep-within 14d --keep-weekly 8 --keep-monthly 12 --prune
# note: watch for a sudden snapshot-count drop (retention-policy timestamp attack)`;

  const verdictText = verdict
    ? !verdict.testable
      ? t("offsite.tamperUnverifiable")
      : verdict.protected
        ? t("offsite.tamperOk")
        : t("offsite.tamperFail")
    : "";
  // The ✓/✗ glyph is rendered as its own JSX node (not baked into the i18n
  // string) so RTL locales (ar/he) place it on the correct side via bidi.
  const verdictGlyph = verdict && verdict.testable ? (verdict.protected ? "✓" : "✗") : "";
  const verdictColor = verdict
    ? !verdict.testable
      ? "text-statusWarn"
      : verdict.protected
        ? "text-statusOk"
        : "text-statusFail"
    : "";

  return (
    <div className="mt-2 flex flex-col gap-4 rounded-card bg-carbon-surface2 p-4">
      {/* Step 1 — backend choice */}
      <div className="flex flex-col gap-2">
        <span className={stepTitle}>{t("offsite.wizard.step1")}</span>
        <div className="flex flex-col gap-1.5">
          {([
            ["rest", "offsite.wizard.backendRest"],
            ["rclone", "offsite.wizard.backendRclone"],
            ["s3", "offsite.wizard.backendS3"],
            ["path", "offsite.wizard.backendPath"],
          ] as const).map(([val, label]) => (
            <label key={val} className="flex items-center gap-2 text-sm text-carbon-text cursor-pointer">
              <input
                type="radio"
                name={`backend-${domain}`}
                checked={backend === val}
                onChange={() => setBackend(val)}
                style={{ accentColor: "var(--accent)" }}
              />
              {t(label)}
            </label>
          ))}
        </div>
        {/* A mounted share needs no server at all — but it does need the path
            RELATIVE to the Host Data mount, which is the one thing nothing in
            this flow used to say (issue #138). */}
        {backend === "path" && (
          <p className="text-xs text-carbon-textMuted leading-relaxed">
            {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
          </p>
        )}
      </div>

      {/* Step 2 — rest-server deploy snippet. Not offered for "config" — the
          backend's deploy-snippet route only covers containers/vms/flash/files
          (only reachable here in remote-primary mode; off-site mode never
          receives domain="config" to begin with). */}
      {backend === "rest" && domain !== "config" && (
        <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={stepTitle}>{t("offsite.wizard.step2")}</span>
          <p className="text-xs text-carbon-textMuted">{t("offsite.wizard.step2Hint")}</p>
          {!snippet && (
            <Button
              label={t("offsite.wizard.generate")}
              labelKey="offsite.wizard.generate"
              tone="accent"
              onClick={() => void genSnippet()}
              disabled={snipBusy}
              busy={snipBusy}
              title={snipBusy ? t("common.saving") : undefined}
              className="self-start"
            />
          )}
          {snippet && (
            <div className="flex flex-col gap-2">
              <div className="rounded-card bg-statusWarnBg px-3 py-2 text-xs text-statusWarn leading-relaxed">
                {t("offsite.wizard.passwordWarning")}
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textMuted">{t("offsite.wizard.password")}</span>
                <CopyBlock text={snippet.password} t={t} />
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textMuted">docker run</span>
                <CopyBlock text={snippet.dockerRun} t={t} />
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textMuted">docker-compose</span>
                <CopyBlock text={snippet.compose} t={t} />
              </div>
              <div className="rounded-card bg-carbon-surface px-3 py-2 text-xs text-carbon-textSub leading-relaxed">
                {t("offsite.wizard.tlsNote")}
              </div>
              {/* Task 5 (rule 13): was a plain underline-on-hover text button.
                  Task 7: tone was "info" (the old fifth hue) only because it
                  was the nearest tone available at the time — a plain action
                  badge, not activity or a state, same as Recovery.tsx's own
                  tone="neutral" reload badge and Settings.tsx's two doc-link
                  badges. */}
              <Badge
                as="button"
                onClick={() => void genSnippet()}
                tone="neutral"
                size="small"
                className="self-start"
              >
                {t("offsite.wizard.regenerate")}
              </Badge>
            </div>
          )}
        </div>
      )}

      {/* Step 3 — repo URL + schedule + credentials + connection test */}
      <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
        <span className={stepTitle}>{t("offsite.wizard.step3")}</span>
        {primary ? (
          // Remote-primary mode: the path is edited on the Storage tab's field
          // itself (switching it back to Local there is how you leave this
          // mode) — shown here read-only so Steps 1/4-6 below still reason
          // about the right URL.
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("offsite.wizard.repoUrl")}</span>
            {/* Task 6 (RTL sweep): a read-only repo URL is a technical value,
                pinned LTR exactly like OffsiteTargetsSection's own repo cell —
                otherwise a leading `/` (a weak bidi character) migrates to the
                trailing edge in ar/he. */}
            <p
              dir="ltr"
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 break-all text-start"
            >
              {repoURL || "—"}
            </p>
            <span className="text-xs text-carbon-textMuted">{t("settings.primaryRemote.hint")}</span>
            {primaryLoadErr && <span className="text-xs text-statusFail">{primaryLoadErr}</span>}
          </div>
        ) : (
          <>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("offsite.wizard.repoUrl")}</span>
              <input
                value={repoURL}
                spellCheck={false}
                onChange={(e) => patchRepo(e.target.value)}
                placeholder={t("offsite.wizard.repoUrlPlaceholder")}
                dir="ltr"
                className={`${inputCls} text-start`}
              />
              <span className="text-xs text-carbon-textMuted">
                {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
              </span>
            </label>
            {/* The off-site schedule is edited in Settings › Schedules now; the wizard
                saves only the repo URL so it can never clobber that cadence.
                  Full-page Speichern-Button sweep: the "Save repository"
                button that used to sit here is gone — patchRepo (above)
                debounce-auto-saves the field itself now. */}
          </>
        )}

        {/* Credentials — reuse the cloud-credential endpoints.
            #182 (manilx): the SELECTOR belongs to every remote backend, not just
            REST. A credential set carries S3 keys as well as REST ones, and
            offsiteModeForTarget resolves whichever set a destination names, so
            an S3 destination can have its own credentials just as much as a REST
            one. Gating the whole block on REST meant an S3 user was never
            offered that choice and could only ever see the shared set.
            #131 still holds for the FIELDS below: only REST needs a username and
            password here, since s3/rclone carry their auth in the shared cloud
            credentials, so for them this block shows the selector and says where
            the shared ones live. A local path needs no credentials at all. */}
        {urlBackend !== "none" && urlBackend !== "path" && (
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface p-3 mt-1">
            <span className="text-xs font-medium text-carbon-textSub">{t("offsite.wizard.credentials")}</span>
            {/* #176: which credentials this destination uses. Without this the
                fields below are the SHARED set, so filling them in from one
                domain's wizard silently rewrote every other domain's. Only
                shown once the destination row exists (i.e. a repo was saved). */}
            {/* No VISIBLE field label: the block heading right above already
                says "Credentials", and repeating it underneath was only the
                right shape while that heading named REST specifically. The name
                moves to aria-label so the control still announces itself. */}
            {canPickCredSet && (
              <label className="flex flex-col gap-1">
                <select
                  aria-label={t("offsite.targets.credsLabel")}
                  value={credsRef}
                  onChange={(e) => void (primary ? pickPrimaryCredSet(e.target.value) : pickCredSet(e.target.value))}
                  className={inputCls}
                >
                  <option value="">{t("offsite.targets.credsDefault")}</option>
                  {credSets.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {/* One place to CHOOSE credentials (this dropdown), one place to
                EDIT them (Settings › Shared cloud credentials, or the set's own
                row). The wizard used to also edit the shared username and
                password inline whenever the backend was REST, which meant the
                same two values had two editors and made "Shared" look like a
                property of this destination rather than one list everything
                falls back to. kramttocs asked for exactly this on #176: "my
                preference would be to ONLY have a dropdown here and never see
                the Username or Password". */}
            {selectedCredSet ? (
              <span className="text-xs text-carbon-textMuted">
                {t("offsite.wizard.credsInSet").replace("{name}", selectedCredSet.name)}
              </span>
            ) : (
              <span className="text-xs text-carbon-textMuted">{t("offsite.wizard.credsSharedElsewhere")}</span>
            )}
          </div>
        )}

        {/* Connection test. Audit fix: this used to be a plain, un-hued
            `bg-carbon-surface` <button> — unlike its sibling OUTSIDE the
            wizard (Settings.tsx's TestConnectionButton), which already reads
            this same enclosing Card's hueIndex through `Badge tone="active"`,
            this one never joined the colour engine at all, visibly
            de-colouring the action in rainbow mode the moment the wizard
            opened. Reuses the exact text-badge shape this same file's own
            "regenerate" Badge above already established (`size="small"`),
            just `tone="active"` + `hueIndex` instead of `tone="neutral"` —
            this IS a domain action (the same connection probe
            TestConnectionButton runs), not a neutral utility like
            copy/regenerate. */}
        <div className="flex items-center gap-3">
          <Badge
            as="button"
            tone="active"
            size="small"
            hueIndex={hueIndex}
            onClick={() => void runTest()}
            disabled={testBusy}
          >
            {testBusy ? t("offsite.testing") : t("offsite.test")}
          </Badge>
        </div>
      </div>

      {/* Step 4 — immutable (append-only) toggle + verbatim tamper verdict */}
      <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
        <span className={stepTitle}>{t("offsite.wizard.step4")}</span>
        <div className="flex items-start justify-between gap-4">
          <div className="flex flex-col gap-0.5">
            <span className="text-sm text-carbon-text">{t("offsite.immutable")}</span>
            <span className="text-xs text-carbon-textMuted">{t("offsite.immutableHint")}</span>
          </div>
          <Toggle
            key={immShake}
            hideLabel
            label={t("offsite.immutable")}
            checked={immutable}
            onChange={(next) => void toggleImmutable(next)}
            disabled={immState === "saving"}
            className={`mt-0.5${immShake ? " glim-shake" : ""}`}
          />
        </div>

        {/* Backend-specific caveats (Step 5) — driven by the live repo URL. */}
        {urlBackend === "rclone" && (
          <div className="rounded-card bg-statusWarnBg px-3 py-2 text-xs text-statusWarn leading-relaxed">
            {t("offsite.rcloneWarning")}
          </div>
        )}
        {urlBackend === "s3" && (
          <div className="rounded-card bg-carbon-surface px-3 py-2 text-xs text-carbon-textSub leading-relaxed">
            {t("offsite.s3Unverified")}
          </div>
        )}

        {/* Verbatim tamper verdict + a manual "test now" — the backend only ever
            verifies REST repos (RunTamperTest reports Testable=false otherwise),
            so a non-REST backend gets the SAME "not verifiable" wording up front
            instead of an active-looking button that leads nowhere (#131).
            GlimStone follow-up pass (v8.0.0) audit note: the verdict below is
            DELIBERATELY left as inline status, not migrated to a toast — it's
            the actual security-check RESULT (testable/protected/detail), no
            auto-dismiss even before this pass, meant to answer "is this repo
            actually tamper-proof" persistently — the same "what did the last
            check say" reasoning as IntegrityCard's results. Only a FAILURE to
            even run the test (couldn't reach the backend at all) moved to a
            toast — see runTamper's own comment.
            Audit fix: this button had the SAME missing-hueIndex gap as the
            Step 3 connection-test button above (same un-hued
            `bg-carbon-surface` shape, same sibling-Card-control comparison),
            plus its own separate gap — the "couldn't even run" toast never
            shook the button, unlike every other copy of the toast+shake
            standard in the app (see tamperShake's own doc comment, and
            IntegrityCard's runTamperFor in Settings.tsx, the exact site whose
            comment names this failure case as the standard's target). */}
        {urlBackend === "rest" ? (
          <div className="flex items-center gap-3 flex-wrap">
            <Badge
              key={tamperShake}
              as="button"
              tone="active"
              size="small"
              hueIndex={hueIndex}
              onClick={() => void runTamper()}
              disabled={tamperState === "busy" || !immutable}
              className={tamperShake ? "glim-shake" : undefined}
            >
              {tamperState === "busy" ? t("offsite.tamperTesting") : t("offsite.tamperTestNow")}
            </Badge>
            {tamperState === "done" && verdict && (
              <span className={`text-sm wrap-break-word ${verdictColor}`}>
                {verdictGlyph && <span aria-hidden="true">{verdictGlyph}&nbsp;</span>}
                {verdictText}
              </span>
            )}
          </div>
        ) : (
          <span className="text-xs text-carbon-textMuted">{t("offsite.tamperUnverifiable")}</span>
        )}
      </div>

      {/* Step 6 — remote-primary mode: bandwidth limits + growth-budget alarm
          (there is no separate off-site copy to prune independently, so the
          far-side-prune / maintenance-window strategies below don't apply);
          off-site mode: the original retention-strategy chooser, unchanged. */}
      {primary ? (
        <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={stepTitle}>{t("settings.offsiteLimits")}</span>
          <p className="text-xs text-carbon-textMuted leading-relaxed">{t("settings.limitHint")}</p>
          {/* Full-page Speichern-Button sweep: these three fields used to
              batch into one bottom Save button — each now debounce-auto-
              saves itself through persistPrimarySafety (below), guarded the
              same way the old button's `disabled={!primaryLoaded}` was: never
              PUT before the real config was actually read (a blank round-trip
              would wipe the stored limits/budget — see primaryLoaded's own
              declaration comment above). */}
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("settings.limitUpload")}</span>
              <input
                type="number"
                min={0}
                value={pLimitUpload}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setPLimitUpload(n);
                  debounced("pLimitUpload", () => void persistPrimarySafety({ limitUpload: n }));
                }}
                className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("settings.limitDownload")}</span>
              <input
                type="number"
                min={0}
                value={pLimitDownload}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setPLimitDownload(n);
                  debounced("pLimitDownload", () => void persistPrimarySafety({ limitDownload: n }));
                }}
                className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
              />
            </label>
          </div>
          <label className="flex flex-col gap-1 max-w-48">
            <span className="text-xs text-carbon-textSub">{t("offsite.retention.budget")}</span>
            <input
              type="number"
              min={0}
              value={pBudget}
              onChange={(e) => {
                const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                setPBudget(n);
                debounced("pBudget", () => void persistPrimarySafety({ growthBudgetGb: n }));
              }}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
            />
          </label>
          <p className="text-xs text-carbon-textMuted leading-relaxed">{t("settings.primaryRemote.budgetHint")}</p>
        </div>
      ) : (
        <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className="flex items-center gap-1">
            <span className={stepTitle}>{t("offsite.prune.title")}</span>
            <InfoBubble tip={t("offsite.prune.info")} />
          </span>

          {/* Shade, not a border, per the house rule: the state sits on a
              lighter surface inside the step rather than inside a box. */}
          <div className="flex flex-col gap-1 rounded-card bg-carbon-surface p-3">
            <span className={`text-sm wrap-break-word ${pruneColor}`}>{pruneText}</span>
            {pruneMode === "policy" && (
              <>
                <span className="text-xs text-carbon-textSub">
                  {t("offsite.prune.effective")
                    .replace("{last}", String(settings.offsiteRetentionKeepLast))
                    .replace("{daily}", String(settings.offsiteRetentionKeepDaily))
                    .replace("{weekly}", String(settings.offsiteRetentionKeepWeekly))
                    .replace("{monthly}", String(settings.offsiteRetentionKeepMonthly))}
                </span>
                <span className="text-xs text-carbon-textMuted">{t("offsite.prune.editedElsewhere")}</span>
              </>
            )}
          </div>

          {/* The cron snippet is only useful while BombVault is standing
              back, i.e. exactly when append-only is on. Showing it in the
              other two states invited someone to run a second pruner
              against a repository BombVault is already pruning. */}
          {pruneMode === "farside" && <CopyBlock text={cronHint} t={t} />}

          {/* The growth budget is NOT tied to a strategy choice any more.
              It was reachable only behind the old "grow" radio, and this
              wizard is the only editor for it in the whole app, so hiding
              it behind a choice made a real, persisted setting
              unreachable depending on an unrelated radio button.
              Full-page Speichern-Button sweep: no save button, it
              debounce-auto-saves through the same shared `save` prop every
              other off-site field on this page already converted to. */}
          <label className="flex flex-col gap-1 max-w-48">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("offsite.retention.budget")}
              <InfoBubble tip={t("offsite.prune.budgetInfo")} />
            </span>
            <input
              type="number"
              min={0}
              value={settings.offsiteGrowthBudgetGB}
              onChange={(e) => {
                const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                setSettings((prev) => (prev ? { ...prev, offsiteGrowthBudgetGB: n } : prev));
                debounced("offsiteGrowthBudgetGB", () =>
                  void save({ offsiteGrowthBudgetGB: n }, setBudgetState, () => undefined)
                );
              }}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
            />
          </label>
        </div>
      )}
    </div>
  );
}
