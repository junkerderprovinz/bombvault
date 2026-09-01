// IntegrityCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

// IntegrityCard runs per-domain repository maintenance: verify (restic check),
// unlock (clear stale locks), prune (reclaim space), and a restore-verification
// "drill". The drill has two kinds, chosen by the "Drill type" toggle:
//   - "Integrity check" (subset): restic check --read-data-subset on the selected
//     source repo — proves the backup data is intact + restorable.
//   - "Real restore (off-site)" (dr): a REAL sandbox restore of the newest
//     off-site snapshot, then verification + cleanup. All domains except config
//     (config's real recovery path is the in-place staged restart, not a sandbox
//     restore of the settings DB).
// The DR-drill target (which container's/VM's off-site snapshot to restore) binds
// to the shared settings.drDrillTarget / drDrillTargetVm via the parent's
// baseline-merging save().
import { Button } from "../../components/Button";
import { CheckDraw } from "../../components/CheckDraw";
import { InfoBubble } from "../../components/InfoBubble";
import { Selector } from "../../components/Selector";
import { IconCheckCircle } from "../../components/Sidebar";
import { RepoSource, SourceToggle, isOffsiteSource } from "../../components/SourceToggle";
import { IconKey, IconPrune } from "../../components/glyphs";
import { useAdvanced } from "../../lib/advanced";
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
  // Prune deletes snapshots, so it stays advanced-only even though the rest of
  // this card (verify, unlock, DR drill) is a first-class default-mode feature.
  const { advanced } = useAdvanced();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  type ActState = "idle" | "busy" | "ok" | "fail";
  type DrillKind = "subset" | "dr";
  const [state, setState] = useState<Record<string, ActState>>({});
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST (push below), never as permanent
  // page text, and the button that triggered it replays `.glim-shake` once.
  // `msg` (a per-key error STRING kept around to render inline forever) is
  // gone — the toast already carries the message the instant the action
  // fails, so nothing needs to remember it past that point. `shake` replaces
  // it: a per-key nonce, same shape, bumped on every failure so a fresh
  // `.glim-shake` class + key remount fires even for the SAME button failing
  // twice in a row (see ToggleRow's own shakeNonce doc comment for why a
  // bumped number, not a boolean, is required here).
  const [shake, setShake] = useState<Record<string, number>>({});
  const [source, setSource] = useState<RepoSource>("local");
  const [kind, setKind] = useState<DrillKind>("subset");
  // The last recorded drill per domain (for the current source), keyed by domain.
  const [lastDrill, setLastDrill] = useState<Record<string, RestoreDrill | null>>({});
  // Append-only check (#109): the off-site wizard's tamper test, surfaced here
  // under its plainer name because this card is where users look for checks.
  // TamperRes mirrors the wizard's tri-state verdict: not-testable (amber) /
  // protected (green) / delete-accepted (red); lastTamper feeds the idle
  // "append-only protection · Last checked …" caption from /api/status.
  type TamperRes =
    | { kind: "busy" }
    | { kind: "verdict"; testable: boolean; protected: boolean }
    | { kind: "error"; message: string };
  const [tamper, setTamper] = useState<Record<string, TamperRes | undefined>>({});
  const [lastTamper, setLastTamper] = useState<Record<string, { at: number; ok: boolean } | null>>({});
  // Container list feeding the DR-drill target dropdown (kind "dr", containers).
  const [containers, setContainers] = useState<Container[]>([]);
  // VM list feeding the DR-drill target dropdown (kind "dr", VMs).
  const [vms, setVMs] = useState<VM[]>([]);
  // Save state for the drill-target dropdowns (persisted via the parent
  // save()). GlimStone follow-up pass (v8.0.0): save() now pushes a toast on
  // both outcomes instead of setting a "saved"/"error" render state (see
  // save()'s own header comment), so only the setters are needed here —
  // save() still requires them as callback params, but nothing reads the
  // values back anymore.
  const [, setTgtState] = useState<SaveState>("idle");
  const [, setTgtError] = useState<string | null>(null);
  const [, setTgtVMState] = useState<SaveState>("idle");
  const [, setTgtVMError] = useState<string | null>(null);

  type Domain = "containers" | "vms" | "flash" | "files";
  type Action = "verify" | "unlock" | "prune";

  const domains: { key: Domain; label: string }[] = [
    { key: "containers", label: t("settings.containersEnabled") },
    { key: "vms", label: t("settings.vmsEnabled") },
    { key: "flash", label: t("settings.flashEnabled") },
    { key: "files", label: t("settings.filesEnabled") },
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
  // probes the OFF-SITE repo, so the source toggle never re-triggers this.
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

  // bumpShake replays `.glim-shake` on the button keyed `key` (see the
  // `shake` state's own doc comment above) — called from every failure branch
  // below alongside the toast `push()` that now carries the message instead
  // of a permanent inline sentence.
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  // runTamperFor proves the domain's off-site repo still refuses deletes — the
  // exact tamper-test API behind the wizard's "Test append-only now" (#109: users
  // searched for it here, and "append-only" is the plainer word for it).
  async function runTamperFor(domain: Domain) {
    setTamper((m) => ({ ...m, [domain]: { kind: "busy" } }));
    try {
      const r = await tamperTest(domain);
      if (r.ok) {
        setTamper((m) => ({
          ...m,
          [domain]: { kind: "verdict", testable: !!r.testable, protected: !!r.protected },
        }));
        // A decisive verdict is also the new "last checked" fact; a not-testable
        // repo records no verdict server-side, so leave the caption untouched.
        if (r.testable) {
          setLastTamper((m) => ({ ...m, [domain]: { at: Math.floor(Date.now() / 1000), ok: !!r.protected } }));
        }
        // The verdict + its run row land in /api/status (scorecard tamper state) —
        // broadcast so the dashboard refetches, mirroring runDrillFor above.
        window.dispatchEvent(new Event("bv:settings-changed"));
        // NOTE: a decisive "not protected" verdict (testable && !protected) is
        // NOT a failed action — the test ran fine and truthfully reported bad
        // news, exactly like a red health badge. That stays a persistent inline
        // verdict (rendered below), not a toast — only the "error" branches
        // below (the test itself couldn't run) are the "you clicked it, it
        // didn't work" case the toast+shake standard targets.
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

  // runDrillFor runs a restore-verification drill and records its result inline,
  // mirroring the per-action result-state pattern above (keyed "<domain>:drill").
  // A "dr" drill does a REAL off-site restore into a sandbox — it always targets
  // the off-site repo (source is ignored) and asks for confirmation first.
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
        // A recorded drill (pass OR fail) changes the shared /api/status the
        // dashboard scorecard reads. Broadcast so the Dashboard refetches its
        // drill / "proven restorable" pills without a page reload — mirrors how
        // saving settings signals the app to refresh.
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

  // Each action carries its own labelKey and glyph now ([324]). This row had
  // three buttons outside both engines: no key, no hue index, no glyph.
  //
  // Correcting what an earlier version of this comment claimed: a missing
  // `labelKey` does NOT cost the width stage — that is derived from `label`,
  // which is always present. What it costs is the GLYPH, since the key is
  // what glyphFor looks up, and a button with no glyph falls back to showing
  // its text in exactly the two modes that exist to hide it. Bad enough on
  // its own, and worth stating accurately: the same wrong reasoning would
  // have justified leaving a data-labelled button keyless.
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
      // A key, not the open padlock this label otherwise resolves to ([528],
      // jdp's call at the live review). `glyphFor` still maps /unlock/ to
      // `IconUnlock` and still keeps it distinct from the credentials key, and
      // that rule is deliberately left alone: it is the fallback for labels
      // nobody has looked at, while this button HAS been looked at. Overriding
      // at the call site is exactly what the `glyph` prop exists for.
      glyph: <IconKey />,
      busy: "…",
    },
    // Prune deletes snapshots — keep it behind Advanced so novices can't reach it.
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

  // Append-only check eligibility: only a domain whose off-site repo is set AND
  // flagged immutable gets the button — the same precondition the wizard's manual
  // test has (anything else could only ever surface a backend error).
  const appendOnlyEligible: Record<Domain, boolean> = {
    containers: settings.containersOffsite !== "" && settings.containersOffsiteImmutable,
    vms: settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable,
    flash: settings.flashOffsite !== "" && settings.flashOffsiteImmutable,
    files: settings.filesOffsite !== "" && settings.filesOffsiteImmutable,
  };

  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus-well";

  return (
    <Card title={t("integrity.title")} hint={t("integrity.hint")} hueIndex={hueIndex}>
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
        <SourceToggle
          source={source}
          onChange={(next) => {
            // The ok/fail indicators belong to the previously selected source —
            // clear them so a "healthy" result doesn't carry over to the other
            // repo where no maintenance has run yet. The drill state + cached
            // last-drill clear here too; the effect above reloads them for `next`.
            setSource(next);
            setState({});
            setLastDrill({});
          }}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* Drill-type toggle: subset integrity check vs a real off-site DR
          restore — on the shared Selector component (GlimStone form-engine
          Phase 2, Task 3; found only by re-grepping the current codebase,
          not on the original Phase 1 audit's own 11-site list).
          `variant="well"` with no `equalWidth` — the small scale of the app's
          one grooved selector (round 7 escalation) — converted alongside
          NotifyCard's "on" Selector in the same round; see that call site's
          own comment for the full root-cause writeup. Both are the same
          "small in-card single-choice" role at the same scale, so both share
          the literal same variant+props, not just an eyeballed match. */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("drill.kindLabel")}</span>
        <Selector
          items={[
            { id: "subset", label: t("drill.kindSubset") },
            { id: "dr", label: t("drill.kindDR") },
          ]}
          label={t("drill.kindLabel")}
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
            <select
              value={settings.drDrillTarget}
              onChange={(e) => {
                const v = e.target.value;
                setSettings((prev) => (prev ? { ...prev, drDrillTarget: v } : prev));
                void save({ drDrillTarget: v }, setTgtState, setTgtError);
              }}
              className={selectCls}
            >
              <option value="">{t("drill.targetMostRecent")}</option>
              {containers.map((c) => (
                <option key={c.name} value={c.name}>{c.name}</option>
              ))}
            </select>
          </label>
          {/* GlimStone follow-up pass (v8.0.0): the "saved"/"error" flash this
              used to render is gone — the shared save() now pushes a toast
              on both outcomes (see its own header comment). */}
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            {t("drill.targetVM")}
            <select
              value={settings.drDrillTargetVm}
              onChange={(e) => {
                const v = e.target.value;
                setSettings((prev) => (prev ? { ...prev, drDrillTargetVm: v } : prev));
                void save({ drDrillTargetVm: v }, setTgtVMState, setTgtVMError);
              }}
              className={selectCls}
            >
              <option value="">{t("drill.targetMostRecent")}</option>
              {vms.map((v) => (
                // value must be the raw libvirt name: pickDRSnapshot (service.go)
                // matches it against the "vm:"+name backup tag, never the
                // display-only friendly name a TrueNAS VM shows here.
                <option key={v.libvirtName} value={v.libvirtName}>{v.name}</option>
              ))}
            </select>
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
              {/* Domain actions + the restore-verification drill on ONE row
                  (jdp, live-review: "Die Buttons bei Container, VMs, etc
                  sollen alle in einer Zeile stehen") — was two separate flex
                  rows (verify/unlock/prune, then a second row for the drill/
                  DR-run button behind an empty w-24 spacer just to align
                  under the first row's buttons). The label alone already
                  anchors the whole row's left edge now that everything sits
                  on it, so that spacer is gone along with the second row;
                  every button below keeps its exact behavior, disabled state
                  and inline busy/ok/fail feedback — this is a pure layout
                  merge, no logic changed. */}
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
                      {/* Minimal fixed glyph, matching the "ok" indicator's OWN
                          visual weight — the actual error text now lives in the
                          toast the failure just pushed (jdp, live review,
                          emphatic standing rule: toast on failure, not a
                          permanent inline sentence; see run()'s own comment). */}
                      {state[k] === "fail" && <span className="text-sm text-statusFail">{t("integrity.failedShort")}</span>}
                    </span>
                  );
                })}
                <Button
                  key={shake[dKey] || 0}
                  label={kind === "dr" ? t("drill.runDR") : t("verify.now")}
                  // Both engines here too ([324]). The labelKey follows the
                  // same branch the label does — a stage derived from the
                  // wrong key would size this button for the other wording.
                  labelKey={kind === "dr" ? "drill.runDR" : "verify.now"}
                  glyph={<IconCheckCircle />}
                  tone="neutral"
                  hueIndex={hueIndex}
                  onClick={() => void runDrillFor(domain)}
                  disabled={state[dKey] === "busy"}
                  busy={state[dKey] === "busy"}
                  // The label is fixed now, so the RUNNING wording moves here,
                  // where Button already puts everything that changes; the
                  // standing hint is what it says the rest of the time.
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
                {/* Same minimal-glyph treatment as the action buttons above —
                    the raw error/detail text went to the toast when the
                    failure happened, not into a permanent inline sentence. */}
                {state[dKey] === "fail" && (
                  <span className="text-sm text-statusFail">✗ {t("verify.failed")}</span>
                )}
                {/* Last recorded drill for this domain/source (idle state only).
                    Names WHICH check ran (off-site DR vs local integrity) and,
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

              {/* Append-only check (#109): the wizard's tamper test, findable in
                  this card and led by the plainer name. Immutable off-site domains
                  only; always probes the OFF-SITE repo (source-independent). The
                  verdict rendering mirrors the wizard, incl. the glyph as its own
                  node so RTL locales place it correctly. */}
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
                      {!tRes.testable
                        ? t("offsite.tamperUnverifiable")
                        : tRes.protected
                          ? t("offsite.tamperOk")
                          : t("offsite.tamperFail")}
                    </span>
                  )}
                  {/* The test itself failing to run (network/server error) is the
                      "you clicked it, it didn't work" case — the toast pushed by
                      runTamperFor carries the real message now, so this stays a
                      fixed, short caption instead of the raw (possibly long)
                      backend text living on the page forever. A decisive
                      "not protected" VERDICT above is a different, legitimate
                      persistent status and is untouched. */}
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

// ---------------------------------------------------------------------------
// Schedule editors — migrated verbatim from the retired Plans page (Jobs.tsx).
// The Schedules tab is now the single owner of every backup/off-site/self-backup/
// restore-check cadence. These render their own Cards (same as on the old Plans
// page); behaviour is unchanged.
// ---------------------------------------------------------------------------

/** Convert a cadence string to a human-readable label. */
// cadenceLabel / ScheduleStatus / scheduleStatus / SCHEDULE_BADGE_TONE /
// ScheduleBadge all MOVED to components/ScheduleBadge.tsx this round, verbatim
// — plus a new `ScheduleRow` that owns the "Zeitplan: [badge]" line the four
// domain Cards and Selbst-Backup each used to hand-roll below. See that file's
// own header for why (CadenceBuilder's redundant inline preview could only be
// deleted once EVERY cadence editor was guaranteed to show its resolved
// schedule above, and ItemScheduleOverride needed the badge too — which an
// export from this file could not give it without an import cycle, since this
// file already imports ItemScheduleOverride).

// Domain section — Containers (editable schedule + included-containers list)
