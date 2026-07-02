import { useEffect, useState } from "react";
import type { Settings, DeploySnippetData } from "../lib/api";
import { deploySnippet, tamperTest, testOffsite, getCloud, setCloud } from "../lib/api";
import { useT } from "../lib/i18n";

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

type Domain = "containers" | "vms" | "flash";
type T = ReturnType<typeof useT>["t"];
type SaveState = "idle" | "saving" | "saved" | "error";

// Per-domain Settings keys — the wizard binds to the exact same fields the
// off-site card and immutable flags already persist (no parallel state).
const REPO_KEY = {
  containers: "containersOffsite",
  vms: "vmsOffsite",
  flash: "flashOffsite",
} as const;
const SCHED_KEY = {
  containers: "containersOffsiteSchedule",
  vms: "vmsOffsiteSchedule",
  flash: "flashOffsiteSchedule",
} as const;
const IMM_KEY = {
  containers: "containersOffsiteImmutable",
  vms: "vmsOffsiteImmutable",
  flash: "flashOffsiteImmutable",
} as const;

// "none" = empty URL (neutral prompt — no REST snippet, no caveat); "other" =
// a recognized non-REST scheme (sftp/b2/gs/azure) or an unrecognized/bare path
// that must NOT get the REST deploy-snippet flow.
type Backend = "rest" | "rclone" | "s3" | "other" | "none";

