import { useEffect, useState } from "react";
import { runDrill } from "../../lib/api";
import type { DomainStatus } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { formatTs, relativeTime } from "../../lib/reltime";
import { statusLabel, statusTone } from "../../lib/runDisplay";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { formatCadence } from "../../components/CadenceBuilder";
import { IconCheck } from "../../components/Sidebar";
import { Card } from "./Card";
import { chipForRpo } from "./rpo";

export function ProtectionCard({
  t,
  domains,
  loading,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  hueIndex?: number;
}) {
  const { lang } = useT();

  // Manual off-site DR run, triggered from a failing DR row so a pass clears the
  // red. `drRunning` is the domain whose DR check is in flight; `drRunError`
  // holds the last returned failure detail per domain (shown next to the button).
  const [drRunning, setDrRunning] = useState<string | null>(null);
  const [drRunError, setDrRunError] = useState<Record<string, string>>({});

  // Keep a domain's transient manual-run message only where the Run-DR button is
  // actually reachable (a DR-capable, non-off domain with an off-site repo), and
  // drop it elsewhere so a refetch (including one triggered by another domain's
  // run) can't resurface a stale error. The next run for that domain clears it.
  useEffect(() => {
    setDrRunError((prev) => {
      const next: Record<string, string> = {};
      for (const d of domains) {
        const drCapable =
          d.domain === "containers" || d.domain === "flash" || d.domain === "files" || d.domain === "zfs";
        const reachable = drCapable && d.status !== "off" && d.offsiteConfigured;
        if (reachable && prev[d.domain] !== undefined) next[d.domain] = prev[d.domain];
      }
      return Object.keys(next).length === Object.keys(prev).length ? prev : next;
    });
  }, [domains]);

  const runOffsiteDr = (domain: string) => {
    setDrRunning(domain);
    setDrRunError((e) => {
      const next = { ...e };
      delete next[domain];
      return next;
    });
    void runDrill(domain, "offsite", "dr")
      .then((res) => {
        // A drill that ran, pass or fail, is recorded and comes back with the
        // status refetch below as d.drillDetail. Only a run that left no row
        // (the repo was busy, say) needs its own message next to the button.
        if (!res.ok && !res.drill) {
          setDrRunError((e) => ({ ...e, [domain]: res.error ?? t("verify.failed") }));
        }
      })
      .catch((err) => {
        setDrRunError((e) => ({
          ...e,
          [domain]: err instanceof Error ? err.message : t("verify.failed"),
        }));
      })
      .finally(() => {
        setDrRunning((cur) => (cur === domain ? null : cur));
        // Refetch the shared /api/status so a pass clears the red DR pill + reason
        // (the Dashboard page listens for this event and reloads getStatus()).
        window.dispatchEvent(new Event("bv:settings-changed"));
      });
  };

  const domainLabel = (domain: string): string => {
    switch (domain) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
      case "files":
        return t("dashboard.domainFiles");
      case "zfs":
        return t("dashboard.domainZFS");
      default:
        return domain;
    }
  };

  const rpoLabel = (status: string): string => {
    switch (status) {
      case "ok":
        return t("dashboard.rpoOk");
      case "warn":
        return t("dashboard.rpoWarn");
      case "overdue":
        return t("dashboard.rpoOverdue");
      case "never":
        return t("dashboard.rpoNever");
      default:
        return t("dashboard.rpoOff");
    }
  };

  return (
    <Card title={t("dashboard.protectionTitle")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && domains.length > 0 && (
        // Rows separated by shade (soft tiles), never divider lines.
        // In a wide card the rows share one grid through subgrid, so every
        // badge column is as wide as its longest badge on any row and the
        // badges stay on one line while the columns still line up.
        <div className="@container glim-content-fade">
        <div className="flex flex-col gap-1 @[60rem]:grid @[60rem]:grid-cols-[7rem_auto_minmax(6rem,1fr)_auto_auto_auto] @[60rem]:gap-x-3">
          {domains.map((d) => {
            const off = d.status === "off";
            // Only containers, flash, files and ZFS run an off-site DR drill
            // (schedule.go drillTasks / runDRDrill). VMs and config can have an
            // off-site repo but cannot be drilled, so they show no DR pill and
            // no Run-DR button.
            const drCapable =
              d.domain === "containers" || d.domain === "flash" || d.domain === "files" || d.domain === "zfs";
            // The scheduled DR drill is switched off for a domain that has an
            // off-site repo (#37). The pill then reads neutral ("manual only"),
            // unless there is a failing result to show.
            const drUnscheduled = drCapable && d.offsiteConfigured && !d.offsiteDrillScheduled;
            // A recorded off-site DR drill that failed. A failure shows whether the
            // run was scheduled or manual, and the opt-out never masks it: only a
            // domain that was never drilled goes neutral.
            const drFailed = !off && drCapable && d.lastDrDrillAt > 0 && !d.lastDrDrillOK;
            return (
              <div
                key={d.domain}
                className="flex flex-col gap-1 rounded-control bg-carbon-surface2 px-2 py-2.5 text-sm @[60rem]:col-span-full @[60rem]:grid @[60rem]:grid-cols-subgrid @[60rem]:items-center"
              >
                {/* In a wide card the cells sit in the shared columns [domain]
                    [status] [schedule] [last run] [verified] [off-site verified]
                    [off-site DR], so the same kind of fact lines up down the card
                    and an absent badge leaves its column blank. The status and
                    badge columns take their longest entry, and the schedule
                    truncates instead. In a narrow card the row wraps as a flex stack under
                    the domain name, so it never scrolls sideways. */}
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 py-0.5 @[60rem]:col-span-full @[60rem]:grid @[60rem]:grid-cols-subgrid">
                  <span
                    className={`col-start-1 basis-full @[60rem]:basis-auto min-w-0 truncate font-medium ${
                      off ? "text-carbon-textMuted" : "text-carbon-text"
                    }`}
                  >
                    {domainLabel(d.domain)}
                  </span>
                  {off ? (
                    <span className="col-start-2 col-span-6 min-w-0 truncate text-xs text-carbon-textMuted">
                      {t("dashboard.rpoOff")}
                    </span>
                  ) : (
                    <>
                      {/* Column 2, status: the RPO chip and its label stay together
                          so the pill never drifts from the words it qualifies. */}
                      <div className="col-start-2 flex min-w-0 items-center gap-2">
                        <span className="shrink-0">
                          <Badge tone={statusTone(chipForRpo(d.status))}>{statusLabel(chipForRpo(d.status), t)}</Badge>
                        </span>
                        <span className="min-w-0 truncate text-carbon-text">
                          {rpoLabel(d.status)}
                        </span>
                      </div>
                      {/* Column 3, schedule cadence. A domain with no cadence of
                          its own can still be covered by the whole-server
                          "Backup Everything" pass, and then the pass's cadence
                          applies. Naming it keeps the row from contradicting
                          the domain's own card, which correctly shows no
                          schedule (#177). */}
                      <span className="col-start-3 min-w-0 truncate text-carbon-textMuted text-xs">
                        {d.coveredBy
                          ? t("dashboard.rpoViaEverything").replace("{cadence}", formatCadence(d.coveredBy, t, lang))
                          : formatCadence(d.schedule, t, lang)}
                      </span>
                      {/* Column 4: last successful run. */}
                      <span
                        className="col-start-4 text-start @[60rem]:text-end text-carbon-textMuted text-xs"
                        title={formatTs(d.lastSuccess)}
                      >
                        {d.lastSuccess ? relativeTime(t, d.lastSuccess) : t("containers.never")}
                      </span>
                      {/* Column 5: local-verify shield badge. */}
                      {d.lastVerified ? (
                        <div className="col-start-5 min-w-0">
                          <Badge
                            tone={d.lastVerifiedOK ? "ok" : "fail"}
                            className="whitespace-nowrap"
                            title={`${t("verify.shield")} · ${formatTs(d.lastVerified)}`}
                          >
                            {d.lastVerifiedOK ? "✓" : "✗"} {t("verify.shield")} {relativeTime(t, d.lastVerified)}
                          </Badge>
                        </div>
                      ) : null}
                      {/* The off-site badges come from recorded runs, so a domain
                          with no second copy anywhere would show none of them
                          and say nothing at the point where it matters most.
                          That there is no second copy is the first fact about a
                          backup, not an advanced one, so it shows here and not
                          only in the advanced ransomware card. A repository
                          marked off the premises counts as that second copy. */}
                      {!d.offsiteConfigured && !d.offPremisesCovered ? (
                        <div className="col-start-6 min-w-0">
                          <Badge tone="fail" className="whitespace-nowrap" title={t("dashboard.noOffsiteTitle")}>
                            ✗ {t("dashboard.noOffsite")}
                          </Badge>
                        </div>
                      ) : null}
                      {/* Column 6: the off-site integrity check (#63), `restic
                          check --read-data-subset` against the off-site repo,
                          with the same pills as the local-verify shield. VMs can
                          run no other off-site drill (DR restores are refused
                          for them), so it shows for every domain with a
                          recorded run, next to the DR pill below. */}
                      {d.lastOffsiteSubsetAt ? (
                        <div className="col-start-6 min-w-0">
                          <Badge
                            tone={d.lastOffsiteSubsetOK ? "ok" : "fail"}
                            className="whitespace-nowrap"
                            title={`${t("drill.offsiteVerified")} · ${formatTs(d.lastOffsiteSubsetAt)}`}
                          >
                            {d.lastOffsiteSubsetOK ? "✓" : "✗"} {t("drill.offsiteVerified")} {relativeTime(t, d.lastOffsiteSubsetAt)}
                          </Badge>
                        </div>
                      ) : null}
                    </>
                  )}
                </div>
                {/* The newest start test of any container, beside the
                    verification row it belongs with. */}
                {!off && d.lastStartTest && (
                  <div className="flex flex-wrap items-center gap-2 ps-1 @[60rem]:col-span-full">
                    <Badge
                      tone={d.lastStartTest.ok ? "ok" : "fail"}
                      wrap
                      className="max-w-full"
                      title={[d.lastStartTest.detail, formatTs(d.lastStartTest.at)].filter(Boolean).join(" · ")}
                    >
                      {d.lastStartTest.ok ? "✓" : "✗"}{" "}
                      {t(d.lastStartTest.ok ? "dashboard.startTestOk" : "dashboard.startTestFailed").replace(
                        "{name}",
                        d.lastStartTest.container
                      )}{" "}
                      · {relativeTime(t, d.lastStartTest.at)}
                    </Badge>
                  </div>
                )}
                {/* The "Run off-site DR check" button is reachable for every
                    configured domain, so a manual run works when the drill is
                    opted out and when the row is green. The red check name and
                    reason show for a recorded failure (drFailed). A local subset
                    pass cannot clear that red, so the button runs {offsite,dr}. */}
                {/* The off-site DR result sits here rather than in its own
                    column: it is the longest badge by far and would squeeze the
                    schedule out of every row. Only containers, flash, folders and
                    ZFS run a DR drill; on a failure the tooltip names the check
                    and the reason. */}
                {!off && drCapable && (drFailed || d.offsiteConfigured || d.lastDrDrillAt > 0) && (
                  <div className="flex flex-wrap items-center gap-2 ps-1 @[60rem]:col-span-full">
                    {/* A passed run counts even when it was manual and the
                        scheduled drill is off; a failure always shows, and only
                        a domain never drilled goes neutral. */}
                    {d.lastDrDrillAt > 0 && d.lastDrDrillOK ? (
                      <Badge
                        tone="ok"
                        wrap
                        className="max-w-full @[60rem]:whitespace-nowrap"
                        title={`${t("drill.provenOffsite")} · ${formatTs(d.lastDrDrillAt)}`}
                      >
                        ✓ {t("drill.provenOffsite")} · {relativeTime(t, d.lastDrDrillAt)}
                      </Badge>
                    ) : drFailed ? (
                      <Badge
                        tone="fail"
                        wrap
                        className="max-w-full @[60rem]:whitespace-nowrap"
                        title={
                          d.drillDetail
                            ? `${t("drill.checkOffsiteDr")} · ${t("drill.failReasonPrefix")} ${d.drillDetail} · ${formatTs(d.lastDrDrillAt)}`
                            : `${t("drill.provenOffsite")} · ${formatTs(d.lastDrDrillAt)}`
                        }
                      >
                        ✗ {t("drill.provenOffsite")} · {relativeTime(t, d.lastDrDrillAt)}
                      </Badge>
                    ) : drUnscheduled ? (
                      <Badge tone="neutral" wrap className="max-w-full @[60rem]:whitespace-nowrap" title={t("drill.manualOnlyTitle")}>
                        {t("drill.manualOnly")}
                      </Badge>
                    ) : null}
                    {drFailed && d.drillDetail && (
                      <span className="text-xs text-statusFail wrap-break-word" title={d.drillDetail}>
                        {t("drill.checkOffsiteDr")} · {t("drill.failReasonPrefix")} {d.drillDetail}
                      </span>
                    )}
                    {d.offsiteConfigured && (
                      <Button
                        label={d.lastDrDrillAt && d.lastDrDrillOK
                            ? t("drill.rerunOffsiteDr")
                            : t("drill.runOffsiteDr")}
                        labelKey={d.lastDrDrillAt && d.lastDrDrillOK ? "drill.rerunOffsiteDr" : "drill.runOffsiteDr"}
                        glyph={<IconCheck />}
                        tone="neutral"
                        onClick={() => runOffsiteDr(d.domain)}
                        className="glim-btn-wrap pointer-coarse:[--btn-h:2.75rem]"
                        disabled={drRunning === d.domain}
                        busy={drRunning === d.domain}
                        title={drRunning === d.domain ? t("drill.runningOffsiteDr") : undefined}
                      />
                    )}
                    {drRunError[d.domain] && (
                      <span className="text-xs text-statusFail wrap-break-word">✗ {drRunError[d.domain]}</span>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
        </div>
      )}
    </Card>
  );
}
