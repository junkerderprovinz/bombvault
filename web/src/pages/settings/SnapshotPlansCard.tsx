// The schedules of the server's own snapshots. Every change is saved as it is
// made; a plan keeps what it takes snapshots of, so changing that is a new plan.
import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "../../components/Button";
import { CadenceBuilder, ALL_CADENCE_MODES } from "../../components/CadenceBuilder";
import { Note } from "../../components/entry/Note";
import { NumberField } from "../../components/NumberField";
import { cadenceLabel } from "../../components/ScheduleBadge";
import { SelectField } from "../../components/SelectField";
import { IconAdd, IconBackupNow, IconTrash } from "../../components/Sidebar";
import { Rows, SettingRow } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import {
  addHostSnapshotPlan,
  deleteHostSnapshotPlan,
  listHostSnapshotPlans,
  listHostSnapshots,
  openedAsPairedPhone,
  runHostSnapshotPlan,
  updateHostSnapshotPlan,
  type HostSnapPlan,
  type HostSnapPlanInput,
  type HostSnapSource,
} from "../../lib/api";
import { codeKey, KIND_LABEL, needsQcow2, sourceKey, sourceLabel } from "../../lib/hostSnapshots";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { InstanceWindow } from "../instances/InstanceWindow";
import { Card } from "./shared";

const KEEP_SAVE_MS = 600;

const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";
// A plan sits on its own sub-card, so its field and buttons take the next surface up.
const PLAN_FIELD = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";

function input(p: HostSnapPlan): HostSnapPlanInput {
  return {
    kind: p.kind,
    fs: p.fs,
    source: p.source,
    cadence: p.cadence,
    keep: p.keep,
    recursive: p.recursive,
    skipAsleep: p.skipAsleep,
    enabled: p.enabled,
  };
}

/** usePlans holds the plans as the server last answered. */
function usePlans() {
  const [plans, setPlans] = useState<HostSnapPlan[] | null>(null);
  useEffect(() => {
    if (openedAsPairedPhone()) return;
    let live = true;
    listHostSnapshotPlans()
      .then((res) => live && setPlans(res.ok ? (res.plans ?? []) : []))
      .catch(() => live && setPlans([]));
    return () => {
      live = false;
    };
  }, []);
  return { plans, setPlans };
}

/** useSources reads the host for what a new plan can take snapshots of, only
 *  once the window that asks for it opens, since a read reaches every pool. */
function useSources(): HostSnapSource[] | null {
  const [sources, setSources] = useState<HostSnapSource[] | null>(null);
  useEffect(() => {
    let live = true;
    listHostSnapshots()
      .then((res) => live && setSources(res.ok ? (res.sources ?? []) : []))
      .catch(() => live && setSources([]));
    return () => {
      live = false;
    };
  }, []);
  return sources;
}

function lastRunLine(p: HostSnapPlan, t: ReturnType<typeof useT>["t"]): string {
  if (p.lastRunAt === 0) return t("hostSnap.plans.never");
  const result = p.lastCode === "ok" ? t("hostSnap.plans.lastOk", Number(p.lastDetail) || 0) : t(codeKey(p.lastCode));
  return t("hostSnap.plans.lastRun").replace("{age}", relativeTime(t, p.lastRunAt)).replace("{result}", () => result);
}

