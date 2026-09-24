import { useState } from "react";
import { Card, ToggleRow } from "./shared";
import { Button } from "../../components/Button";
import { HUE_OFFSET } from "../../components/Selector";
import { DirectRepoDialog } from "../../components/placement/DirectRepoDialog";
import { itemName, NewTargetPreviewLines } from "../../components/placement/NewTargetQuestion";
import { PlacementBar } from "../../components/placement/PlacementBar";
import { UploadLines } from "../../components/placement/PlacementRow";
import {
  applyPlacementDefault,
  confirmPlacementDefault,
  getApplyDefaultPreview,
  getConfirmPreview,
  previewPlacementDefault,
  putPlacementDefault,
  type DefaultChange,
  type DefaultImpact,
  type DefaultRow,
  type OkEnvelope,
  type PlacementOptions,
  type SegmentId,
  type SendToOption,
  type TargetPreviewRow,
  type UnmatchedName,
  type UploadEstimate,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { defaultView, domainLabel, formatList, stepForChip, stepForDefaultSegment, viewHomeLabel } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useHostLabel } from "../../lib/useHostLabel";
import { useNamedRepos } from "../../lib/useNamedRepos";
import { usePlacementDefaults } from "../../lib/usePlacementDefaults";
import { usePlacementOptions } from "../../lib/usePlacementOptions";

type T = ReturnType<typeof useT>["t"];

const COUNT_KEYS: [keyof DefaultRow["counts"], TranslationKey][] = [
  ["follow", "placementDefaults.countFollow"],
  ["own", "placementDefaults.countOwn"],
  ["open", "placementDefaults.countOpen"],
  ["chosenNoRun", "placementDefaults.countChosenNoRun"],
];

function impactLines(t: T, lang: string, impact: DefaultImpact, home: string): string[] {
  const uncheckable = (names: string[]) => t("placement.uncheckable").replace("{list}", () => formatList(lang, names));
  const lines: string[] = [];
  for (const d of impact.dropped) {
    lines.push(
      d.unknown
        ? t("placementDefaults.dropAskUnknown").replace(/\{target\}/g, () => d.name).replace("{n}", String(d.items))
        : t("placementDefaults.dropAsk")
            .replace("{target}", () => d.name)
            .replace("{n}", String(d.items))
            .replace("{copies}", String(d.snapshots))
    );
    if (d.uncheckable.length > 0) lines.push(uncheckable(d.uncheckable));
  }
  for (const a of impact.added) {
    lines.push(
      t("placementDefaults.addAsk")
        .replace("{target}", () => a.name)
        .replace("{n}", String(a.items))
        .replace("{snapshots}", String(a.snapshots))
    );
    if (a.uncheckable.length > 0) lines.push(uncheckable(a.uncheckable));
  }
  if (impact.openTakeHome > 0) {
    lines.push(t("placementDefaults.openTakeHome").replace("{home}", () => home).replace("{n}", String(impact.openTakeHome)));
  }
  return lines;
}

// sumUploads adds up per target what all candidates would upload.
function sumUploads(uploads: UploadEstimate[]): UploadEstimate[] {
  const byTarget = new Map<string, UploadEstimate>();
  for (const u of uploads) {
    const seen = byTarget.get(u.targetId);
    byTarget.set(
      u.targetId,
      seen
        ? { ...seen, snapshots: seen.snapshots + u.snapshots, uncheckable: [...new Set([...seen.uncheckable, ...u.uncheckable])] }
        : u
    );
  }
  return [...byTarget.values()];
}

