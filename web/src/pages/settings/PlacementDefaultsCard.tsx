import { useState } from "react";
import { Link } from "react-router-dom";
import { Card, ToggleRow } from "./shared";
import { Button } from "../../components/Button";
import { HUE_OFFSET } from "../../components/Selector";
import { itemName, NewTargetPreviewLines } from "../../components/placement/NewTargetQuestion";
import { PlacementBar } from "../../components/placement/PlacementBar";
import { UploadLines } from "../../components/placement/PlacementRow";
import { useDefaultWrites } from "../../components/placement/useDefaultWrites";
import {
  applyPlacementDefault,
  confirmPlacementDefault,
  getApplyDefaultPreview,
  getConfirmPreview,
  type DefaultRow,
  type PlacementDomain,
  type PlacementOptions,
  type TargetPreviewRow,
  type UnmatchedName,
  type UploadEstimate,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { domainLabel, formatList, stepForLocal, stepForTarget } from "../../lib/placement";
import { useNewDestinations } from "../../lib/placementButtons";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { usePlacementDefaults } from "../../lib/usePlacementDefaults";
import { usePlacementOptions } from "../../lib/usePlacementOptions";

const COUNT_KEYS: [keyof DefaultRow["counts"], TranslationKey][] = [
  ["follow", "placementDefaults.countFollow"],
  ["own", "placementDefaults.countOwn"],
  ["open", "placementDefaults.countOpen"],
  ["chosenNoRun", "placementDefaults.countChosenNoRun"],
];

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
  domain,
  targets,
  unmatched,
  onExclude,
}: {
  domain: PlacementDomain;
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
          <NewTargetPreviewLines domain={domain} target={row.name} preview={row.preview} />
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
  const writes = useDefaultWrites();
  const { host, busy, shake, confirm, fail, guarded } = writes;
  const destinations = useNewDestinations(row.domain, options);
  const domain = domainLabel(t, row.domain);
  const ctx = writes.context(row, options);
  const { view } = ctx;

  function apply() {
    return guarded(async () => {
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
      if (!(await confirm(t("placementDefaults.applyAsk").replace("{n}", String(reset.length)), { extra, cancelTone: "neutral" }))) return;
      const res = await applyPlacementDefault(row.domain, reset.map((c) => c.key));
      if (!res.ok) {
        fail(res, res.code === "domain-busy" ? t("placementDefaults.applyBusy") : undefined);
        return;
      }
      const changed = (res.kept ?? []).filter((k) => k.reason === "changed");
      if (changed.length > 0) push(t("placementDefaults.applyKeptChanged").replace("{list}", () => names(changed)), "warn");
      placementChanged();
    });
  }

  function confirmDefault() {
    return guarded(async () => {
      const preview = await getConfirmPreview(row.domain);
      if (!preview.ok) {
        fail(preview);
        return;
      }
      let excluded: string[] = [];
      const extra = (
        <ConfirmLines domain={row.domain} targets={preview.targets ?? []} unmatched={preview.unmatched ?? []} onExclude={(ids) => (excluded = ids)} />
      );
      const go = await confirm(t("placementDefaults.confirmAsk"), {
        extra,
        confirmLabel: t("placementDefaults.confirm"),
        confirmLabelKey: "placementDefaults.confirm",
        cancelTone: "neutral",
      });
      if (!go) return;
      const res = await confirmPlacementDefault(row.domain, excluded);
      if (!res.ok) {
        fail(res);
        return;
      }
      push(t("placementDefaults.confirmed"), "success");
      placementChanged();
    });
  }

  const counts = COUNT_KEYS.filter(([k]) => row.counts[k] > 0)
    .map(([k, key]) => t(key).replace("{n}", String(row.counts[k])))
    .join(" · ");

  return (
    <>
      {writes.dialogs}
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
          destinations={destinations}
          onLocal={(on) => writes.run(ctx, stepForLocal(on, view, options, t, host))}
          onTarget={(id, on) => writes.run(ctx, stepForTarget(id, on, view, options, t))}
          onDestination={(id) => void writes.addDestination(ctx, id)}
          onHome={(id) => {
            if (id !== row.home) void writes.change(ctx, { home: id });
          }}
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
          className="glim-btn-elastic glim-btn-wrap"
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
        <Link
          to="/settings/storage#offsite"
          className="w-fit text-sm text-accentText hover:underline pointer-coarse:inline-flex pointer-coarse:min-h-(--btn-h) pointer-coarse:items-center"
        >
          {t("placementDefaults.othersOffsite")}
        </Link>
      </div>
    </Card>
  );
}
