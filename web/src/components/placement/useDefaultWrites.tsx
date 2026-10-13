import { useState, type ReactNode } from "react";
import {
  ApiError,
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
import { useLocalNames } from "../../lib/useLocalName";
import { useNamedRepos } from "../../lib/useNamedRepos";
import { InfoBubble } from "../InfoBubble";
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
  const localOf = useLocalNames();
  const repos = useNamedRepos();
  const { confirm, confirmDialog, dismiss, revise } = useConfirm();
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
      homeName: (id) => viewHomeLabel(t, localOf(row.domain), { ...view, repo: id, ...labelOf(id), repoOff: false }, options),
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
      dismiss();
      fail({ ok: false, error: err instanceof ApiError ? err.message : undefined });
    } finally {
      setBusy(false);
    }
  }

  function homeLead(ctx: DefaultContext, home: string): string {
    const domain = domainLabel(t, ctx.row.domain);
    return t("placementDefaults.confirmHome").replace("{domain}", () => domain).replace("{home}", () => ctx.homeName(home));
  }

  function asking(home: string | undefined, extra?: ReactNode): ConfirmOptions {
    return home !== undefined
      ? { extra, confirmLabel: t("placement.saveHome"), confirmLabelKey: "placement.saveHome", cancelTone: "neutral" }
      : { extra, cancelTone: "neutral" };
  }

  // A change that neither moves the location nor changes what a target
  // receives is saved without a question, and has none.
  function question(ctx: DefaultContext, impact: DefaultImpact, home: string | undefined): [string, ConfirmOptions] | null {
    const lines = impactLines(t, lang, ctx.row.domain, impact, ctx.homeName(home ?? ctx.row.home));
    const lead = home !== undefined ? homeLead(ctx, home) : lines.shift();
    if (lead === undefined) return null;
    const extra =
      lines.length > 0 ? (
        <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
          {lines.map((line, i) => (
            <p key={i}>{line}</p>
          ))}
        </div>
      ) : undefined;
    return [lead, asking(home, extra)];
  }

  // A preview the server refused refuses the write as well, so only the way
  // out is left. One that never came back leaves the write to the server,
  // which without the numbers saves only a change that touches no storage
  // location and answers any other with the numbers to ask about.
  function unchecked(ctx: DefaultContext, home: string | undefined, err: unknown): [string, ConfirmOptions] {
    const failed = err instanceof ApiError ? err : undefined;
    const reason = failed?.message ?? t("apiError.serverFailed");
    const detail = failed?.detail;
    const said = detail && <InfoBubble tip={t("apiError.said").replace("{said}", () => detail)} />;
    if (failed && failed.status >= 400 && failed.status < 500 && failed.status !== 408) {
      return [reason, { extra: said, confirmBlocked: reason, cancelTone: "neutral" }];
    }
    const note = t("placementDefaults.previewFailed").replace("{reason}", () => reason);
    if (home === undefined) return [note, asking(undefined, said)];
    return [
      homeLead(ctx, home),
      asking(
        home,
        <p className="flex items-start gap-1 text-sm text-carbon-textSub">
          <span>{note}</span>
          {said}
        </p>
      ),
    ];
  }

  function change(ctx: DefaultContext, next: DefaultChange) {
    const { row } = ctx;
    const home = next.home !== undefined && next.home !== row.home ? next.home : undefined;
    return guarded(async () => {
      // Working out the impact takes the server a moment, as long as it takes
      // to list the snapshots, so the window opens at once and waits for it.
      const waiting = t("restoreCheck.running");
      let answered = false;
      const first = confirm(home !== undefined ? homeLead(ctx, home) : waiting, { ...asking(home), confirmBlocked: waiting });
      void first.then(() => {
        answered = true;
      });
      let impact: DefaultImpact | undefined;
      let asked: [string, ConfirmOptions] | null;
      try {
        const preview = await previewPlacementDefault(row.domain, next);
        if (answered) return;
        if (!preview.ok || !preview.impact) {
          dismiss();
          fail(preview);
          return;
        }
        impact = preview.impact;
        asked = question(ctx, impact, home);
      } catch (err) {
        if (answered) return;
        asked = unchecked(ctx, home, err);
      }
      if (asked) revise(...asked);
      else dismiss();
      for (let round = 0; ; round++) {
        if (asked && !(await (round === 0 ? first : confirm(...asked)))) return;
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
        asked = question(ctx, impact, home);
      }
    });
  }

  function run(ctx: DefaultContext, step: PlacementStep) {
    if (step.kind === "blocked") {
      push(blockedText(t, step, viewHomeLabel(t, localOf(ctx.row.domain), ctx.view, ctx.options)), "warn");
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

  return { context, busy, shake, dialogs, confirm, fail, guarded, change, run, addDestination };
}