function ConfirmLines({
  targets,
  unmatched,
  onExclude,
}: {
  targets: TargetPreviewRow[];
  unmatched: UnmatchedName[];
  onExclude: (identities: string[]) => void;
}) {
  const { t } = useT();
  const [excluded, setExcluded] = useState<string[]>([]);
  function toggle(identity: string, on: boolean) {
    const next = on ? [...excluded, identity] : excluded.filter((x) => x !== identity);
    setExcluded(next);
    onExclude(next);
  }
  return (
    <div className="flex flex-col gap-3">
      {targets.map((row) => (
        <div key={row.targetId} className="flex flex-col gap-1">
          <p className="text-sm text-carbon-text">{row.name}</p>
          <NewTargetPreviewLines target={row.name} preview={row.preview} />
        </div>
      ))}
      {unmatched.length > 0 && (
        <div className="flex flex-col gap-1">
          <p className="text-sm text-carbon-textSub">{t("placementDefaults.unmatched")}</p>
          {unmatched.map((u) => (
            <ToggleRow
              key={u.identity}
              label={t("placementDefaults.unmatchedLine")
                .replace("{name}", () => itemName(u.identity))
                .replace("{n}", String(u.snapshots))}
              checked={excluded.includes(u.identity)}
              onChange={(on) => toggle(u.identity, on)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function DefaultEditor({ row, options, hueOffset }: { row: DefaultRow; options: PlacementOptions; hueOffset: number }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const host = useHostLabel();
  const repos = useNamedRepos();
  const { confirm, confirmDialog } = useConfirm();
  const [direct, setDirect] = useState<SendToOption | null>(null);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const domain = domainLabel(t, row.domain);
  const repoName = (id: string) => repos.find((r) => r.id === id)?.name ?? id;
  const view = { ...defaultView(row, options), repoLabel: repoName(row.home) };
  const homeName = (id: string) => viewHomeLabel(t, host, { ...view, repo: id, repoLabel: repoName(id), repoOff: false }, options);

  function fail(res: OkEnvelope) {
    push(placementErrorText(t, lang, res, "settings.error"), "fail");
    setShake((n) => n + 1);
  }

  // agreed asks when a change moves the location or changes what a target
  // receives. A change that does neither is saved without a question.
  async function agreed(impact: DefaultImpact, home: string | undefined): Promise<boolean> {
    const lines = impactLines(t, lang, impact, homeName(home ?? row.home));
    const lead =
      home !== undefined
        ? t("placementDefaults.confirmHome").replace("{domain}", () => domain).replace("{home}", () => homeName(home))
        : lines.shift();
    if (lead === undefined) return true;
    const extra =
      lines.length > 0 ? (
        <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
          {lines.map((line, i) => (
            <p key={i}>{line}</p>
          ))}
        </div>
      ) : undefined;
    return confirm(
      lead,
      home !== undefined ? { extra, confirmLabel: t("placement.saveHome"), confirmLabelKey: "placement.saveHome" } : { extra }
    );
  }

  async function change(next: DefaultChange) {
    const home = next.home !== undefined && next.home !== row.home ? next.home : undefined;
    setBusy(true);
    try {
      const preview = await previewPlacementDefault(row.domain, next);
      if (!preview.ok || !preview.impact) {
        fail(preview);
        return;
      }
      let impact = preview.impact;
      for (;;) {
        if (!(await agreed(impact, home))) return;
        const res = await putPlacementDefault(row.domain, next, impact);
        if (res.ok) {
          placementChanged();
          return;
        }
        if (res.code !== "stale" || !res.impact) {
          fail(res);
          return;
        }
        impact = res.impact;
      }
    } finally {
      setBusy(false);
    }
  }

  async function apply() {
    setBusy(true);
    try {
      const preview = await getApplyDefaultPreview(row.domain);
      if (!preview.ok) {
        fail(preview);
        return;
      }
      const reset = preview.reset ?? [];
      const kept = preview.kept ?? [];
      if (reset.length === 0) {
        push(t("placementDefaults.applyNone"), "success");
        return;
      }
      const names = (list: { label: string }[]) => formatList(lang, list.map((x) => x.label));
      const losing = reset.filter((c) => c.losesHome || c.losesRule);
      const keptBackups = kept.filter((k) => k.reason === "has-backups");
      const keptUnreadable = kept.filter((k) => k.reason === "unreadable");
      const uploads = sumUploads(reset.flatMap((c) => c.uploads));
      const extra = (
        <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
          {losing.length > 0 && <p>{t("placementDefaults.applyLoses").replace("{list}", () => names(losing))}</p>}
          {keptBackups.length > 0 && <p>{t("placementDefaults.applyKeptBackups").replace("{list}", () => names(keptBackups))}</p>}
          {keptUnreadable.length > 0 && (
            <p>{t("placementDefaults.applyKeptUnreadable").replace("{list}", () => names(keptUnreadable))}</p>
          )}
          {uploads.length > 0 && <UploadLines added={uploads} />}
        </div>
      );
      if (!(await confirm(t("placementDefaults.applyAsk").replace("{n}", String(reset.length)), { extra }))) return;
      const res = await applyPlacementDefault(row.domain, reset.map((c) => c.key));
      if (!res.ok) {
        push(res.code === "domain-busy" ? t("placementDefaults.applyBusy") : placementErrorText(t, lang, res, "settings.error"), "fail");
        setShake((n) => n + 1);
        return;
      }
      const changed = (res.kept ?? []).filter((k) => k.reason === "changed");
      if (changed.length > 0) push(t("placementDefaults.applyKeptChanged").replace("{list}", () => names(changed)), "warn");
      placementChanged();
    } finally {
      setBusy(false);
    }
  }

  async function confirmDefault() {
    setBusy(true);
    try {
      const preview = await getConfirmPreview(row.domain);
      if (!preview.ok) {
        fail(preview);
        return;
      }
      let excluded: string[] = [];
      const extra = (
        <ConfirmLines targets={preview.targets ?? []} unmatched={preview.unmatched ?? []} onExclude={(ids) => (excluded = ids)} />
      );
      const go = await confirm(t("placementDefaults.confirmAsk"), {
        extra,
        confirmLabel: t("placementDefaults.confirm"),
        confirmLabelKey: "placementDefaults.confirm",
      });
      if (!go) return;
      const res = await confirmPlacementDefault(row.domain, excluded);
      if (!res.ok) {
        fail(res);
        return;
      }
      push(t("placementDefaults.confirmed"), "success");
      placementChanged();
    } finally {
      setBusy(false);
    }
  }

  function onSegment(seg: SegmentId) {
    const step = stepForDefaultSegment(seg, row, options);
    if (step.kind === "direct") setDirect(step.target);
    else if (step.kind === "change") void change(step.change);
  }

  function onSendTo(opt: SendToOption) {
    if (!opt.repoId) setDirect(opt);
    else if (opt.repoId !== row.home) void change({ home: opt.repoId });
  }

  function onChip(targetId: string, on: boolean) {
    const step = stepForChip(targetId, on, view, options);
    if (step.kind === "save" && step.change.copies && "skip" in step.change.copies) void change({ skip: step.change.copies.skip });
  }

  const counts = COUNT_KEYS.filter(([k]) => row.counts[k] > 0)
    .map(([k, key]) => t(key).replace("{n}", String(row.counts[k])))
    .join(" · ");

  return (
    <>
      {confirmDialog}
      {row.paused && (
        <div className="flex flex-col gap-0.5">
          <p className="text-xs text-statusWarn">{t("placementDefaults.paused")}</p>
          <p className="text-xs text-carbon-textMuted">{t("placementDefaults.pausedHint").replace("{domain}", () => domain)}</p>
        </div>
      )}
      {row.homeOff && (
        <p className="text-xs text-statusWarn">{t("placementDefaults.homeOff").replace("{home}", () => view.repoLabel)}</p>
      )}
      {row.homeKind === "missing" && <p className="text-xs text-statusWarn">{t("placementDefaults.homeMissing")}</p>}
      <div key={shake} className={shake ? "glim-shake" : undefined}>
        <PlacementBar
          domain={row.domain}
          context="default"
          view={view}
          options={options}
          host={host}
          hueOffset={hueOffset}
          disabled={busy}
          onSegment={onSegment}
          onHome={(id) => {
            if (id !== row.home) void change({ home: id });
          }}
          onSendTo={onSendTo}
          onChip={onChip}
        />
      </div>
      {counts !== "" && <p className="text-xs text-carbon-textMuted">{counts}</p>}
      <div className="flex items-center gap-2 flex-wrap">
        <Button
          label={t("placementDefaults.apply")}
          labelKey="placementDefaults.apply"
          tone="neutral"
          onClick={() => void apply()}
          disabled={busy}
          className="glim-btn-elastic"
        />
        {row.paused && (
          <Button
            label={t("placementDefaults.confirm")}
            labelKey="placementDefaults.confirm"
            tone="accent"
            onClick={() => void confirmDefault()}
            disabled={busy}
          />
        )}
      </div>
      {direct && (
        <DirectRepoDialog
          target={{ id: direct.targetId, name: direct.name }}
          mode="create"
          onClose={() => setDirect(null)}
          onDone={(result) => {
            setDirect(null);
            if (result.kind === "created") void change({ home: result.repo.id });
          }}
        />
      )}
    </>
  );
}

function DefaultLine({ row, hueOffset }: { row: DefaultRow; hueOffset: number }) {
  const { t } = useT();
  const { options } = usePlacementOptions(row.domain);
  const domain = domainLabel(t, row.domain);
  const unreadable = row.unreadable || options?.unreadable === true;
  return (
    <section aria-label={domain} className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
      <span className="text-sm font-semibold text-carbon-text">{domain}</span>
      {unreadable && <p className="text-xs text-statusWarn">{t("placement.unreadable")}</p>}
      {!unreadable && options && <DefaultEditor row={row} options={options} hueOffset={hueOffset} />}
    </section>
  );
}

/** PlacementDefaultsCard shows each domain's default: where a new item goes at
 *  its first backup, and where items without their own choice are copied. */
export function PlacementDefaultsCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const { rows, error } = usePlacementDefaults();
  return (
    <Card title={t("placementDefaults.title")} hint={t("placementDefaults.hint")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-3">
        {error !== null && <p className="text-xs text-statusWarn">{t("placement.unreadable")}</p>}
        {rows?.map((row, i) => (
          <DefaultLine key={row.domain} row={row} hueOffset={HUE_OFFSET.placement + 3 * i} />
        ))}
      </div>
    </Card>
  );
}
