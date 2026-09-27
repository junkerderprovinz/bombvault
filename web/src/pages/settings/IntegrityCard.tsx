// IntegrityCard runs per-domain repository maintenance: verify (restic check),
// unlock (clear stale locks), prune (reclaim space), and a restore drill. The
// drill is either an integrity check (restic check --read-data-subset on the
// selected repo) or a real restore of the newest off-site snapshot into a
// sandbox, verified and cleaned up afterwards. Config has no drill, because its
// recovery path is the staged restart rather than a sandbox restore. The drill
// targets are shared settings, saved through the parent's merging save().
import { Button } from "../../components/Button";
import { CheckDraw } from "../../components/CheckDraw";
import { InfoBubble } from "../../components/InfoBubble";
import { HUE_OFFSET, Selector } from "../../components/Selector";
import { SelectField } from "../../components/SelectField";
import { IconCheckCircle } from "../../components/Sidebar";
import { RepoSource, SourceToggle, isOffsiteSource } from "../../components/SourceToggle";
import { IconKey, IconPrune } from "../../components/glyphs";
import { useAdvanced } from "../../lib/advanced";
import { heldTagLabels } from "../../lib/anomalies";
import { Container, RestoreDrill, Settings, VM, checkDomain, getDrills, getStatus, listContainers, listVMs, pruneDomain, runDrill, tamperTest, unlockDomain } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { Card, type SaveState } from "./shared";
import { ReactNode, useEffect, useState } from "react";

