import { Link } from "react-router-dom";
import type { DomainStatus } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { NO_VALUE, relativeTime } from "../../lib/reltime";
import { statusLabel, statusTone } from "../../lib/runDisplay";
import { Badge } from "../../components/Badge";
import { Card } from "./Card";

// protectionChip maps the red/amber/green aggregate to a statusTone/Badge variant.
function protectionChip(level: string): string {
  switch (level) {
    case "green":
      return "ok";
    case "amber":
      return "info";
    case "red":
      return "failed";
    default:
      return "neutral";
  }
}

// A checklist row is "ok" (proven), "amber" (stale or overdue, like the chip's
// amber), "bad" (a gap, which links to Settings) or "muted" (does not apply or
// never ran, so nothing is claimed and nothing has failed).
type RowState = "ok" | "amber" | "bad" | "muted";

export function RansomwareCard({
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
  // Every row is derived from the /api/status domain fields (tamperState,
  // replicationState, drillState, encryptionOn, pruneStrategySet). The backend
  // computes them from the same inputs as the aggregate chip, so a row cannot
  // contradict it and the card needs no /api/settings round-trip.
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

  const protLabel = (level: string): string => {
    switch (level) {
      case "green":
        return t("ransomware.protGreen");
      case "amber":
        return t("ransomware.protAmber");
      default:
        return t("ransomware.protRed");
    }
  };

  // In scope: enabled domains that carry a protection posture (protection != "").
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  // Render nothing at all when no domain is in scope (nobody has off-site yet).
  if (!loading && shown.length === 0) return null;

  const ageText = (at: number): string => (at > 0 ? relativeTime(t, at) : t("containers.never"));

  // The append-only, replication and drill rows map the backend's state string,
  // which is kept consistent with the chip. Icon, label and colour all follow
  // the state, so a red or never-run row cannot read "verified". The one
  // divergence is the "" arm of appendOnlyRow, explained there.
  const appendOnlyRow = (d: DomainStatus): { label: string; state: RowState; at?: number } => {
    switch (d.tamperState) {
      case "ok":
        return { label: t("ransomware.appendOnlyVerified"), state: "ok", at: d.lastTamperAt };
      case "stale":
        return { label: t("ransomware.appendOnlyStale"), state: "amber", at: d.lastTamperAt };
      case "failed":
        return { label: t("ransomware.appendOnlyFailed"), state: "bad", at: d.lastTamperAt };
      case "never":
        return { label: t("ransomware.appendOnlyNever"), state: "bad" };
      default:
        // The off-site copy carries no append-only flag, so the backend has
        // nothing to prove (protectionChecks leaves Tamper empty exactly when
        // !offsiteImmutable). A grey dash would read as "does not apply" here,
        // and it does apply: whatever reaches the credentials on this box can
        // delete a deletable off-site copy. Amber marks the gap without
        // claiming a failure.
        //
        // This is the one row whose colour differs from the chip's. Every
        // other amber row means something was due and has not happened, and
        // protectionLevel folds those in. This one reports a configuration the
        // user may have chosen (plenty of cloud targets cannot do append-only
        // at all), and a chip that reads "Needs attention" for good over a
        // choice stops being a health signal.
        //
        // The row turns amber only once an off-site copy exists. With none
        // configured the row above is already red and links to the same
        // settings page.
        return {
          label: t("ransomware.appendOnlyOff"),
          state: d.offsiteConfigured ? "amber" : "muted",
        };
    }
  };
  const replicationRow = (d: DomainStatus): { label: string; state: RowState; at?: number } => {
    switch (d.replicationState) {
      case "ok":
        return { label: t("ransomware.replicationCurrent"), state: "ok", at: d.lastReplicationAt };
      case "overdue":
        return { label: t("ransomware.replicationOverdue"), state: "amber", at: d.lastReplicationAt };
      case "never":
        return { label: t("ransomware.replicationNever"), state: "muted" };
      case "paused":
        return { label: t("ransomware.replicationPaused"), state: "amber" };
      default:
        // Replication is coupled to each backup, so there is no expectation of its own.
        return { label: t("ransomware.replicationCurrent"), state: "muted" };
    }
  };
  const drillRow = (d: DomainStatus): { label: string; state: RowState; at?: number; detail?: string } => {
    switch (d.drillState) {
      case "ok":
        return { label: t("ransomware.drillOffsite"), state: "ok", at: d.lastDrDrillAt };
      case "failed":
        // The latest off-site DR drill failed. The row is red like the "proven
        // restorable" pill, however recently it ran, and carries the scrubbed
        // reason so it can name the check and why it failed.
        return { label: t("ransomware.drillFailed"), state: "bad", at: d.lastDrDrillAt, detail: d.drillDetail };
      case "overdue":
        return { label: t("ransomware.drillOverdue"), state: "amber", at: d.lastDrDrillAt };
      case "never":
        return { label: t("ransomware.drillNever"), state: "muted" };
      default:
        // No drill schedule is set, so nothing is claimed.
        return { label: t("ransomware.drillOffsite"), state: "muted" };
    }
  };

  return (
    <Card title={t("ransomware.title")} hueIndex={hueIndex}>
      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {!loading && (
      // Rows separated by shade (soft tiles), never divider lines; the same
      // surface token every other Dashboard list uses.
      <div className="flex flex-col gap-1 glim-content-fade">
      {shown.map((d) => {
          // Each row has a label, a state and an optional age stamp. A "bad" row
          // is a gap the user should fix, so it links into Settings. Every state
          // comes from the backend and cannot diverge from the chip above.
          const ao = appendOnlyRow(d);
          const rep = replicationRow(d);
          const dr = drillRow(d);
          const rows: { key: string; label: string; state: RowState; at?: number; detail?: string }[] = [
            {
              key: "configured",
              label: t("ransomware.configured"),
              state: d.offsiteConfigured ? "ok" : "bad",
            },
            { key: "appendOnly", ...ao },
            { key: "replication", ...rep },
            { key: "drill", ...dr },
            {
              key: "encryption",
              label: t("ransomware.encryptionOn"),
              state: d.encryptionOn ? "ok" : "bad",
            },
            {
              key: "prune",
              label: t("ransomware.pruneStrategy"),
              state: d.pruneStrategySet ? "ok" : "bad",
            },
          ];

          return (
            <div key={d.domain} className="flex flex-col gap-1.5 rounded-control bg-carbon-surface2 px-2 py-2.5">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="font-medium text-carbon-text w-28 shrink-0 truncate">
                  {domainLabel(d.domain)}
                </span>
                <Badge tone={statusTone(protectionChip(d.protection))}>{statusLabel(protectionChip(d.protection), t)}</Badge>
                <span className="min-w-0 text-sm text-carbon-textSub">{protLabel(d.protection)}</span>
              </div>
              <div className="flex flex-col gap-0.5 ps-1">
                {rows.map((row) => {
                  const icon =
                    row.state === "ok" ? "✓" : row.state === "amber" ? "!" : row.state === "bad" ? "✗" : NO_VALUE;
                  const iconColor =
                    row.state === "ok"
                      ? "text-statusOk"
                      : row.state === "amber"
                        ? "text-statusWarn"
                        : row.state === "bad"
                          ? "text-statusFail"
                          : "text-carbon-textMuted";
                  const labelColor =
                    row.state === "amber"
                      ? "text-statusWarn"
                      : row.state === "muted"
                        ? "text-carbon-textMuted"
                        : "text-carbon-textSub";
                  return (
                    <div key={row.key} className="flex flex-col gap-0.5">
                      <div className="flex items-center gap-2 text-sm">
                        <span className={`w-4 shrink-0 text-center ${iconColor}`}>{icon}</span>
                        {row.state === "bad" ? (
                          // A plain link, not a Badge: the label navigates only
                          // in the "bad" state, and its siblings render as
                          // plain text-sm. A fixed-height chip on the one
                          // clickable state would make the row's height and
                          // type jump with whichever domain is faulted. The
                          // fail colour and the hover underline say both
                          // "wrong" and "clickable".
                          <Link to="/settings/offsite" className="text-statusFail hover:underline flex-1 truncate min-w-0 pointer-coarse:py-3">
                            {row.label}
                          </Link>
                        ) : (
                          <span className={`flex-1 truncate min-w-0 ${labelColor}`}>{row.label}</span>
                        )}
                        {row.at !== undefined && (
                          <span className="text-xs text-carbon-textMuted shrink-0">{ageText(row.at)}</span>
                        )}
                      </div>
                      {/* Which check failed and why (the off-site DR reason from /api/status). */}
                      {row.detail && (
                        <span className="text-xs text-statusFail wrap-break-word ps-6" title={row.detail}>
                          {t("drill.checkOffsiteDr")} · {t("drill.failReasonPrefix")} {row.detail}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
      )}
    </Card>
  );
}
