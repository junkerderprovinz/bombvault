import { Link } from "react-router-dom";
import type { DomainStatus } from "../../lib/api";
import { isFreshInstall } from "../../lib/freshInstall";
import type { useT } from "../../lib/i18n";
import { Button } from "../../components/Button";

export const RECOVERY_NUDGE_DISMISSED = "bombvault.recoveryNudgeDismissed";

// FreshInstallNudge points a new or rebuilt install, where no domain has ever
// backed up, at the Recovery tab to bring back its existing backups. It reads
// the status domains the page has already fetched, and the dismissal persists
// in localStorage.
export function FreshInstallNudge({
  t,
  domains,
  loading,
  dismissed,
  onDismiss,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  dismissed: boolean;
  onDismiss: () => void;
}) {
  // Gate: do nothing (and read nothing) once dismissed or while status is still
  // loading. Only then is the fresh predicate evaluated against shared data.
  if (dismissed || loading) return null;
  if (!isFreshInstall(domains)) return null;

  return (
    <div className="bg-carbon-surface rounded-card p-5 flex items-center gap-4">
      <div className="flex-1 flex flex-col gap-1.5">
        <p className="text-sm text-carbon-text">{t("recovery.freshNudge")}</p>
        {/* The card's one call to action is its primary action, so the link
            takes the filled accent pill the app's primary buttons use. It
            cannot be a Button, which renders a <button> and cannot navigate.
            Under 48rem it takes the link-as-control height the Config and
            Flash destination links carry, so it stays a touch target. */}
        <Link
          to="/recovery"
          className="self-start inline-flex items-center gap-1 rounded-pill bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity max-md:min-h-[2.75rem]"
        >
          {t("recovery.freshNudgeCta")} <span className="inline-block rtl:-scale-x-100">→</span>
        </Link>
      </div>
      {/* A chip normally rides inside a host pill whose row carries the touch
          floor. This one is the card's only close control, loose in a flex
          row, so below 48rem an ::after owned by the button widens its 18px
          engine box by 14px on each side (46px in total). A padded wrapper
          would only be dead zone. */}
      <Button
        label={t("common.close")}
        labelKey="common.close"
        variant="chip"
        onClick={onDismiss}
        className="shrink-0 max-md:relative max-md:after:absolute max-md:after:-inset-3.5 max-md:after:content-['']"
      />
    </div>
  );
}