export function IntegrityCard({
  t,
  settings,
  setSettings,
  save,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  hueIndex?: number;
}) {
  const { advanced } = useAdvanced();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  type ActState = "idle" | "busy" | "ok" | "fail";
  type DrillKind = "subset" | "dr";
  const [state, setState] = useState<Record<string, ActState>>({});
  // A failed action reports its message in a toast and shakes the button that
  // caused it. The value is a counter rather than a flag, so the same button
  // failing twice in a row shakes twice.
  const [shake, setShake] = useState<Record<string, number>>({});
  const [source, setSource] = useState<RepoSource>("local");
  const [kind, setKind] = useState<DrillKind>("subset");
  // The last recorded drill per domain (for the current source), keyed by domain.
  const [lastDrill, setLastDrill] = useState<Record<string, RestoreDrill | null>>({});
  // The append-only check is the off-site wizard's tamper test, under its
  // plainer name because this card is where users look for checks. A verdict
  // is not testable (amber), protected (green) or delete accepted (red);
  // lastTamper feeds the idle "last checked" caption from /api/status.
  type TamperRes =
    | { kind: "busy" }
    // `detail` is the far side's own answer.
    | { kind: "verdict"; testable: boolean; protected: boolean; detail?: string }
    | { kind: "error"; message: string };
  const [tamper, setTamper] = useState<Record<string, TamperRes | undefined>>({});
  const [lastTamper, setLastTamper] = useState<Record<string, { at: number; ok: boolean } | null>>({});
  // Container list feeding the DR-drill target dropdown (kind "dr", containers).
  const [containers, setContainers] = useState<Container[]>([]);
  // VM list feeding the DR-drill target dropdown (kind "dr", VMs).
  const [vms, setVMs] = useState<VM[]>([]);
  // save() reports the outcome in a toast, so only the setters are used.
  const [, setTgtState] = useState<SaveState>("idle");
  const [, setTgtError] = useState<string | null>(null);
  const [, setTgtVMState] = useState<SaveState>("idle");
  const [, setTgtVMError] = useState<string | null>(null);

  type Domain = "containers" | "vms" | "flash" | "files" | "zfs";
  type Action = "verify" | "unlock" | "prune";

  const domains: { key: Domain; label: string }[] = [
    { key: "containers", label: t("settings.containersEnabled") },
    { key: "vms", label: t("settings.vmsEnabled") },
    { key: "flash", label: t("settings.flashEnabled") },
    { key: "files", label: t("settings.filesEnabled") },
    { key: "zfs", label: t("settings.zfsEnabled") },
  ];

  // Load the containers once for the DR-drill target picker (includes orphans
  // that still have off-site backups, so any drillable target is selectable).
  useEffect(() => {
    let active = true;
    listContainers()
      .then((r) => {
        if (active && r.ok) setContainers(r.containers ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // Load the VMs once for the DR-drill target picker, same reasoning as containers.
  useEffect(() => {
    let active = true;
    listVMs()
      .then((r) => {
        if (active && r.ok) setVMs(r.vms ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // Load the latest drill for each domain on mount and whenever the source
  // changes, so the "last verified" line reflects the selected repo.
  useEffect(() => {
    let active = true;
    for (const { key: domain } of domains) {
      getDrills(domain, source, 1)
        .then((r) => {
          if (!active) return;
          if (r.ok) setLastDrill((m) => ({ ...m, [domain]: r.latest ?? null }));
        })
        .catch(() => undefined);
    }
    return () => {
      active = false;
    };
    // domains is a stable literal list; re-run only when the source changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  // Load each domain's last tamper-test verdict once, so the append-only row's
  // idle caption mirrors the drill row's "last verified" line. The check always
  // probes the off-site repo, so the source toggle never re-triggers this.
  useEffect(() => {
    let active = true;
    getStatus()
      .then((r) => {
        if (!active || !r.ok || !r.domains) return;
        const m: Record<string, { at: number; ok: boolean } | null> = {};
        for (const d of r.domains) {
          m[d.domain] = d.lastTamperAt > 0 ? { at: d.lastTamperAt, ok: d.lastTamperOK } : null;
        }
        setLastTamper(m);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  // runTamperFor checks that the domain's off-site repo still refuses deletes,
  // through the same API as the wizard's append-only test.
  async function runTamperFor(domain: Domain) {
    setTamper((m) => ({ ...m, [domain]: { kind: "busy" } }));
    try {
      const r = await tamperTest(domain);
      if (r.ok) {
        setTamper((m) => ({
          ...m,
          [domain]: { kind: "verdict", testable: !!r.testable, protected: !!r.protected, detail: r.detail },
        }));
        // A decisive verdict is also the new "last checked" fact; a not-testable
        // repo records no verdict server-side, so leave the caption untouched.
        if (r.testable) {
          setLastTamper((m) => ({ ...m, [domain]: { at: Math.floor(Date.now() / 1000), ok: !!r.protected } }));
        }
        // The verdict lands in /api/status, so tell the dashboard to refetch.
        window.dispatchEvent(new Event("bv:settings-changed"));
        // A "not protected" verdict is not a failed action: the test ran and
        // reported bad news, so it stays an inline verdict rather than a toast.
      } else {
        const message = r.error ?? t("offsite.tamperError");
        setTamper((m) => ({ ...m, [domain]: { kind: "error", message } }));
        push(message, "fail");
        bumpShake(`${domain}:tamper`);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("offsite.tamperError");
      setTamper((m) => ({ ...m, [domain]: { kind: "error", message } }));
      push(message, "fail");
      bumpShake(`${domain}:tamper`);
    }
  }

  async function run(domain: Domain, action: Action) {
    if (action === "prune" && !(await confirm(t("integrity.pruneConfirm")))) return;
    const key = `${domain}:${action}`;
    setState((s) => ({ ...s, [key]: "busy" }));
    try {
      const r =
        action === "verify" ? await checkDomain(domain, source)
        : action === "unlock" ? await unlockDomain(domain, source)
        : await pruneDomain(domain, source);
      // A repository shared with another domain gets only its stale locks
      // cleared, so unlock can report either outcome and still have changed
      // nothing there. The server names those repositories on both paths; this
      // is a known limit, so it is a warning rather than the error.
      const skipped: string[] = "skipped" in r && Array.isArray(r.skipped) ? r.skipped : [];
      if (skipped.length) {
        push(t("integrity.unlockPartial").replace("{list}", skipped.join(", ")), "warn");
      }
      const paused: string[] = "paused" in r && Array.isArray(r.paused) ? r.paused : [];
      if (r.ok && paused.length) {
        push(t("anomaly.prunePaused").replace("{names}", heldTagLabels(paused, t).join(", ")), "warn");
      }
      if (r.ok) {
        setState((s) => ({ ...s, [key]: "ok" }));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        push(r.error ?? t("integrity.failed"), "fail");
        bumpShake(key);
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      push(err instanceof Error ? err.message : t("integrity.failed"), "fail");
      bumpShake(key);
    }
  }

  // runDrillFor runs a restore drill and records its result under
  // "<domain>:drill". A "dr" drill restores into a sandbox for real, so it
  // always targets the off-site repo and asks for confirmation first.
  async function runDrillFor(domain: Domain) {
    if (kind === "dr" && !(await confirm(t("drill.confirmDR")))) return;
    const key = `${domain}:drill`;
    setState((s) => ({ ...s, [key]: "busy" }));
    try {
      const r = await runDrill(domain, kind === "dr" ? "offsite" : source, kind);
      if (r.ok && r.drill) {
        const drill = r.drill;
        setLastDrill((m) => ({ ...m, [domain]: drill }));
        setState((s) => ({ ...s, [key]: drill.ok ? "ok" : "fail" }));
        if (!drill.ok) {
          push(drill.detail || t("verify.failed"), "fail");
          bumpShake(key);
        }
        // A recorded drill, pass or fail, changes /api/status, so tell the
        // dashboard to refetch its drill pills.
        window.dispatchEvent(new Event("bv:settings-changed"));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        push(r.error ?? t("verify.failed"), "fail");
        bumpShake(key);
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      push(err instanceof Error ? err.message : t("verify.failed"), "fail");
      bumpShake(key);
    }
  }

  const actions: { key: Action; label: string; labelKey: string; glyph: ReactNode; busy: string }[] = [
    {
      key: "verify",
      label: t("integrity.verify"),
      labelKey: "integrity.verify",
      glyph: <IconCheckCircle />,
      busy: t("integrity.checking"),
    },
    {
      key: "unlock",
      label: t("integrity.unlock"),
      labelKey: "integrity.unlock",
      // A key rather than the open padlock glyphFor maps /unlock/ to.
      glyph: <IconKey />,
      busy: "…",
    },
    // Prune deletes snapshots, so it stays behind Advanced.
    ...(advanced
      ? [
          {
            key: "prune" as Action,
            label: t("integrity.prune"),
            labelKey: "integrity.prune",
            glyph: <IconPrune />,
            busy: "…",
          },
        ]
      : []),
  ];

  // Only a domain whose off-site repo is set and flagged immutable gets the
  // append-only check, as in the wizard; anywhere else it could only return a
  // backend error.
  const appendOnlyEligible: Record<Domain, boolean> = {
    containers: settings.containersOffsite !== "" && settings.containersOffsiteImmutable,
    vms: settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable,
    flash: settings.flashOffsite !== "" && settings.flashOffsiteImmutable,
    files: settings.filesOffsite !== "" && settings.filesOffsiteImmutable,
    zfs: settings.zfsOffsite !== "" && settings.zfsOffsiteImmutable,
  };

  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 glim-field-focus-well";

  return (
    <Card title={t("integrity.title")} hint={t("integrity.hint")} hueIndex={hueIndex}>
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
        <SourceToggle
          source={source}
          onChange={(next) => {
            // The results belong to the previous source; a "healthy" result
            // must not carry over to a repo where nothing has run. The effect
            // above reloads the last drill for `next`.
            setSource(next);
            setState({});
            setLastDrill({});
          }}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* Drill type: a subset integrity check or a real off-site restore.
          The small "well" variant, the same props as NotifyCard's selector in
          the same role. */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("drill.kindLabel")}</span>
        <Selector
          items={[
            { id: "subset", label: t("drill.kindSubset") },
            { id: "dr", label: t("drill.kindDR") },
          ]}
          label={t("drill.kindLabel")}
          // The one shared start in the table; see HUE_OFFSET for why that is
          // arithmetic rather than an oversight.
          hueOffset={HUE_OFFSET.drillKind}
          select="one"
          active={kind}
          onChange={(val) => {
            // Clear any lingering per-domain result so a subset "healthy"
            // doesn't read as a DR pass (or vice versa) after switching kind.
            setKind(val as DrillKind);
            setState({});
          }}
          variant="well"
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* DR-drill controls: an explainer + the container/VM target pickers. Each
          target is a shared setting (settings.drDrillTarget / drDrillTargetVm)
          saved via the parent's baseline-merging save(), so it never clobbers
          other cards' edits. Flash and files have no picker (their whole
          snapshot is restored). */}
      {kind === "dr" && (
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            <span className="flex items-center gap-1">
              {t("drill.target")}
              <InfoBubble tip={t("drill.drNote")} />
            </span>
            <SelectField
              value={settings.drDrillTarget}
              onChange={(v) => {
                setSettings((prev) => (prev ? { ...prev, drDrillTarget: v } : prev));
                void save({ drDrillTarget: v }, setTgtState, setTgtError);
              }}
              label={t("drill.target")}
              options={[
                { value: "", label: t("drill.targetMostRecent") },
                ...containers.map((c) => ({ value: c.name, label: c.name })),
              ]}
              className={selectCls}
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            {t("drill.targetVM")}
            <SelectField
              value={settings.drDrillTargetVm}
              onChange={(v) => {
                setSettings((prev) => (prev ? { ...prev, drDrillTargetVm: v } : prev));
                void save({ drDrillTargetVm: v }, setTgtVMState, setTgtVMError);
              }}
              label={t("drill.targetVM")}
              options={[
                { value: "", label: t("drill.targetMostRecent") },
                // The value is the raw libvirt name: pickDRSnapshot (service.go)
                // matches it against the "vm:"+name backup tag, never the
                // display-only friendly name a TrueNAS VM shows here.
                ...vms.map((vm) => ({ value: vm.libvirtName, label: vm.name })),
              ]}
              className={selectCls}
            />
          </label>
        </div>
      )}

      <div className="flex flex-col gap-3">
        {domains.map(({ key: domain, label }) => {
          const dKey = `${domain}:drill`;
          const drill = lastDrill[domain];
          const tRes = tamper[domain];
          const tLast = lastTamper[domain];
          return (
            <div key={domain} className="flex flex-col gap-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm text-carbon-textSub w-24 shrink-0">{label}</span>
                {actions.map((a) => {
                  const k = `${domain}:${a.key}`;
                  return (
                    <span key={a.key} className="inline-flex items-center gap-1">
                      <Button
                        key={shake[k] || 0}
                        label={a.label}
                        labelKey={a.labelKey}
                        glyph={a.glyph}
                        tone="neutral"
                        hueIndex={hueIndex}
                        onClick={() => void run(domain, a.key)}
                        disabled={state[k] === "busy"}
                        busy={state[k] === "busy"}
                        title={t(`integrity.${a.key}Hint`)}
                        className={shake[k] ? "glim-shake" : ""}
                      />
                      {state[k] === "ok" && (
                        <span className="inline-flex items-center gap-1 text-sm text-statusOk">
                          <CheckDraw />
                          {t("integrity.ok")}
                        </span>
                      )}
                      {/* The error text went to the toast; this short marker
                          matches the weight of the ok indicator. */}
                      {state[k] === "fail" && <span className="text-sm text-statusFail">{t("integrity.failedShort")}</span>}
                    </span>
                  );
                })}
                <Button
                  key={shake[dKey] || 0}
                  label={kind === "dr" ? t("drill.runDR") : t("verify.now")}
                  // The labelKey follows the label's branch, or the button
                  // would be sized for the other wording.
                  labelKey={kind === "dr" ? "drill.runDR" : "verify.now"}
                  glyph={<IconCheckCircle />}
                  tone="neutral"
                  hueIndex={hueIndex}
                  onClick={() => void runDrillFor(domain)}
                  disabled={state[dKey] === "busy"}
                  busy={state[dKey] === "busy"}
                  // The label stays fixed, so the running wording goes in the
                  // title, which shows the standing hint the rest of the time.
                  title={
                    state[dKey] === "busy"
                      ? kind === "dr" ? t("drill.runningDR") : t("verify.running")
                      : kind === "dr" ? t("drill.drNote") : t("verify.hint")
                  }
                  className={shake[dKey] ? "glim-shake" : ""}
                />
                {state[dKey] === "ok" && (
                  <span className="inline-flex items-center gap-1 text-sm text-statusOk">
                    <CheckDraw />
                    {t("verify.ok")}
                  </span>
                )}
                {state[dKey] === "fail" && (
                  <span className="text-sm text-statusFail">✗ {t("verify.failed")}</span>
                )}
                {/* Last recorded drill for this domain/source (idle state only).
                    Names which check ran (off-site restore or local integrity) and,
                    on a stored failure, the scrubbed reason. */}
                {state[dKey] !== "busy" && state[dKey] !== "ok" && state[dKey] !== "fail" && (
                  drill ? (
                    <>
                      <span className="text-xs text-carbon-textMuted">
                        {isOffsiteSource(drill.source) && drill.kind === "dr"
                          ? t("drill.checkOffsiteDr")
                          : t("drill.checkLocal")}
                        {" · "}
                        {t("verify.last").replace("{time}", relativeTime(t, drill.at))} {drill.ok ? "✓" : "✗"}
                      </span>
                      {!drill.ok && drill.detail && (
                        <span className="text-xs text-statusFail wrap-break-word" title={drill.detail}>
                          {t("drill.failReasonPrefix")} {drill.detail}
                        </span>
                      )}
                    </>
                  ) : (
                    <span className="text-xs text-carbon-textMuted">{t("verify.never")}</span>
                  )
                )}
              </div>

              {/* The append-only check always probes the off-site repo,
                  whatever the source. The glyph is its own node so RTL
                  locales place it correctly. */}
              {appendOnlyEligible[domain] && (
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="w-24 shrink-0" />
                  <Button
                    key={shake[`${domain}:tamper`] || 0}
                    label={t("integrity.appendOnly")}
                    labelKey="integrity.appendOnly"
                    tone="neutral"
                    onClick={() => void runTamperFor(domain)}
                    disabled={tRes?.kind === "busy"}
                    busy={tRes?.kind === "busy"}
                    title={t("integrity.appendOnlyHint")}
                  />
                  {tRes?.kind === "verdict" && (
                    <span
                      className={`text-sm wrap-break-word ${
                        !tRes.testable ? "text-statusWarn" : tRes.protected ? "text-statusOk" : "text-statusFail"
                      }`}
                    >
                      {tRes.testable && (
                        <span aria-hidden="true" className="inline-flex items-center">
                          {tRes.protected ? <CheckDraw /> : "✗"}&nbsp;
                        </span>
                      )}
                      {/* The server's own words rather than a fixed sentence,
                          as in OffsiteWizard.tsx. */}
                      {!tRes.testable
                        ? t("offsite.tamperUnverifiable")
                        : tRes.protected
                          ? t("offsite.tamperOk")
                          : tRes.detail
                            ? `${t("offsite.tamperFail")} — ${tRes.detail}`
                            : t("offsite.tamperFail")}
                    </span>
                  )}
                  {/* The test could not run: the toast has the message, the
                      page keeps a short fixed caption. */}
                  {tRes?.kind === "error" && (
                    <span className="text-sm text-statusFail">{t("offsite.tamperError")}</span>
                  )}
                  {/* Idle caption: the last recorded check, mirroring the drill
                      row's "Last verified …" line. */}
                  {!tRes &&
                    (tLast ? (
                      <span className="text-xs text-carbon-textMuted">
                        {t("integrity.appendOnlyLast").replace("{time}", relativeTime(t, tLast.at))} {tLast.ok ? "✓" : "✗"}
                      </span>
                    ) : (
                      <span className="text-xs text-carbon-textMuted">{t("integrity.appendOnlyNever")}</span>
                    ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
      {confirmDialog}
    </Card>
  );
}