function inferBackend(url: string): Backend {
  const u = url.trim();
  if (u === "") return "none";
  if (u.startsWith("rclone:")) return "rclone";
  if (u.startsWith("s3:") || u.startsWith("s3://")) return "s3";
  if (u.startsWith("rest:") || u.startsWith("http://") || u.startsWith("https://")) return "rest";
  // Recognized non-REST schemes (and anything else) are "other": no REST snippet,
  // no rclone/s3 caveat — the wizard makes no false REST assumption.
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
      <pre className="flex-1 overflow-x-auto rounded border border-carbon-border bg-carbon-background p-2 text-[11px] leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <button
        type="button"
        onClick={() => void copy()}
        className="shrink-0 rounded bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover"
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
}) {
  const repoKey = REPO_KEY[domain];
  const schedKey = SCHED_KEY[domain];
  const immKey = IMM_KEY[domain];

  const repoURL = settings[repoKey];
  const immutable = settings[immKey];
  const [backend, setBackend] = useState<Backend>(() => inferBackend(repoURL));

  // Step 2 — rest-server deploy snippet (generated on demand, never persisted).
  const [snippet, setSnippet] = useState<DeploySnippetData | null>(null);
  const [snipState, setSnipState] = useState<"idle" | "busy" | "error">("idle");
  const [snipErr, setSnipErr] = useState<string | null>(null);

  // Step 3 — REST credentials (reuses the cloud-credential endpoints). S3 fields
  // are loaded + preserved on save so this flow never clobbers them.
  const [cloud, setCloudState] = useState({ s3KeyId: "", s3Region: "", restUser: "", restPassword: "" });
  const [restPwSet, setRestPwSet] = useState(false);
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
  function patchSched(v: string) {
    setSettings((prev) => (prev ? { ...prev, [schedKey]: v } : prev));
  }

  async function genSnippet() {
    setSnipState("busy");
    setSnipErr(null);
    try {
      const r = await deploySnippet(domain);
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
      const r = await setCloud({ s3KeyId: cloud.s3KeyId, s3Secret: "", s3Region: cloud.s3Region, restUser: cloud.restUser, restPassword: cloud.restPassword });
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
      const r = await testOffsite(domain);
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
      const r = await tamperTest(domain);
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

  // Toggling immutable ON persists the flag AND — only after a CONFIRMED save —
  // proves it with a tamper test (the verdict is shown verbatim). A failed save
  // rolls the optimistic flip back and surfaces the error, so a green "protected"
  // verdict can never appear while the server flag actually stayed OFF.
  async function toggleImmutable(next: boolean) {
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
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]";
  const stepTitle = "text-xs font-semibold text-carbon-textSub uppercase tracking-widest";

  // Far-side prune cron hint (includes --keep-within 14d + a snapshot-count note).
  const cronHint = `# Run on the storage box itself — BombVault stays append-only:
0 4 * * 0 restic -r /path/on/storage-box/restic/bombvault-${domain}/${domain} forget \\
  --keep-within 14d --keep-weekly 8 --keep-monthly 12 --prune
# note: watch for a sudden snapshot-count drop (retention-policy timestamp attack)`;

  const verdictText = verdict
    ? !verdict.testable
      ? t("offsite.tamperUnverifiable")
      : verdict.protected
        ? t("offsite.tamperOk")
        : t("offsite.tamperFail")
    : "";
  const verdictColor = verdict
    ? !verdict.testable
      ? "text-[#f1c21b]"
      : verdict.protected
        ? "text-[#6fdc8c]"
        : "text-[#ff8389]"
    : "";

  // Backend caveats key off the ACTUAL repo URL (live), not the Step-1 radio — so
  // a saved/edited rclone: or s3: URL always shows its warning, and a REST/empty
  // URL never shows a spurious one.
  const urlBackend = inferBackend(repoURL);

  return (
    <div className="mt-2 flex flex-col gap-4 rounded-lg border border-carbon-border bg-carbon-surface2 p-4">
      {/* Step 1 — backend choice */}
      <div className="flex flex-col gap-2">
        <span className={stepTitle}>{t("offsite.wizard.step1")}</span>
        <div className="flex flex-col gap-1.5">
          {([
            ["rest", "offsite.wizard.backendRest"],
            ["rclone", "offsite.wizard.backendRclone"],
            ["s3", "offsite.wizard.backendS3"],
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
      </div>

      {/* Step 2 — rest-server deploy snippet */}
      {backend === "rest" && (
        <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={stepTitle}>{t("offsite.wizard.step2")}</span>
          <p className="text-xs text-carbon-textMuted">{t("offsite.wizard.step2Hint")}</p>
          {!snippet && (
            <button
              type="button"
              onClick={() => void genSnippet()}
              disabled={snipState === "busy"}
              className="self-start rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
            >
              {snipState === "busy" ? t("common.saving") : t("offsite.wizard.generate")}
            </button>
          )}
          {snipState === "error" && snipErr && <span className="text-xs text-[#ff8389]">{snipErr}</span>}
          {snippet && (
            <div className="flex flex-col gap-2">
              <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2 text-xs text-[#f1c21b] leading-relaxed">
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
              <button
                type="button"
                onClick={() => void genSnippet()}
                className="self-start text-xs text-[#78a9ff] hover:underline"
              >
                {t("offsite.wizard.regenerate")}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Step 3 — repo URL + schedule + credentials + connection test */}
      <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
        <span className={stepTitle}>{t("offsite.wizard.step3")}</span>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("offsite.wizard.repoUrl")}</span>
          <input
            value={repoURL}
            spellCheck={false}
            onChange={(e) => patchRepo(e.target.value)}
            placeholder={t("offsite.wizard.repoUrlPlaceholder")}
            className={inputCls}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("settings.schedule")}</span>
          <input
            value={settings[schedKey]}
            spellCheck={false}
            onChange={(e) => patchSched(e.target.value)}
            placeholder={t("offsite.schedulePlaceholder")}
            className={inputCls}
          />
        </label>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() =>
              void save(
                { [repoKey]: settings[repoKey], [schedKey]: settings[schedKey] } as Partial<Settings>,
                setRepoState,
                setRepoErr
              )
            }
            disabled={repoState === "saving"}
            className="rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
          >
            {repoState === "saving" ? t("common.saving") : t("offsite.wizard.saveRepo")}
          </button>
          {repoState === "saved" && <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>}
          {repoState === "error" && repoErr && <span className="text-xs text-[#ff8389]">{repoErr}</span>}
        </div>

        {/* REST credentials — reuse the cloud-credential endpoints. */}
        <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface border border-carbon-border p-3 mt-1">
          <span className="text-xs font-medium text-carbon-textSub">{t("offsite.wizard.credentials")}</span>
          <label className="flex flex-col gap-1 text-xs font-mono text-carbon-textSub">
            RESTIC_REST_USERNAME
            <input
              value={cloud.restUser}
              onChange={(e) => setCloudState((p) => ({ ...p, restUser: e.target.value }))}
              spellCheck={false}
              className={inputCls}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-mono text-carbon-textSub">
            RESTIC_REST_PASSWORD
            <input
              type="password"
              value={cloud.restPassword}
              onChange={(e) => setCloudState((p) => ({ ...p, restPassword: e.target.value }))}
              spellCheck={false}
              placeholder={restPwSet ? t("cloud.secretSet") : ""}
              className={inputCls}
            />
          </label>
          {cloudLoadErr && <span className="text-xs text-[#ff8389]">{cloudLoadErr}</span>}
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => void saveCreds()}
              disabled={credState === "saving" || !cloudLoaded}
              className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {credState === "saving" ? t("common.saving") : t("offsite.wizard.saveCreds")}
            </button>
            {credState === "saved" && <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>}
            {credState === "error" && credErr && <span className="text-xs text-[#ff8389]">{credErr}</span>}
          </div>
        </div>

        {/* Connection test */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => void runTest()}
            disabled={testState === "busy"}
            className="rounded-lg border border-carbon-border bg-carbon-surface px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {testState === "busy" ? t("offsite.testing") : t("offsite.test")}
          </button>
          {testState === "ok" && <span className="text-xs text-[#6fdc8c]">{t("offsite.testOk")}</span>}
          {testState === "uninit" && <span className="text-xs text-[#f1c21b]">{t("offsite.testUninitialized")}</span>}
          {testState === "fail" && (
            <span className="text-xs text-[#ff8389] break-words">{testErr ?? t("offsite.testFailed")}</span>
          )}
        </div>
      </div>

      {/* Step 4 — immutable (append-only) toggle + verbatim tamper verdict */}
      <div className="flex flex-col gap-2 border-t border-carbon-border pt-3">
        <span className={stepTitle}>{t("offsite.wizard.step4")}</span>
        <div className="flex items-start justify-between gap-4">
          <div className="flex flex-col gap-0.5">
            <span id={`imm-label-${domain}`} className="text-sm text-carbon-text">{t("offsite.immutable")}</span>
            <span className="text-xs text-carbon-textMuted">{t("offsite.immutableHint")}</span>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={immutable}
            aria-labelledby={`imm-label-${domain}`}
            disabled={immState === "saving"}
            onClick={() => void toggleImmutable(!immutable)}
            className={`relative inline-flex h-5 w-9 shrink-0 mt-0.5 items-center rounded-full transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#78a9ff] disabled:opacity-50 ${
              immutable ? "bg-accent" : "bg-carbon-surface3"
            }`}
          >
            <span
              className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
                immutable ? "translate-x-[18px]" : "translate-x-[3px]"
              }`}
            />
          </button>
        </div>
        {immState === "error" && immErr && <span className="text-xs text-[#ff8389]">{immErr}</span>}

        {/* Backend-specific caveats (Step 5) — driven by the live repo URL. */}
        {urlBackend === "rclone" && (
          <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2 text-xs text-[#f1c21b] leading-relaxed">
            {t("offsite.rcloneWarning")}
          </div>
        )}
        {urlBackend === "s3" && (
          <div className="rounded-lg bg-carbon-surface border border-carbon-border px-3 py-2 text-xs text-carbon-textSub leading-relaxed">
            {t("offsite.s3Unverified")}
          </div>
        )}

        {/* Verbatim tamper verdict + a manual "test now". */}
        <div className="flex items-center gap-3 flex-wrap">
          <button
            type="button"
            onClick={() => void runTamper()}
            disabled={tamperState === "busy" || !immutable}
            className="rounded-lg border border-carbon-border bg-carbon-surface px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {tamperState === "busy" ? t("offsite.tamperTesting") : t("offsite.tamperTestNow")}
          </button>
          {tamperState === "done" && verdict && (
            <span className={`text-sm break-words ${verdictColor}`}>{verdictText}</span>
          )}
          {tamperState === "error" && tamperErr && (
            <span className="text-sm text-[#ff8389] break-words">{tamperErr}</span>
          )}
        </div>
      </div>

      {/* Step 6 — retention strategy chooser */}
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
        {retention === "window" && (
          <p className="text-xs text-carbon-textMuted leading-relaxed">{t("offsite.retention.windowHint")}</p>
        )}
        {retention === "grow" && (
          <div className="flex flex-col gap-2">
            <p className="text-xs text-carbon-textMuted leading-relaxed">{t("offsite.retention.growHint")}</p>
            <label className="flex flex-col gap-1 max-w-[12rem]">
              <span className="text-xs text-carbon-textSub">{t("offsite.retention.budget")}</span>
              <input
                type="number"
                min={0}
                value={settings.offsiteGrowthBudgetGB}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, offsiteGrowthBudgetGB: n } : prev));
                }}
                className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-none focus:border-[#78a9ff]"
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
                className="rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
              >
                {budgetState === "saving" ? t("common.saving") : t("offsite.retention.saveBudget")}
              </button>
              {budgetState === "saved" && <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
