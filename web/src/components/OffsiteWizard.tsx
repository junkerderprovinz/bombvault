import { useEffect, useRef, useState } from "react";
import type { Settings, DeploySnippetData, PrimaryRemoteConfig, OffsiteDomain } from "../lib/api";
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
  getCloud,
} from "../lib/api";
import type { OffsiteTarget } from "../lib/api";
import { useCloudCredSets } from "../lib/useCloudCredSets";
import { restPathUserMismatch } from "../lib/restRepo";
import { SelectField } from "./SelectField";
import { useT } from "../lib/i18n";
import { directAsk, directUse } from "../lib/directRepo";
import { useNamedRepos } from "../lib/useNamedRepos";
import { useOffsiteTargets } from "../lib/useOffsiteTargets";
import { useConfirm } from "../lib/useConfirm";
import { InfoBubble } from "./InfoBubble";
import { NumberField } from "./NumberField";
import { Toggle } from "./Toggle";
import { Badge } from "./Badge";
import { withLtrFragments, REPO_LOCAL_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { useToast } from "../lib/toast";
import { Button } from "./Button";
import { OffsiteLocationInput } from "./placement/OffsiteLocationInput";

// Guided off-site setup for one domain. It has no persistence of its own: the
// repo URL, immutable flag and growth budget go through the same `settings`,
// `setSettings` and `save` as the Settings page. Credentials are only chosen
// here and edited under Settings › Shared cloud credentials. Around those
// inputs the wizard adds the backend choice, a rest-server deploy snippet, a
// connection test, an append-only tamper verdict and the prune state.

type Domain = OffsiteDomain;
type T = ReturnType<typeof useT>["t"];
type SaveState = "idle" | "saving" | "saved" | "error";

// The same Settings fields the off-site card writes.
const REPO_KEY = {
  containers: "containersOffsite",
  vms: "vmsOffsite",
  flash: "flashOffsite",
  files: "filesOffsite",
  config: "configOffsite",
} as const;
const IMM_KEY = {
  containers: "containersOffsiteImmutable",
  vms: "vmsOffsiteImmutable",
  flash: "flashOffsiteImmutable",
  files: "filesOffsiteImmutable",
  config: "configOffsiteImmutable",
} as const;
// Each domain's backup path. Remote-primary mode reads it for display and
// backend inference; the Storage tab edits it.
const PATH_KEY: Record<Domain, keyof Settings> = {
  containers: "containersPath",
  vms: "vmsPath",
  flash: "flashPath",
  config: "configPath",
  files: "filesPath",
};

// "none" is an empty URL. "path" is a folder under the Host Data mount, such as
// a mounted NAS share. "other" is any other scheme (sftp, b2, gs, azure), which
// must not get the REST deploy snippet. "path" and "other" get the same
// caveats and differ only in what Step 1 explains.
type Backend = "rest" | "rclone" | "s3" | "path" | "other" | "none";

function inferBackend(url: string): Backend {
  const u = url.trim();
  if (u === "") return "none";
  if (u.startsWith("rclone:")) return "rclone";
  if (u.startsWith("s3:") || u.startsWith("s3://")) return "s3";
  if (u.startsWith("rest:") || u.startsWith("http://") || u.startsWith("https://")) return "rest";
  // No scheme: a local or mounted path, which restic and resolveRepo accept
  // relative to the Host Data mount.
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(u)) return "path";
  return "other";
}