function PlanRow({
  plan,
  onChange,
  onRemove,
}: {
  plan: HostSnapPlan;
  onChange: (next: HostSnapPlan) => void;
  onRemove: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const ownLogin = useOwnLogin();
  const [keep, setKeep] = useState(plan.keep);
  const [running, setRunning] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const label = `${t(KIND_LABEL[plan.kind])}: ${sourceLabel(plan.kind, plan.fs, plan.source, t)}`;

  async function save(next: HostSnapPlan) {
    try {
      const res = await updateHostSnapshotPlan(next.id, input(next));
      if (res.ok && res.plan) onChange(res.plan);
      else if (!ownLogin.handled(res)) push(t(codeKey(res.code)), "fail");
    } catch {
      push(t("common.saveFailed"), "fail");
    }
  }

  function changeKeep(n: number) {
    setKeep(n);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (n >= 1 && n <= 1000) void save({ ...plan, keep: n });
    }, KEEP_SAVE_MS);
  }

  async function runNow() {
    setRunning(true);
    try {
      const res = await runHostSnapshotPlan(plan.id);
      if (!res.ok) {
        if (!ownLogin.handled(res)) push(t(codeKey(res.code)), "fail");
      } else if (res.result?.code === "ok") {
        push(t("hostSnap.plans.ran").replace("{source}", () => label), "success");
      } else {
        push(t(codeKey(res.result?.code)), res.result?.code === "skipped-asleep" ? "warn" : "fail");
      }
      onChange({ ...plan, lastRunAt: Math.floor(Date.now() / 1000), lastCode: res.result?.code ?? plan.lastCode, lastDetail: String(res.result?.removed.length ?? 0) });
    } catch {
      push(t("hostSnap.code.failed"), "fail");
    } finally {
      setRunning(false);
    }
  }

  const summary = [plan.enabled ? cadenceLabel(plan.cadence, t) : t("jobs.notScheduled"), `${t("hostSnap.plans.keep")}: ${plan.keep}`].join(" · ");
  const toggle = (name: string, checked: boolean, change: (v: boolean) => void, hint?: string) => (
    <SettingRow label={name} hint={hint} narrow>
      <Toggle hideLabel label={name} checked={checked} onChange={change} />
    </SettingRow>
  );

  return (
    <li className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4">
      <div data-plan-head className="flex min-w-0 flex-col gap-0.5">
        <span dir="ltr" className="text-start text-sm font-semibold text-carbon-text wrap-anywhere">
          {label}
        </span>
        <span className="text-xs text-carbon-textMuted">{summary}</span>
      </div>
      <Rows>
        <div className="py-3 first:pt-0">
          <CadenceBuilder
            label={t("hostSnap.plans.when")}
            inline
            row
            value={plan.cadence}
            modes={ALL_CADENCE_MODES.filter((m) => m !== "off")}
            onChange={(cadence) => {
              if (cadence !== plan.cadence && cadence !== "off" && cadence !== "") void save({ ...plan, cadence });
            }}
          />
        </div>
        <SettingRow label={t("hostSnap.plans.keep")} narrow>
          <NumberField
            min={1}
            max={1000}
            value={keep}
            aria-label={t("hostSnap.plans.keep")}
            onChange={(e) => changeKeep(Number(e.target.value))}
            wrapperClassName="w-28"
            className={`w-full ${PLAN_FIELD}`}
          />
        </SettingRow>
        {plan.kind === "zfs" && toggle(t("hostSnap.create.recursive"), plan.recursive, (v) => void save({ ...plan, recursive: v }))}
        {plan.kind !== "vm" &&
          toggle(t("hostSnap.plans.skipAsleep"), plan.skipAsleep, (v) => void save({ ...plan, skipAsleep: v }), t("hostSnap.plans.skipAsleepHint"))}
        {toggle(t("hostSnap.plans.enabled"), plan.enabled, (v) => void save({ ...plan, enabled: v }))}
      </Rows>
      <div className="flex flex-wrap items-center justify-end gap-2">
        <span className="me-auto text-xs text-carbon-textMuted">{lastRunLine(plan, t)}</span>
        <Button
          label={t("hostSnap.plans.runNow")}
          labelKey="hostSnap.plans.runNow"
          glyph={<IconBackupNow />}
          tone="neutral"
          busy={running}
          disabled={running}
          onClick={() => void runNow()}
        />
        <Button label={t("hostSnap.plans.remove")} labelKey="hostSnap.plans.remove" glyph={<IconTrash />} tone="neutral" onClick={onRemove} />
      </div>
      {ownLogin.dialog}
    </li>
  );
}

