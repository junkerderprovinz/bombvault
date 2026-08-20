import { useEffect, useState } from "react";
import type { Settings, DeploySnippetData, PrimaryRemoteConfig, PrimaryRemoteDomain } from "../lib/api";
import {
  deploySnippet,
  tamperTest,
  testOffsite,
  getCloud,
  setCloud,
  getPrimaryRemote,
  setPrimaryRemote,
  testPrimaryRemote,
  primaryRemoteTamperTest,
} from "../lib/api";
import { useT } from "../lib/i18n";
import { RevealInput } from "./RevealInput";
import { Toggle } from "./Toggle";
import { useReveal } from "../lib/useReveal";
import { Badge } from "./Badge";
import { withLtrFragments, REPO_LOCAL_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";

// ---------------------------------------------------------------------------
// OffsiteWizard — guided per-domain off-site setup.
//
// It does NOT own any new persistence: the repo URL/schedule + immutable flag +
// growth budget flow through the SAME `settings`/`setSettings`/`save` the Settings
// page already uses, and the REST credentials flow through the SAME getCloud/
// setCloud cloud-credential endpoints the Cloud card uses. The wizard only wraps
// those existing inputs in a step-by-step flow and adds the guided extras:
// backend choice, a rest-server deploy snippet, a connection test, an
// append-only tamper verdict, and a retention-strategy chooser.
// ---------------------------------------------------------------------------

// "config" is a valid domain for REMOTE-PRIMARY mode (primary=true) below —
// off-site mode (primary=false, the original behaviour) never receives it:
// Settings.tsx's off-site tab only ever lists containers/vms/flash/files.
type OffsiteDomain = "containers" | "vms" | "flash" | "files";
type Domain = OffsiteDomain | "config";
type T = ReturnType<typeof useT>["t"];
type SaveState = "idle" | "saving" | "saved" | "error";

// Per-domain Settings keys — off-site MODE binds to the exact same fields the
// off-site card and immutable flags already persist (no parallel state).
const REPO_KEY = {
  containers: "containersOffsite",
  vms: "vmsOffsite",
  flash: "flashOffsite",
  files: "filesOffsite",
} as const;
// The off-site schedule is owned by Settings › Schedules — the wizard no longer
// edits it, so there is no SCHED_KEY map here.
const IMM_KEY = {
  containers: "containersOffsiteImmutable",
  vms: "vmsOffsiteImmutable",
  flash: "flashOffsiteImmutable",
  files: "filesOffsiteImmutable",
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
// button that flips to "copied" for a moment. Clipboard may be unavailable on a
// non-HTTPS origin — the text stays selectable in that case.
function CopyBlock({ text, t }: { text: string; t: T }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard unavailable (non-HTTPS) — the text is selectable in the box */
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <button
        type="button"
        onClick={() => void copy()}
        className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover"
      >
        {copied ? t("vm.ssh.copied") : t("vm.ssh.copy")}
      </button>
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
}) {
  // Off-site mode never receives "config" (see the Domain/OffsiteDomain
  // comment above) — this narrows once so REPO_KEY/IMM_KEY lookups below
  // don't need a repeated undefined-guard. Never read in primary mode.
  const offsiteDomain = domain as OffsiteDomain;
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

  useEffect(() => {
    if (!primary) return;
    let active = true;
    getPrimaryRemote(domain as PrimaryRemoteDomain)
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

  // Step 2 — rest-server deploy snippet (generated on demand, never persisted).
  const [snippet, setSnippet] = useState<DeploySnippetData | null>(null);
  const [snipState, setSnipState] = useState<"idle" | "busy" | "error">("idle");
  const [snipErr, setSnipErr] = useState<string | null>(null);

  // Step 3 — REST credentials (reuses the cloud-credential endpoints). S3 fields
  // are loaded + preserved on save so this flow never clobbers them.
  const [cloud, setCloudState] = useState({ s3KeyId: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
  const [restPwSet, setRestPwSet] = useState(false);
  const revealRestPassword = useReveal();
  const [credState, setCredState] = useState<SaveState>("idle");
  const [credErr, setCredErr] = useState<string | null>(null);
  // cloudLoaded gates the "Save credentials" button: we must never POST a cloud
  // object that wasn't loaded from the server, or a blank round-trip would WIPE
  // the stored S3/REST non-secret fields (or clear CloudConf entirely).
  const [cloudLoaded, setCloudLoaded] = useState(false);
  const [cloudLoadErr, setCloudLoadErr] = useState<string | null>(null);

  // Step 3 — connection test verdict.
  const [testState, setTestState] = useState<"idle" | "busy" | "ok" | "uninit" | "fail">("idle");
  const [testErr, setTestErr] = useState<string | null>(null);

  // Repo URL/schedule save state.
  const [repoState, setRepoState] = useState<SaveState>("idle");
  const [repoErr, setRepoErr] = useState<string | null>(null);

  // Step 4 — immutable flag + tamper verdict.
  const [immState, setImmState] = useState<SaveState>("idle");
  const [immErr, setImmErr] = useState<string | null>(null);
  const [tamperState, setTamperState] = useState<"idle" | "busy" | "done" | "error">("idle");
  const [verdict, setVerdict] = useState<{ testable: boolean; protected: boolean; detail: string } | null>(null);
  const [tamperErr, setTamperErr] = useState<string | null>(null);

  // Step 6 — retention strategy (UI-only; only the budget number persists).
  const [retention, setRetention] = useState<"farside" | "window" | "grow">(
    settings.offsiteGrowthBudgetGB > 0 ? "grow" : "farside"
  );
  const [budgetState, setBudgetState] = useState<SaveState>("idle");
  const [budgetErr, setBudgetErr] = useState<string | null>(null);

  // Load the stored cloud creds once (mirrors the Cloud card) so a save can keep
  // the S3 fields + treat a blank REST password as "keep the stored one".
  useEffect(() => {
    let active = true;
    getCloud()
      .then((r) => {
        if (!active) return;
        if (!r.ok) {
          setCloudLoadErr(t("offsite.wizard.credLoadError"));
          return;
        }
        setCloudState((p) => ({
          ...p,
          s3KeyId: r.s3KeyId ?? "",
          s3Region: r.s3Region ?? "",
          restUser: r.restUser ?? "",
          s3StorageClass: r.s3StorageClass ?? "",
        }));
        setRestPwSet(!!r.restPasswordSet);
        // Only now is a save safe: the object about to be POSTed reflects the
        // server's stored non-secret fields.
        setCloudLoaded(true);
      })
      .catch(() => {
        if (active) setCloudLoadErr(t("offsite.wizard.credLoadError"));
      });
    return () => {
      active = false;
    };
    // t is stable for a given language; the load runs once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function patchRepo(v: string) {
    setSettings((prev) => (prev ? { ...prev, [repoKey]: v } : prev));
  }

  async function genSnippet() {
    setSnipState("busy");
    setSnipErr(null);
    try {
      // Never called for domain "config" — deploySnippet's backend route does
      // not accept it (see the Step 2 render gate below), so this cast is safe.
      const r = await deploySnippet(domain as OffsiteDomain);
      if (r.ok && r.snippet) {
        setSnippet(r.snippet);
        setSnipState("idle");
      } else {
        setSnipState("error");
        setSnipErr(r.error ?? t("offsite.wizard.snippetError"));
      }
    } catch (e) {
      setSnipState("error");
      setSnipErr(e instanceof Error ? e.message : t("offsite.wizard.snippetError"));
    }
  }

  async function saveCreds() {
    // Never POST creds that were not loaded from the server (a blank round-trip
    // would wipe the stored non-secret fields). The button is disabled in this
    // state too; this is the defensive backstop.
    if (!cloudLoaded) return;
    setCredState("saving");
    setCredErr(null);
    try {
      // Blank secrets = keep the stored value; S3 fields are round-tripped so the
      // wizard never wipes an existing S3 credential.
      const r = await setCloud({ s3KeyId: cloud.s3KeyId, s3Secret: "", s3Region: cloud.s3Region, restUser: cloud.restUser, restPassword: cloud.restPassword, s3StorageClass: cloud.s3StorageClass });
      if (r.ok) {
        setCredState("saved");
        setCloudState((p) => ({ ...p, restPassword: "" }));
        setRestPwSet(restPwSet || cloud.restPassword !== "");
        setTimeout(() => setCredState("idle"), 3000);
      } else {
        setCredState("error");
        setCredErr(r.error ?? t("settings.error"));
      }
    } catch (e) {
      setCredState("error");
      setCredErr(e instanceof Error ? e.message : t("settings.error"));
    }
  }

  async function runTest() {
    setTestState("busy");
    setTestErr(null);
    try {
      const r = primary ? await testPrimaryRemote(domain as PrimaryRemoteDomain) : await testOffsite(domain as OffsiteDomain);
      if (r.ok && r.reachable && r.initialized) setTestState("ok");
      else if (r.ok && r.reachable) setTestState("uninit");
      else {
        setTestState("fail");
        setTestErr(r.error ?? null);
      }
    } catch (e) {
      setTestState("fail");
      setTestErr(e instanceof Error ? e.message : null);
    }
  }

  async function runTamper() {
    setTamperState("busy");
    setTamperErr(null);
    try {
      const r = primary
        ? await primaryRemoteTamperTest(domain as PrimaryRemoteDomain)
        : await tamperTest(domain as OffsiteDomain);
      if (r.ok) {
        setVerdict({ testable: !!r.testable, protected: !!r.protected, detail: r.detail ?? "" });
        setTamperState("done");
      } else {
        setTamperState("error");
        setTamperErr(r.error ?? t("offsite.tamperError"));
      }
    } catch (e) {
      setTamperState("error");
      setTamperErr(e instanceof Error ? e.message : t("offsite.tamperError"));
    }
  }

  // savePrimarySafety PUTs the FULL remote-primary safety config (the API has
  // no partial-patch form, unlike the off-site path's Settings-column save) —
  // used by both toggleImmutable (immutable + the CURRENT limits/budget) and
  // the bandwidth/budget form's own Save button (limits/budget + the CURRENT
  // immutable flag), so neither one clobbers the field the other owns.
  async function savePrimarySafety(patch: { immutable?: boolean; limitUpload?: number; limitDownload?: number; growthBudgetGb?: number }) {
    return setPrimaryRemote(domain as PrimaryRemoteDomain, {
      immutable: patch.immutable ?? immutable,
      limitUpload: patch.limitUpload ?? pLimitUpload,
      limitDownload: patch.limitDownload ?? pLimitDownload,
      growthBudgetGb: patch.growthBudgetGb ?? pBudget,
    });
  }

  // Toggling immutable ON persists the flag AND — only after a CONFIRMED save —
  // proves it with a tamper test (the verdict is shown verbatim). A failed save
  // rolls the optimistic flip back and surfaces the error, so a green "protected"
  // verdict can never appear while the server flag actually stayed OFF.
  async function toggleImmutable(next: boolean) {
    if (primary) {
      if (!primaryLoaded) return; // never save before the config was actually read (mirrors cloudLoaded)
      setPrimaryConfig((prev) => (prev ? { ...prev, immutable: next } : prev));
      setImmState("saving");
      setImmErr(null);
      try {
        const r = await savePrimarySafety({ immutable: next });
        if (!r.ok) {
          setPrimaryConfig((prev) => (prev ? { ...prev, immutable: !next } : prev));
          setImmState("error");
          setImmErr(r.error ?? t("settings.error"));
          return;
        }
        setImmState("saved");
        setTimeout(() => setImmState("idle"), 3000);
      } catch (e) {
        setPrimaryConfig((prev) => (prev ? { ...prev, immutable: !next } : prev));
        setImmState("error");
        setImmErr(e instanceof Error ? e.message : t("settings.error"));
        return;
      }
      if (next) void runTamper();
      else {
        setVerdict(null);
        setTamperState("idle");
      }
      return;
    }
    setSettings((prev) => (prev ? { ...prev, [immKey]: next } : prev));
    const ok = await save({ [immKey]: next } as Partial<Settings>, setImmState, setImmErr);
    if (!ok) {
      // Roll back the optimistic toggle; immErr / immState==='error' show the reason.
      setSettings((prev) => (prev ? { ...prev, [immKey]: !next } : prev));
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
            <button
              type="button"
              onClick={() => void genSnippet()}
              disabled={snipState === "busy"}
              className="self-start rounded-control bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
            >
              {snipState === "busy" ? t("common.saving") : t("offsite.wizard.generate")}
            </button>
          )}
          {snipState === "error" && snipErr && <span className="text-xs text-statusFail">{snipErr}</span>}
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
                saves only the repo URL so it can never clobber that cadence. */}
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() =>
                  void save(
                    { [repoKey]: settings[repoKey] } as Partial<Settings>,
                    setRepoState,
                    setRepoErr
                  )
                }
                disabled={repoState === "saving"}
                className="rounded-control bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
              >
                {repoState === "saving" ? t("common.saving") : t("offsite.wizard.saveRepo")}
              </button>
              {repoState === "saved" && <span className="text-xs text-statusOk">{t("settings.saved")}</span>}
              {repoState === "error" && repoErr && <span className="text-xs text-statusFail">{repoErr}</span>}
            </div>
          </>
        )}

        {/* REST credentials — reuse the cloud-credential endpoints. Only the REST
            backend needs a username/password; rclone/s3 carry their own auth in
            their own config, so this block would be pure noise for them (#131). */}
        {urlBackend === "rest" && (
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface p-3 mt-1">
            <span className="text-xs font-medium text-carbon-textSub">{t("offsite.wizard.credentials")}</span>
            <label dir="ltr" className="flex flex-col gap-1 text-xs font-mono text-carbon-textSub text-start">
              RESTIC_REST_USERNAME
              <input
                value={cloud.restUser}
                onChange={(e) => setCloudState((p) => ({ ...p, restUser: e.target.value }))}
                spellCheck={false}
                className={`${inputCls} text-start`}
              />
            </label>
            <label dir="ltr" className="flex flex-col gap-1 text-xs font-mono text-carbon-textSub text-start">
              RESTIC_REST_PASSWORD
              <RevealInput
                {...revealRestPassword}
                value={cloud.restPassword}
                onChange={(e) => setCloudState((p) => ({ ...p, restPassword: e.target.value }))}
                spellCheck={false}
                placeholder={restPwSet ? t("cloud.secretSet") : ""}
                wrapperClassName="w-full"
                className={inputCls}
              />
            </label>
            {cloudLoadErr && <span className="text-xs text-statusFail">{cloudLoadErr}</span>}
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => void saveCreds()}
                disabled={credState === "saving" || !cloudLoaded}
                className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
              >
                {credState === "saving" ? t("common.saving") : t("offsite.wizard.saveCreds")}
              </button>
              {credState === "saved" && <span className="text-xs text-statusOk">{t("settings.saved")}</span>}
              {credState === "error" && credErr && <span className="text-xs text-statusFail">{credErr}</span>}
            </div>
          </div>
        )}

        {/* Connection test */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => void runTest()}
            disabled={testState === "busy"}
            className="rounded-control bg-carbon-surface px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {testState === "busy" ? t("offsite.testing") : t("offsite.test")}
          </button>
          {testState === "ok" && <span className="text-xs text-statusOk">{t("offsite.testOk")}</span>}
          {testState === "uninit" && <span className="text-xs text-statusWarn">{t("offsite.testUninitialized")}</span>}
          {testState === "fail" && (
            <span className="text-xs text-statusFail wrap-break-word">{testErr ?? t("offsite.testFailed")}</span>
          )}
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
            hideLabel
            label={t("offsite.immutable")}
            checked={immutable}
            onChange={(next) => void toggleImmutable(next)}
            disabled={immState === "saving"}
            className="mt-0.5"
          />
        </div>
        {immState === "error" && immErr && <span className="text-xs text-statusFail">{immErr}</span>}

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
            instead of an active-looking button that leads nowhere (#131). */}
        {urlBackend === "rest" ? (
          <div className="flex items-center gap-3 flex-wrap">
            <button
              type="button"
              onClick={() => void runTamper()}
              disabled={tamperState === "busy" || !immutable}
              className="rounded-control bg-carbon-surface px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {tamperState === "busy" ? t("offsite.tamperTesting") : t("offsite.tamperTestNow")}
            </button>
            {tamperState === "done" && verdict && (
              <span className={`text-sm wrap-break-word ${verdictColor}`}>
                {verdictGlyph && <span aria-hidden="true">{verdictGlyph}&nbsp;</span>}
                {verdictText}
              </span>
            )}
            {tamperState === "error" && tamperErr && (
              <span className="text-sm text-statusFail wrap-break-word">{tamperErr}</span>
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
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("settings.limitUpload")}</span>
              <input
                type="number"
                min={0}
                value={pLimitUpload}
                onChange={(e) => setPLimitUpload(Math.max(0, parseInt(e.target.value, 10) || 0))}
                className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("settings.limitDownload")}</span>
              <input
                type="number"
                min={0}
                value={pLimitDownload}
                onChange={(e) => setPLimitDownload(Math.max(0, parseInt(e.target.value, 10) || 0))}
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
              onChange={(e) => setPBudget(Math.max(0, parseInt(e.target.value, 10) || 0))}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
            />
          </label>
          <p className="text-xs text-carbon-textMuted leading-relaxed">{t("settings.primaryRemote.budgetHint")}</p>
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() =>
                void (async () => {
                  setBudgetState("saving");
                  try {
                    const r = await savePrimarySafety({ limitUpload: pLimitUpload, limitDownload: pLimitDownload, growthBudgetGb: pBudget });
                    if (r.ok) {
                      setBudgetState("saved");
                      setTimeout(() => setBudgetState("idle"), 3000);
                    } else {
                      setBudgetState("error");
                      setBudgetErr(r.error ?? t("settings.error"));
                    }
                  } catch (e) {
                    setBudgetState("error");
                    setBudgetErr(e instanceof Error ? e.message : t("settings.error"));
                  }
                })()
              }
              disabled={budgetState === "saving" || !primaryLoaded}
              className="rounded-control bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
            >
              {budgetState === "saving" ? t("common.saving") : t("settings.save")}
            </button>
            {budgetState === "saved" && <span className="text-xs text-statusOk">{t("settings.saved")}</span>}
            {budgetState === "error" && budgetErr && <span className="text-xs text-statusFail">{budgetErr}</span>}
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={stepTitle}>{t("offsite.retention.title")}</span>
          <div className="flex flex-col gap-1.5">
            {([
              ["farside", "offsite.retention.farside"],
              ["window", "offsite.retention.window"],
              ["grow", "offsite.retention.grow"],
            ] as const).map(([val, label]) => (
              <label key={val} className="flex items-center gap-2 text-sm text-carbon-text cursor-pointer">
                <input
                  type="radio"
                  name={`retention-${domain}`}
                  checked={retention === val}
                  onChange={() => setRetention(val)}
                  style={{ accentColor: "var(--accent)" }}
                />
                {t(label)}
              </label>
            ))}
          </div>

          {retention === "farside" && (
            <div className="flex flex-col gap-1">
              <p className="text-xs text-carbon-textMuted">{t("offsite.retention.farsideHint")}</p>
              <CopyBlock text={cronHint} t={t} />
            </div>
          )}
          {retention === "window" && urlBackend === "rest" && (
            <p className="text-xs text-carbon-textMuted leading-relaxed">{t("offsite.retention.windowHint")}</p>
          )}
          {/* "window" (a temporary second rest-server) is REST-specific — for any
              other backend the instructions above don't apply, so say so instead
              of silently showing nothing for a selected option (#131). */}
          {retention === "window" && urlBackend !== "rest" && (
            <p className="text-xs text-carbon-textMuted leading-relaxed">{t("offsite.retention.windowRestOnly")}</p>
          )}
          {retention === "grow" && (
            <div className="flex flex-col gap-2">
              <p className="text-xs text-carbon-textMuted leading-relaxed">{t("offsite.retention.growHint")}</p>
              <label className="flex flex-col gap-1 max-w-48">
                <span className="text-xs text-carbon-textSub">{t("offsite.retention.budget")}</span>
                <input
                  type="number"
                  min={0}
                  value={settings.offsiteGrowthBudgetGB}
                  onChange={(e) => {
                    const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                    setSettings((prev) => (prev ? { ...prev, offsiteGrowthBudgetGB: n } : prev));
                  }}
                  className="rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well"
                />
              </label>
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() =>
                    void save(
                      { offsiteGrowthBudgetGB: settings.offsiteGrowthBudgetGB },
                      setBudgetState,
                      () => undefined
                    )
                  }
                  disabled={budgetState === "saving"}
                  className="rounded-control bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
                >
                  {budgetState === "saving" ? t("common.saving") : t("offsite.retention.saveBudget")}
                </button>
                {budgetState === "saved" && <span className="text-xs text-statusOk">{t("settings.saved")}</span>}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
