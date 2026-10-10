import { useState, type ReactNode } from "react";
import {
  previewPlacementDefault,
  putPlacementDefault,
  type DefaultChange,
  type DefaultImpact,
  type DefaultRow,
  type OkEnvelope,
  type PlacementDomain,
  type PlacementOptions,
  type PlacementView,
  type SendToOption,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import {
  defaultChangeOf,
  defaultView,
  domainLabel,
  formatList,
  newTargetRefused,
  stepForNewTarget,
  viewHomeLabel,
  type PlacementStep,
} from "../../lib/placement";
import { blockedText, tickDestination } from "../../lib/placementButtons";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { useConfirm, type ConfirmOptions } from "../../lib/useConfirm";
import { useHostLabel } from "../../lib/useHostLabel";
import { useNamedRepos } from "../../lib/useNamedRepos";
import { DirectRepoDialog } from "./DirectRepoDialog";

type T = ReturnType<typeof useT>["t"];

/** One domain's default with what its buttons offer. */
export interface DefaultContext {
  row: DefaultRow;
  options: PlacementOptions;
  view: PlacementView;
  /** A repository id as the lists name it. */
  homeName: (repoId: string) => string;
}

// Only containers have project folders, so only their questions name them.
function impactLines(t: T, lang: string, domain: PlacementDomain, impact: DefaultImpact, home: string): string[] {
  const containers = domain === "containers";
  const uncheckable = (names: string[]) => t("placement.uncheckable").replace("{list}", () => formatList(lang, names));
  const lines: string[] = [];
  for (const d of impact.dropped) {
    lines.push(
      d.unknown
        ? t(containers ? "placementDefaults.dropAskUnknownContainers" : "placementDefaults.dropAskUnknown").replace(/\{target\}/g, () => d.name).replace("{n}", String(d.items))
        : t(containers ? "placementDefaults.dropAskContainers" : "placementDefaults.dropAsk")
            .replace("{target}", () => d.name)
            .replace("{n}", String(d.items))
            .replace("{copies}", String(d.snapshots))
    );
    if (d.uncheckable.length > 0) lines.push(uncheckable(d.uncheckable));
  }
  for (const a of impact.added) {
    lines.push(
      t(containers ? "placementDefaults.addAskContainers" : "placementDefaults.addAsk")
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

export interface DefaultWrites {
  host: string;
  context: (row: DefaultRow, options: PlacementOptions) => DefaultContext;
  busy: boolean;
  /** Counts the writes that were refused, for a shake that replays. */
  shake: number;
  /** The question and the window a write can open. Rendered once by the caller. */
  dialogs: ReactNode;
  confirm: (message: string, options?: ConfirmOptions) => Promise<boolean>;
  /** Reports a refused write, in `text` where the server's code has a sentence of the caller's. */
  fail: (res: OkEnvelope, text?: string) => void;
  /** Runs a write with the buttons held, and reports a server that did not answer. */
  guarded: (work: () => Promise<void>) => Promise<void>;
  change: (ctx: DefaultContext, next: DefaultChange) => Promise<void>;
  run: (ctx: DefaultContext, step: PlacementStep) => void;
  addDestination: (ctx: DefaultContext, destinationId: string) => Promise<void>;
}

/**
 * useDefaultWrites changes a domain's default the way every surface has to: it
 * asks the server what the change means, puts that to the person where it
 * moves the location or changes what a target receives, and writes with the
 * numbers that were shown.
 */
export function useDefaultWrites(): DefaultWrites {
  const { t, lang } = useT();
  const { push } = useToast();
  const host = useHostLabel();
  const repos = useNamedRepos();
  const { confirm, confirmDialog } = useConfirm();
  const [direct, setDirect] = useState<{ ctx: DefaultContext; target: SendToOption; skip: string[] } | null>(null);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  function context(row: DefaultRow, options: PlacementOptions): DefaultContext {
    const labelOf = (id: string) => {
      const r = repos.find((x) => x.id === id);
      return { repoLabel: r?.name ?? id, repoDirectOf: r?.directOf };
    };
    const view = { ...defaultView(row, options), ...labelOf(row.home) };
    return {
      row,
      options,
      view,
      homeName: (id) => viewHomeLabel(t, host, { ...view, repo: id, ...labelOf(id), repoOff: false }, options),
    };
  }

  function fail(res: OkEnvelope, text = placementErrorText(t, lang, res, "settings.error")) {
    push(text, "fail");
    setShake((n) => n + 1);
  }

  async function guarded(work: () => Promise<void>) {
    setBusy(true);
    try {
      await work();
    } catch (err) {
      fail({ ok: false, error: err instanceof Error ? err.message : undefined });
    } finally {
      setBusy(false);
    }
  }

  // A change that neither moves the location nor changes what a target
  // receives is saved without a question.
  async function agreed(ctx: DefaultContext, impact: DefaultImpact, home: string | undefined): Promise<boolean> {
    const { row } = ctx;
    const domain = domainLabel(t, row.domain);
    const lines = impactLines(t, lang, row.domain, impact, ctx.homeName(home ?? row.home));
    const lead =
      home !== undefined
        ? t("placementDefaults.confirmHome").replace("{domain}", () => domain).replace("{home}", () => ctx.homeName(home))
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
      home !== undefined
        ? { extra, confirmLabel: t("placement.saveHome"), confirmLabelKey: "placement.saveHome", cancelTone: "neutral" }
        : { extra, cancelTone: "neutral" }
    );
  }

  function change(ctx: DefaultContext, next: DefaultChange) {
    const { row } = ctx;
    const home = next.home !== undefined && next.home !== row.home ? next.home : undefined;
    return guarded(async () => {
      const preview = await previewPlacementDefault(row.domain, next);
      if (!preview.ok || !preview.impact) {
        fail(preview);
        return;
      }
      let impact = preview.impact;
      for (;;) {
        if (!(await agreed(ctx, impact, home))) return;
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
    });
  }

  function run(ctx: DefaultContext, step: PlacementStep) {
    if (step.kind === "blocked") {
      push(blockedText(t, step, viewHomeLabel(t, host, ctx.view, ctx.options)), "warn");
      setShake((n) => n + 1);
      return;
    }
    if (step.kind === "direct") {
      setDirect({ ctx, target: step.target, skip: step.skip ?? ["*"] });
      return;
    }
    const next = defaultChangeOf(step);
    if (next) void change(ctx, next);
  }

  async function addDestination(ctx: DefaultContext, destinationId: string) {
    const refused = newTargetRefused(ctx.view);
    if (refused) return run(ctx, refused);
    setBusy(true);
    const made = await tickDestination(destinationId, ctx.row.domain);
    setBusy(false);
    if ("error" in made) {
      fail(made.error);
      return;
    }
    run(ctx, stepForNewTarget(made.targetId, ctx.view, ctx.options));
  }

  const dialogs = (
    <>
      {confirmDialog}
      {direct && (
        <DirectRepoDialog
          target={{ id: direct.target.targetId, name: direct.target.name }}
          mode="create"
          onClose={() => setDirect(null)}
          onDone={(result) => {
            setDirect(null);
            if (result.kind === "created") void change(direct.ctx, { home: result.repo.id, skip: direct.skip });
          }}
        />
      )}
    </>
  );

  return { host, context, busy, shake, dialogs, confirm, fail, guarded, change, run, addDestination };
}
