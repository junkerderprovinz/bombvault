// SharedUpkeepSection shows, in the receiving server's window, how this server
// keeps the shared repositories it holds: what they save, when it tidies up,
// how long it keeps every server's backups, and how its last passes went.
import { useCallback, useEffect, useState } from "react";
import {
  getSharedUpkeep,
  runSharedUpkeep,
  setSharedUpkeepRules,
  type SharedSpace,
  type SharedUpkeep,
  type SharedUpkeepRules,
} from "../../lib/api";
import type { useT } from "../../lib/i18n";
import type { KeepCounts } from "../../lib/keepPlan";
import { humanBytes } from "../../lib/forecast";
import { formatDuration, formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { Button } from "../../components/Button";
import { IconPrune, IconRefresh } from "../../components/glyphs";
import { IconPencil } from "../../components/navGlyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { PlanButton } from "../../components/PlanButton";
import { NumberSetting } from "../../components/storage/fields";
import { KeepCountFields } from "../../components/storage/keepFields";
import { Rows, SettingRow } from "../../components/storage/rows";

type T = ReturnType<typeof useT>["t"];

/** While a tidy-up runs, the section asks again this often. */
const RUNNING_POLL_MS = 5000;

export function SharedUpkeepSection({ t }: { t: T }) {
  const { push } = useToast();
  const [repos, setRepos] = useState<SharedUpkeep[]>([]);

  const load = useCallback(async () => {
    try {
      const r = await getSharedUpkeep();
      if (r.ok) setRepos((r.repos ?? []).filter((x) => x.role === "owner"));
    } catch {
      // The window works without it, and the next open asks again.
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const running = repos.some((r) => r.running);
  useEffect(() => {
    if (!running) return;
    const id = setInterval(() => void load(), RUNNING_POLL_MS);
    return () => clearInterval(id);
  }, [running, load]);

  async function save(id: string, rules: SharedUpkeepRules) {
    try {
      const r = await setSharedUpkeepRules(id, rules);
      if (!r.ok) push(r.error ?? t("settings.error"), "fail");
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    }
    await load();
  }

  async function run(id: string) {
    try {
      const r = await runSharedUpkeep(id);
      if (!r.ok) push(r.error ?? t("settings.error"), "fail");
      else push(t("sharedUpkeep.started"), "success");
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    }
    await load();
  }

  if (repos.length === 0) return null;
  return (
    <div data-new="shared-upkeep" className="flex flex-col gap-4">
      {repos.map((repo) => (
        <OwnedRepo key={repo.id} t={t} repo={repo} onSave={(rules) => void save(repo.id, rules)} onRun={() => void run(repo.id)} />
      ))}
    </div>
  );
}

function OwnedRepo({
  t,
  repo,
  onSave,
  onRun,
}: {
  t: T;
  repo: SharedUpkeep;
  onSave: (rules: SharedUpkeepRules) => void;
  onRun: () => void;
}) {
  const rules = repo.rules;
  if (!rules) return null;
  const keep = rules.keep;
  const counts: KeepCounts = [keep.keepLast, keep.keepDaily, keep.keepWeekly, keep.keepMonthly, keep.keepYearly];
  const change = (next: Partial<SharedUpkeepRules>) => onSave({ ...rules, ...next });
  const setCount = (i: number, n: number) => {
    const c = [...counts] as KeepCounts;
    c[i] = n;
    change({ keep: { keepLast: c[0], keepDaily: c[1], keepWeekly: c[2], keepMonthly: c[3], keepYearly: c[4] } });
  };

  return (
    <section className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {t("sharedUpkeep.title")}
          <InfoBubble tip={t("sharedUpkeep.tip")} />
        </span>
        <bdi dir="ltr" className="font-mono text-xs text-carbon-textSub break-all text-start">
          {repo.location}
        </bdi>
        <Button
          label={t(repo.running ? "sharedUpkeep.running" : "sharedUpkeep.run")}
          labelKey={repo.running ? "sharedUpkeep.running" : "sharedUpkeep.run"}
          hint={t("sharedUpkeep.runTip")}
          glyph={<IconPrune />}
          tone="neutral"
          busy={repo.running}
          disabled={repo.running}
          onClick={onRun}
          className="ms-auto"
        />
      </div>

      <SpaceLine t={t} space={repo.space} />

      <Rows>
        <SettingRow label={t("sharedUpkeep.window")} hint={t("sharedUpkeep.windowTip")}>
          <PlanButton
            label={t("sharedUpkeep.window")}
            value={rules.pruneCadence}
            onChange={(next) => change({ pruneCadence: next })}
            modes={["daily", "weekly"]}
          />
        </SettingRow>
        <SettingRow label={t("sharedUpkeep.hours")}>
          <NumberSetting label={t("sharedUpkeep.hours")} value={rules.pruneHours} onCommit={(n) => change({ pruneHours: n })} />
        </SettingRow>
        <SettingRow label={t("sharedUpkeep.maxUnused")} hint={t("sharedUpkeep.maxUnusedTip")}>
          <NumberSetting label={t("sharedUpkeep.maxUnused")} value={rules.maxUnused} onCommit={(n) => change({ maxUnused: n })} />
        </SettingRow>
        <SettingRow label={t("sharedUpkeep.checkSubset")} hint={t("sharedUpkeep.checkSubsetTip")}>
          <NumberSetting label={t("sharedUpkeep.checkSubset")} value={rules.checkSubset} onCommit={(n) => change({ checkSubset: n })} />
        </SettingRow>
      </Rows>

      <div className="flex flex-col gap-1.5">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("sharedUpkeep.keep")}
          <InfoBubble tip={t("sharedUpkeep.keepTip")} />
        </span>
        {rules.keepSet ? (
          <>
            <KeepCountFields counts={counts} disabled={false} onCommit={setCount} />
            <Button
              label={t("sharedUpkeep.keepFollow")}
              labelKey="sharedUpkeep.keepFollow"
              glyph={<IconRefresh />}
              tone="neutral"
              onClick={() => change({ keepSet: false })}
              className="self-start"
            />
          </>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-xs text-carbon-textMuted">{t("sharedUpkeep.keepOwn")}</p>
            <Button
              label={t("sharedUpkeep.keepSet")}
              labelKey="sharedUpkeep.keepSet"
              glyph={<IconPencil />}
              tone="neutral"
              onClick={() => change({ keepSet: true })}
            />
          </div>
        )}
      </div>

      <LastPasses t={t} repo={repo} />
    </section>
  );
}

/** The space line: what the repository holds, what the servers apart would
 *  need, and what each server's backups use. */
export function SpaceLine({ t, space }: { t: T; space?: SharedSpace }) {
  if (!space) return <p className="text-xs text-carbon-textMuted">{t("sharedUpkeep.notMeasured")}</p>;
  const apart = space.hosts.reduce((sum, h) => sum + h.bytes, 0);
  return (
    <div className="flex flex-col gap-1">
      <p className="text-sm text-carbon-text">
        {t("sharedUpkeep.space")
          .replace("{used}", () => humanBytes(space.total))
          .replace("{servers}", () => t("sharedUpkeep.servers", space.hosts.length))
          .replace("{apart}", () => humanBytes(apart))
          .replace("{saved}", () => humanBytes(Math.max(apart - space.total, 0)))}
      </p>
      <ul className="flex flex-col gap-0.5 text-xs text-carbon-textSub">
        {space.hosts.map((h) => (
          <li key={h.host}>
            {t("sharedUpkeep.usedBy")
              .replace("{name}", () => h.name || h.host)
              .replace("{size}", () => humanBytes(h.bytes))}
          </li>
        ))}
      </ul>
    </div>
  );
}

function LastPasses({ t, repo }: { t: T; repo: SharedUpkeep }) {
  const { forget, prune, nextWindow } = repo;
  const lines: { text: string; tone: string }[] = [];
  if (forget) {
    lines.push({ text: t("sharedUpkeep.forgot", forget.forgotten).replace("{when}", () => relativeTime(t, forget.at)), tone: "text-carbon-textSub" });
    if (forget.skipped?.length) lines.push({ text: t("sharedUpkeep.skipped").replace("{names}", () => forget.skipped!.join(", ")), tone: "text-statusWarn" });
    if (forget.unknown?.length) lines.push({ text: t("sharedUpkeep.unknown").replace("{hosts}", () => forget.unknown!.join(", ")), tone: "text-carbon-textMuted" });
    if (forget.error) lines.push({ text: forget.error, tone: "text-statusFail" });
  }
  if (!prune) lines.push({ text: t("sharedUpkeep.never"), tone: "text-carbon-textMuted" });
  else if (prune.postponed?.length) {
    lines.push({
      text: t("sharedUpkeep.postponed").replace("{when}", () => relativeTime(t, prune.at)).replace("{names}", () => prune.postponed!.join(", ")),
      tone: "text-statusWarn",
    });
  } else if (prune.error) {
    lines.push({ text: t("sharedUpkeep.pruneFailed").replace("{when}", () => relativeTime(t, prune.at)).replace("{error}", () => prune.error!), tone: "text-statusFail" });
  } else {
    lines.push({
      text: t("sharedUpkeep.pruned").replace("{when}", () => relativeTime(t, prune.at)).replace("{duration}", () => formatDuration(prune.seconds)),
      tone: "text-carbon-textSub",
    });
    if (prune.checkError) lines.push({ text: t("sharedUpkeep.checkFailed").replace("{error}", () => prune.checkError!), tone: "text-statusFail" });
    else if (prune.checked) lines.push({ text: t("sharedUpkeep.checked"), tone: "text-statusOk" });
  }
  if (nextWindow) lines.push({ text: t("sharedUpkeep.next").replace("{when}", () => formatTs(nextWindow)), tone: "text-carbon-textMuted" });
  return (
    <ul className="flex flex-col gap-0.5 text-xs">
      {lines.map((l) => (
        <li key={l.text} className={`${l.tone} wrap-break-word`}>
          {l.text}
        </li>
      ))}
    </ul>
  );
}
