// Adding a node takes three steps: its name, which makes its APP_KEY, where and
// when its backups are to go, and the compose file that starts it. The window then
// waits until the node answers, and only then sends it where its backups go,
// since a node takes that from the home it has adopted.

import { useEffect, useState } from "react";
import { addNode, listNodes, placeNode, setNodeSchedule, type NodeCompose, type NodeView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { secretsSentence } from "../../lib/secretsRefusal";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { BusyRing } from "../../components/BusyRing";
import { Button } from "../../components/Button";
import { CheckDraw } from "../../components/CheckDraw";
import { StepBar } from "../../components/restore/StepBar";
import { Rows, SettingRow } from "../../components/storage/rows";
import { ComposeView } from "./NodeCompose";
import { DEFAULT_NODE_SCHEDULE, NodeSchedule } from "./NodeSchedule";
import { InstanceWindow } from "./InstanceWindow";
import {
  PlacementFields,
  firstChoice,
  placedSentence,
  placementOf,
  placementReady,
  placementTarget,
  reachFailures,
  saveAddress,
  usePlacementSources,
  type PlacementChoice,
} from "./NodePlacement";

const INPUT = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** How often the window asks whether the node has come online. */
export const ADOPT_POLL_MS = 3000;

type Placed = { state: "waiting" } | { state: "sending" } | { state: "done"; failures: string[] } | { state: "failed"; error: string };

export function AddNodeWindow({ onClose, onChanged }: { onClose: () => void; onChanged: () => void }) {
  const { t } = useT();
  const ownLogin = useOwnLogin();
  const sources = usePlacementSources();
  const [step, setStep] = useState(1);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [created, setCreated] = useState<{ node: NodeView; compose: NodeCompose } | null>(null);
  const [choice, setChoice] = useState<PlacementChoice | null>(null);
  const [adopted, setAdopted] = useState(false);
  const [placed, setPlaced] = useState<Placed>({ state: "waiting" });
  const [schedule, setSchedule] = useState(DEFAULT_NODE_SCHEDULE);

  useEffect(() => {
    if (sources && !choice) setChoice(firstChoice(sources));
  }, [sources, choice]);

  // Nothing but the node starting tells the window it is there, so it asks.
  const nodeId = created?.node.id;
  useEffect(() => {
    if (step !== 3 || !nodeId || adopted) return;
    const timer = window.setInterval(() => {
      listNodes()
        .then((res) => {
          if (res.nodes?.some((n) => n.id === nodeId && n.adoptedAt > 0)) setAdopted(true);
        })
        .catch(() => undefined);
    }, ADOPT_POLL_MS);
    return () => window.clearInterval(timer);
  }, [step, nodeId, adopted]);

  const placeable = !!sources && !!choice && placementReady(choice, sources);
  const nowhere = !!sources && !sources.receiver && sources.locations.length === 0;

  useEffect(() => {
    if (!adopted || !nodeId || !choice || !placeable || placed.state !== "waiting") return;
    setPlaced({ state: "sending" });
    onChanged();
    placeNode(nodeId, placementOf(choice))
      .then((res) => setPlaced(res.ok ? { state: "done", failures: reachFailures(t, res.reach) } : { state: "failed", error: res.error ?? t("settings.error") }))
      .catch((err: unknown) => setPlaced({ state: "failed", error: err instanceof Error ? err.message : t("settings.error") }))
      .finally(onChanged);
  }, [adopted, nodeId, choice, placeable, placed.state, onChanged, t]);

  async function create() {
    if (created) {
      setStep(2);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await addNode(name.trim());
      if (res.node && res.compose) {
        setCreated({ node: res.node, compose: res.compose });
        onChanged();
        setStep(2);
        return;
      }
      if (ownLogin.handled(res)) return;
      setError(secretsSentence(t, res.code) ?? res.error ?? t("settings.error"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  async function confirmPlacement() {
    if (!choice || !sources || !created) return;
    setBusy(true);
    setError(null);
    try {
      const saved = await setNodeSchedule(created.node.id, schedule).catch((err: unknown) => ({
        ok: false as const,
        error: err instanceof Error ? err.message : undefined,
      }));
      let refused = saved.ok ? null : (saved.error ?? t("settings.error"));
      if (!refused && placeable) refused = await saveAddress(choice, sources, t);
      if (refused) setError(refused);
      else setStep(3);
    } finally {
      setBusy(false);
    }
  }

  const go = (next: number) => {
    setError(null);
    setStep(next);
  };
  const target = choice && sources && placeable ? placementTarget(choice, sources) : "";
  const nodeName = created?.node.name ?? name.trim();

  const footer =
    step === 1 ? (
      <>
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={busy} />
        <Button label={t("nodes.next")} labelKey="nodes.next" tone="accent" onClick={() => void create()} busy={busy} disabled={busy || name.trim() === ""} />
      </>
    ) : step === 2 ? (
      <>
        <Button label={t("common.back")} labelKey="common.back" tone="neutral" onClick={() => go(1)} disabled={busy} />
        <Button
          label={t("nodes.next")}
          labelKey="nodes.next"
          tone="accent"
          onClick={() => void confirmPlacement()}
          busy={busy}
          disabled={busy || (!placeable && !nowhere)}
        />
      </>
    ) : (
      <>
        <Button label={t("common.back")} labelKey="common.back" tone="neutral" onClick={() => go(2)} disabled={adopted} />
        <Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onClose} />
      </>
    );

  return (
    <InstanceWindow title={t("nodes.add")} hint={t("nodes.tip")} onClose={onClose} footer={footer}>
      <StepBar
        steps={[t("nodes.step.name"), t("nodes.step.placement"), t("nodes.step.compose")]}
        current={step}
        reach={created ? (step === 3 || placeable || nowhere ? 3 : 2) : 1}
        label={t("nodes.steps")}
        onGo={(next) => (next === 3 ? void confirmPlacement() : go(next))}
      />

      {step === 1 && (
        <>
          <p className="text-sm text-carbon-textSub">{t("nodes.nameIntro")}</p>
          <Rows>
            <SettingRow label={t("nodes.step.name")} hint={t("nodes.nameTip")} htmlFor="bv-node-name">
              <input
                id="bv-node-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && name.trim() !== "" && !busy) void create();
                }}
                placeholder="vps-falkenstein"
                maxLength={64}
                disabled={!!created}
                spellCheck={false}
                autoComplete="off"
                autoFocus
                className={`${INPUT} w-72 max-w-full`}
              />
            </SettingRow>
          </Rows>
        </>
      )}

      {step === 2 && sources && choice && <PlacementFields choice={choice} sources={sources} onChange={setChoice} />}
      {step === 2 && sources && <NodeSchedule value={schedule} onChange={(section, cadence) => setSchedule({ ...schedule, [section]: cadence })} />}
      {step === 2 && !sources && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}

      {step === 3 && created && (
        <>
          <ComposeView values={created.compose} />
          <div role="status" data-testid="node-adoption" className="flex flex-col gap-1.5 rounded-card bg-carbon-surface2 p-3">
            {adopted ? (
              <span className="inline-flex items-center gap-1.5 text-sm text-statusOk">
                <CheckDraw />
                {t("nodes.online").replace("{name}", () => nodeName)}
              </span>
            ) : (
              <span className="inline-flex items-center gap-2 text-sm text-carbon-textSub">
                <BusyRing />
                {t("nodes.waitingFor").replace("{name}", () => nodeName)}
              </span>
            )}
            {target && placed.state === "waiting" && (
              <span className="text-xs text-carbon-textMuted">{choice && placedSentence(t, choice, target, true)}</span>
            )}
            {!target && <span className="text-xs text-carbon-textMuted">{t("nodes.placementWaits")}</span>}
            {placed.state === "sending" && (
              <span className="inline-flex items-center gap-2 text-xs text-carbon-textSub">
                <BusyRing />
                {t("nodes.placing")}
              </span>
            )}
            {placed.state === "done" && placed.failures.length === 0 && (
              <span className="inline-flex items-center gap-1.5 text-xs text-statusOk">
                <CheckDraw />
                {choice && placedSentence(t, choice, target)}
              </span>
            )}
            {placed.state === "done" &&
              placed.failures.map((failure) => (
                <span key={failure} className="text-xs text-statusWarn wrap-break-word">
                  {failure}
                </span>
              ))}
            {placed.state === "failed" && <span className="text-xs text-statusFail wrap-break-word">{placed.error}</span>}
          </div>
        </>
      )}

      {error && (
        <p role="alert" className="text-sm text-statusFail wrap-break-word">
          {error}
        </p>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}