// CopyBlock is a monospace <pre> with a copy button. On a non-HTTPS origin the
// clipboard is unavailable, and the text can still be selected by hand.
function CopyBlock({ text, t }: { text: string; t: T }) {
  const { push } = useToast();
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      push(t("common.copied"), "success");
    } catch {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <Button
        label={t("common.copy")}
        labelKey="common.copy"
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
   * false: the off-site destination wizard. Repo, immutable flag and growth
   * budget bind to the domain's off-site Settings columns.
   *
   * true: safety settings for a remote primary, where the domain's own backup
   * path (edited on the Storage tab) is a restic remote. The repo URL is read
   * only, the prune step becomes a bandwidth and budget form because there is
   * no separate off-site copy to prune, and the values come from the
   * primary-remote API instead of Settings.
   */
  primary?: boolean;
  /** The enclosing Card's hue, so the test buttons match the Card's other
   *  controls. Callers without a single per-domain hue, such as
   *  PathModeSwitch's remote-mode dialog, leave it out. */
  hueIndex?: number;
}) {
  const repoKey = REPO_KEY[domain];
  const immKey = IMM_KEY[domain];

  // Remote-primary mode loads its saved safety settings once. primaryLoaded
  // gates every save, so a config that was never read cannot be written back
  // blank over the stored limits and budget.
  const [primaryConfig, setPrimaryConfig] = useState<PrimaryRemoteConfig | null>(null);
  const [primaryLoaded, setPrimaryLoaded] = useState(false);
  const [primaryLoadErr, setPrimaryLoadErr] = useState<string | null>(null);
  const [pLimitUpload, setPLimitUpload] = useState(0);
  const [pLimitDownload, setPLimitDownload] = useState(0);
  const [pBudget, setPBudget] = useState(0);
  // The primary path's credential set. savePrimarySafety writes the full
  // config, so every save carries it along.
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
    // domain and primary are fixed for a mounted dialog and t for a language,
    // so this runs once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [primary, domain]);

  // The URL every step works from: the live backup path in primary mode, the
  // saved off-site repo otherwise.
  const livePath = String(settings[PATH_KEY[domain]] ?? "");
  const repoURL = primary ? livePath : settings[repoKey];
  const immutable = primary ? (primaryConfig?.immutable ?? false) : settings[immKey];
  const [backend, setBackend] = useState<Backend>(() => inferBackend(repoURL));

  const { push } = useToast();

  const fieldTarget = useOffsiteTargets(domain).find((x) => x.sortOrder === 0);
  const namedRepos = useNamedRepos();
  const { confirm, confirmDialog } = useConfirm();
  const { lang } = useT();

  // The number fields save themselves after a pause, as elsewhere in Settings;
  // none of them is a draft that could be discarded, and the Settings page's
  // own debouncedSave is not reachable from here. The repo URL does not: a new
  // location starts uploads, so it waits for Enter, Save or leaving it.
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // Step 2: the rest-server deploy snippet, generated on demand and never
  // stored.
  const [snippet, setSnippet] = useState<DeploySnippetData | null>(null);
  const [snipBusy, setSnipBusy] = useState(false);

  // Step 3: the credential set of the domain's primary off-site destination.
  // That row carries creds_ref, which syncPrimaryOffsiteTarget preserves and
  // offsiteModeForTarget resolves per destination. The shared hook keeps the
  // list current when CloudCredSetsCard on the same page adds a set.
  const credSets = useCloudCredSets();
  const [primaryTarget, setPrimaryTarget] = useState<OffsiteTarget | null>(null);
  // Primary and off-site mode keep the choice in different places; the one
  // selector reads whichever mode is active.
  const credsRef = primary ? pCredsRef : (primaryTarget?.credsRef ?? "");
  const selectedCredSet = credSets.find((c) => c.id === credsRef);
  // Off-site: the destination row only exists once a repo has been saved.
  // Primary: the safety row is created on demand by the PUT, so the only thing
  // to wait for is the initial read that tells us the current value.
  const canPickCredSet = primary ? primaryLoaded : primaryTarget !== null;

  // The user these credentials sign in with: the named set's if one is chosen,
  // the shared one otherwise.
  const [sharedRestUser, setSharedRestUser] = useState("");
  useEffect(() => {
    let alive = true;
    void getCloud()
      .then((r) => { if (alive) setSharedRestUser(r.restUser ?? ""); })
      .catch(() => { /* a failed read only means no hint */ });
    return () => { alive = false; };
  }, []);
  // Shown while the URL is typed, before a connection test answers 401.
  const userMismatch = restPathUserMismatch(repoURL, selectedCredSet ? selectedCredSet.restUser : sharedRestUser);

  const [testBusy, setTestBusy] = useState(false);

  // save() takes a SaveState callback; the repo field has no use for the state.
  const [, setRepoState] = useState<SaveState>("idle");

  // Step 4: the immutable flag and the tamper verdict.
  const [immState, setImmState] = useState<SaveState>("idle");
  // A new key remounts the Toggle, so a failed save replays the shake even
  // twice in a row.
  const [immShake, setImmShake] = useState(0);
  const [tamperState, setTamperState] = useState<"idle" | "busy" | "done">("idle");
  const [verdict, setVerdict] = useState<{ testable: boolean; protected: boolean; detail: string } | null>(null);
  // Bumped when the tamper test cannot run at all.
  const [tamperShake, setTamperShake] = useState(0);

  // The prune step reports what happens rather than offering a choice:
  // service.go's copyToOffsiteTarget never prunes an immutable target from
  // here and applies the shared off-site keep values otherwise.
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
  // Green when append-only is on and the far side prunes, plain for the
  // ordinary keep policy, and a warning when nothing prunes, since the
  // repository then grows without limit and nothing else says so.
  const pruneColor =
    pruneMode === "farside" ? "text-statusOk" : pruneMode === "none" ? "text-statusWarn" : "text-carbon-text";
  const pruneText =
    pruneMode === "farside"
      ? t("offsite.prune.stateFarSide")
      : pruneMode === "policy"
        ? t("offsite.prune.statePolicy")
        : t("offsite.prune.stateNone");
  // Both modes report the budget save through toasts; only the setter is used.
  const [, setBudgetState] = useState<SaveState>("idle");

  // Loads the primary off-site destination for the credential selector. The
  // row exists only once a repo has been saved, so during setup a failure is
  // ordinary and stays silent; the selector stays hidden until then.
  function refreshPrimaryTarget() {
    if (primary) return; // remote-primary mode has no off-site destination row
    listOffsiteTargets(domain)
      .then((r) => {
        if (!r.ok) return;
        setPrimaryTarget((r.targets ?? []).find((x) => x.sortOrder === 0) ?? null);
      })
      .catch(() => {});
  }

  useEffect(() => {
    refreshPrimaryTarget();
    // Re-read when the repo URL changes: saving the first repo creates the row
    // this selector edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [domain, primary, repoURL]);

  // Persist the credential-set choice onto the primary destination. The row is
  // rewritten from Settings on every settings save, and syncPrimaryOffsiteTarget
  // carries creds_ref across that rewrite.
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

  function saveRepo(v: string): Promise<boolean> {
    return save({ [repoKey]: v } as Partial<Settings>, setRepoState, () => undefined);
  }

  async function genSnippet() {
    setSnipBusy(true);
    try {
      // The Step 2 gate keeps "config" out; the deploy-snippet route does not
      // accept it.
      const r = await deploySnippet(domain);
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

  async function runTest() {
    setTestBusy(true);
    try {
      const r = primary ? await testPrimaryRemote(domain) : await testOffsite(domain);
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

  // A failure to run the test is a toast; a verdict stays inline.
  async function runTamper() {
    setTamperState("busy");
    try {
      const r = primary
        ? await primaryRemoteTamperTest(domain)
        : await tamperTest(domain);
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

  // The primary-remote API only takes the full safety config, so each caller
  // passes its own fields and the others keep their current values.
  async function savePrimarySafety(patch: { immutable?: boolean; limitUpload?: number; limitDownload?: number; growthBudgetGb?: number; credsRef?: string }) {
    return setPrimaryRemote(domain, {
      immutable: patch.immutable ?? immutable,
      limitUpload: patch.limitUpload ?? pLimitUpload,
      limitDownload: patch.limitDownload ?? pLimitDownload,
      growthBudgetGb: patch.growthBudgetGb ?? pBudget,
      credsRef: patch.credsRef ?? pCredsRef,
    });
  }

  // Like pickCredSet, but through savePrimarySafety so the limits and the
  // immutable flag stay as they are.
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

  // Saves the bandwidth and budget fields. It waits for primaryLoaded, so a
  // config that was never read is not written back blank.
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

  // Turning immutable on saves the flag and, only after a confirmed save,
  // proves it with a tamper test. A failed save rolls the toggle back, so a
  // green verdict can never show while the server flag is still off.
  async function toggleImmutable(next: boolean) {
    if (primary) {
      if (!primaryLoaded) return;
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
    const use = !next && fieldTarget ? directUse(fieldTarget, namedRepos) : undefined;
    if (use && !(await confirm(directAsk(t, lang, "offsite.directAppendOnlyAsk", [use])))) return;
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
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus-well";
  const stepTitle = "text-xs font-semibold text-carbon-textSub uppercase tracking-widest";

  // Caveats follow the repo URL, not the Step 1 radio, so a saved rclone: or s3:
  // URL always shows its warning.
  const urlBackend = inferBackend(repoURL);

  // The far-side prune job. A REST server has a storage box with a local repo
  // path to run it on; for every other backend it runs from a separate machine
  // against the real repo URL.
  const cronHint =
    urlBackend === "rest"
      ? `# Run on the storage box itself, so BombVault stays append-only:
0 4 * * 0 restic -r /path/on/storage-box/restic/bombvault-${domain}/${domain} forget \\
  --keep-within 14d --keep-weekly 8 --keep-monthly 12 --prune
# note: watch for a sudden snapshot-count drop (retention-policy timestamp attack)`
      : `# Run from a separate machine with this remote configured. BombVault itself
# never prunes an immutable off-site repo:
0 4 * * 0 restic -r ${repoURL || "<repo-url>"} forget \\
  --keep-within 14d --keep-weekly 8 --keep-monthly 12 --prune
# note: watch for a sudden snapshot-count drop (retention-policy timestamp attack)`;

  // The verdict names the outcome and the server's detail gives the reason.
  // The detail quotes an HTTP status and the far side's behaviour, so it stays
  // untranslated, like restic and rclone messages in the activity log.
  const verdictText = verdict
    ? !verdict.testable
      ? t("offsite.tamperUnverifiable")
      : verdict.protected
        ? t("offsite.tamperOk")
        : verdict.detail
          ? `${t("offsite.tamperFail")} — ${verdict.detail}`
          : t("offsite.tamperFail")
    : "";
  // The glyph is its own node rather than part of the translation, so bidi
  // places it on the correct side in ar and he.
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
      {confirmDialog}
      {/* Step 1: backend choice */}
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
        {/* A mounted share needs no server, but its path is relative to the
            Host Data mount. */}
        {backend === "path" && (
          <p className="text-xs text-carbon-textMuted leading-relaxed">
            {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
          </p>
        )}
      </div>

      {/* Step 2: rest-server deploy snippet. The deploy-snippet route does not
          cover "config". */}
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
              {/* The recipes below contain an `echo '<user>:<hash>' >>
                  .htpasswd` line whose hash is this password. The bubble says
                  it is one secret in two forms, not two secrets. */}
              <div className="flex flex-col gap-1">
                <span className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
                  {t("offsite.wizard.password")}
                  <InfoBubble tip={t("offsite.wizard.passwordInfo")} />
                </span>
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
              {/* A container started with docker run has no Unraid template
                  behind it, so it cannot be edited in Unraid's Docker UI. The
                  template can. */}
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textMuted">{t("offsite.wizard.unraidTemplate")}</span>
                <CopyBlock text={snippet.unraid} t={t} />
              </div>
              <div className="rounded-card bg-carbon-surface px-3 py-2 text-xs text-carbon-textSub leading-relaxed">
                {t("offsite.wizard.tlsNote")}
              </div>
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

      {/* Step 3: repo URL, credentials and connection test */}
      <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
        <span className={stepTitle}>{t("offsite.wizard.step3")}</span>
        {primary ? (
          // Remote-primary mode: the path is edited on the Storage tab, where
          // switching it back to local also leaves this mode. Shown read-only
          // so the other steps work from the right URL.
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("offsite.wizard.repoUrl")}</span>
            {/* Pinned LTR, or a leading `/` moves to the trailing edge in ar
                and he. */}
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
            {/* With --private-repos, which every recipe here turns on, the
                first path segment has to be the htpasswd user; otherwise the
                server answers a bare 401. The bubble states the rule. */}
            <label className="flex flex-col gap-1">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("offsite.wizard.repoUrl")}
                <InfoBubble tip={t("offsite.wizard.repoUrlInfo")} />
              </span>
              <OffsiteLocationInput
                domain={domain}
                value={repoURL}
                targetId={primaryTarget?.id}
                targetName={primaryTarget?.name}
                placeholder={t("offsite.wizard.repoUrlPlaceholder")}
                className={`${inputCls} text-start`}
                onSave={saveRepo}
              />
              <span className="text-xs text-carbon-textMuted">
                {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
              </span>
              {/* A user of "bombvault_containers" against a URL starting
                  "bombvault-containers" ends in a 401. Only a warning: a server
                  without --private-repos accepts it, and the field keeps
                  saving. */}
              {userMismatch && (
                <span className="text-xs text-statusWarn">
                  {t("offsite.wizard.repoUserMismatch")
                    .replace("{segment}", userMismatch.segment)
                    .replace("{user}", userMismatch.user)}
                </span>
              )}
            </label>
            {/* The off-site schedule belongs to Settings › Schedules; the
                wizard saves only the repo URL so it can never clobber that
                cadence. */}
          </>
        )}

        {/* Credentials, for every remote backend: a set carries S3 keys as
            well as REST ones, and offsiteModeForTarget resolves whichever set a
            destination names. A local path needs none. */}
        {urlBackend !== "none" && urlBackend !== "path" && (
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface p-3 mt-1">
            <span className="text-xs font-medium text-carbon-textSub">{t("offsite.wizard.credentials")}</span>
            {/* Shown once the destination row exists, that is once a repo was
                saved. The heading above already says Credentials, so the
                select's name is only its accessible label. */}
            {canPickCredSet && (
              <label className="flex flex-col gap-1">
                <SelectField
                  label={t("offsite.targets.credsLabel")}
                  value={credsRef}
                  onChange={(v) => void (primary ? pickPrimaryCredSet(v) : pickCredSet(v))}
                  options={[
                    { value: "", label: t("offsite.targets.credsDefault") },
                    ...credSets.map((c) => ({ value: c.id, label: c.name })),
                  ]}
                  className={inputCls}
                />
              </label>
            )}
            {/* The dropdown chooses credentials; Settings › Shared cloud
                credentials and each set's own row edit them. */}
            {selectedCredSet ? (
              <span className="text-xs text-carbon-textMuted">
                {t("offsite.wizard.credsInSet").replace("{name}", selectedCredSet.name)}
              </span>
            ) : (
              <span className="text-xs text-carbon-textMuted">{t("offsite.wizard.credsSharedElsewhere")}</span>
            )}
          </div>
        )}

        {/* Hued like TestConnectionButton outside the wizard, which runs the
            same probe. */}
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

      {/* Step 4: append-only toggle and tamper verdict */}
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

        {/* Step 5: caveats for the repo URL's backend */}
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

        {/* Only REST repos can be tamper-tested (RunTamperTest reports
            Testable=false otherwise), so other backends get a sentence instead
            of a button. The button is absent, not disabled, while append-only
            is off. The verdict stays inline rather than in a toast because it
            is the result of a security check people come back to read. */}
        {urlBackend === "rest" ? (
          immutable ? (
            <div className="flex items-center gap-3 flex-wrap">
              <Badge
                key={tamperShake}
                as="button"
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => void runTamper()}
                disabled={tamperState === "busy"}
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
          ) : null
        ) : (
          <span className="text-xs text-carbon-textMuted">{t("offsite.tamperUnverifiable")}</span>
        )}
      </div>

      {/* Step 6. Remote-primary mode: bandwidth limits and the growth-budget
          alarm, since there is no separate off-site copy to prune. Off-site
          mode: the prune state. */}
      {primary ? (
        <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={stepTitle}>{t("settings.offsiteLimits")}</span>
          <p className="text-xs text-carbon-textMuted leading-relaxed">{t("settings.limitHint")}</p>
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("settings.limitUpload")}</span>
              <NumberField
                min={0}
                value={pLimitUpload}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setPLimitUpload(n);
                  debounced("pLimitUpload", () => void persistPrimarySafety({ limitUpload: n }));
                }}
                className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus-well"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("settings.limitDownload")}</span>
              <NumberField
                min={0}
                value={pLimitDownload}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setPLimitDownload(n);
                  debounced("pLimitDownload", () => void persistPrimarySafety({ limitDownload: n }));
                }}
                className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus-well"
              />
            </label>
          </div>
          <label className="flex flex-col gap-1 max-w-48">
            <span className="text-xs text-carbon-textSub">{t("offsite.retention.budget")}</span>
            <NumberField
              min={0}
              value={pBudget}
              onChange={(e) => {
                const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                setPBudget(n);
                debounced("pBudget", () => void persistPrimarySafety({ growthBudgetGb: n }));
              }}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus-well"
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

          {/* Only while append-only is on and BombVault leaves pruning to the
              far side; otherwise it would invite a second pruner. */}
          {pruneMode === "farside" && <CopyBlock text={cronHint} t={t} />}

          {/* Always shown: the wizard is the only editor for the growth
              budget. */}
          <label className="flex flex-col gap-1 max-w-48">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("offsite.retention.budget")}
              <InfoBubble tip={t("offsite.prune.budgetInfo")} />
            </span>
            <NumberField
              min={0}
              value={settings.offsiteGrowthBudgetGB}
              onChange={(e) => {
                const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                setSettings((prev) => (prev ? { ...prev, offsiteGrowthBudgetGB: n } : prev));
                debounced("offsiteGrowthBudgetGB", () =>
                  void save({ offsiteGrowthBudgetGB: n }, setBudgetState, () => undefined)
                );
              }}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus-well"
            />
          </label>
        </div>
      )}
    </div>
  );
}