function AddPlanWindow({
  onAdded,
  onClose,
}: {
  onAdded: (p: HostSnapPlan) => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const ownLogin = useOwnLogin();
  const sources = useSources();
  const options = useMemo(
    () =>
      (sources ?? [])
        .filter((s) => s.canCreate)
        .map((s) => ({ value: sourceKey(s.kind, s.fs, s.source), label: `${t(KIND_LABEL[s.kind])}: ${sourceLabel(s.kind, s.fs, s.source, t)}`, src: s })),
    [sources, t]
  );
  const [chosen, setPicked] = useState("");
  const picked = chosen || (options[0]?.value ?? "");
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const src = options.find((o) => o.value === picked)?.src;

  async function add() {
    if (!src) return;
    setBusy(true);
    setRefusal(null);
    try {
      const res = await addHostSnapshotPlan({
        kind: src.kind,
        fs: src.fs ?? "",
        source: src.source,
        cadence: "daily 03:00",
        keep: 7,
        recursive: false,
        skipAsleep: true,
        enabled: true,
      });
      if (res.ok && res.plan) {
        onAdded(res.plan);
        return;
      }
      if (!ownLogin.handled(res)) setRefusal(t(codeKey(res.code)));
    } catch {
      setRefusal(t("hostSnap.code.failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("hostSnap.plans.add")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("hostSnap.plans.add")}
            labelKey="hostSnap.plans.add"
            glyph={<IconAdd />}
            tone="accent"
            disabled={!src || busy}
            busy={busy}
            onClick={() => void add()}
          />
        </>
      }
    >
      {sources === null ? null : options.length === 0 ? (
        <Note>{t("hostSnap.plans.noSources")}</Note>
      ) : (
        <SettingRow label={t("hostSnap.create.source")}>
          <SelectField value={picked} onChange={setPicked} options={options} label={t("hostSnap.create.source")} wrap className={FIELD} />
        </SettingRow>
      )}
      <Note>{t("hostSnap.plans.addHint")}</Note>
      {sources && needsQcow2(sources).length > 0 && (
        <Note>{t("hostSnap.create.needsQcow2").replace("{names}", () => needsQcow2(sources).join(", "))}</Note>
      )}
      {refusal && (
        <div role="alert">
          <Note tone="fail">{refusal}</Note>
        </div>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}

/** SnapshotPlansCard lists the schedules, each with its cadence and keep. */
export function SnapshotPlansCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const { plans, setPlans } = usePlans();
  const [adding, setAdding] = useState(false);

  async function remove(p: HostSnapPlan) {
    const label = sourceLabel(p.kind, p.fs, p.source, t);
    if (!(await confirm(t("hostSnap.plans.removeConfirm").replace("{source}", () => label), { confirmKey: "hostSnap.plans.remove", confirmTone: "neutral", cancelTone: "accent" })))
      return;
    try {
      const res = await deleteHostSnapshotPlan(p.id);
      if (res.ok) setPlans((prev) => (prev ?? []).filter((x) => x.id !== p.id));
      else if (!ownLogin.handled(res)) push(t(codeKey(res.code)), "fail");
    } catch {
      push(t("hostSnap.code.failed"), "fail");
    }
  }

  if (openedAsPairedPhone()) {
    return (
      <Card title={t("hostSnap.plans.title")} hint={t("hostSnap.plans.hint")} hueIndex={hueIndex} newId="settings-snapshots">
        <Note>{t("hostSnap.pairedOnly")}</Note>
      </Card>
    );
  }

  return (
    <Card title={t("hostSnap.plans.title")} hint={t("hostSnap.plans.hint")} hueIndex={hueIndex} newId="settings-snapshots">
      {plans !== null && plans.length === 0 && <p className="text-xs text-carbon-textMuted">{t("hostSnap.plans.none")}</p>}
      {plans !== null && plans.length > 0 && (
        <ul aria-label={t("hostSnap.plans.title")} className="m-0 flex list-none flex-col gap-4 p-0">
          {plans.map((p) => (
            <PlanRow
              key={p.id}
              plan={p}
              onChange={(next) => setPlans((prev) => (prev ?? []).map((x) => (x.id === next.id ? next : x)))}
              onRemove={() => void remove(p)}
            />
          ))}
        </ul>
      )}
      <div className="flex justify-end">
        <Button label={t("hostSnap.plans.add")} labelKey="hostSnap.plans.add" glyph={<IconAdd />} tone="subtle" onClick={() => setAdding(true)} />
      </div>
      {adding && (
        <AddPlanWindow
          onClose={() => setAdding(false)}
          onAdded={(p) => {
            setAdding(false);
            setPlans((prev) => [...(prev ?? []), p]);
          }}
        />
      )}
      {confirmDialog}
      {ownLogin.dialog}
    </Card>
  );
}
