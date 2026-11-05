import { useContext, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { confirmPlacementDefault, type OkEnvelope, type PlacementDomain, type Run } from "../../lib/api";
import { ATTENTION_CHANGED_EVENT, LocationsContext } from "../../lib/attention";
import { decisionRoute, rulesRoute, runSection } from "../../lib/copyDecision";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { Button } from "../Button";
import { IconConfirm, IconForward } from "../glyphs";

/** useCopyDecisionText is what the decision says about the section. */
export function useCopyDecisionText(domain: PlacementDomain): { title: string; body: string } {
  const { t } = useT();
  const label = t(DOMAIN_LABEL[domain]);
  return {
    title: t("copyDecision.title").replaceAll("{domain}", () => label),
    body: t("copyDecision.body").replaceAll("{domain}", () => label),
  };
}

/**
 * CopyDecisionActions are the two answers: take the backups as this server's
 * own, which confirms the section's default and lets its copies run, or look
 * at the section's rules first, where copying keeps waiting until it is
 * confirmed.
 */
export function CopyDecisionActions({ domain, className }: { domain: PlacementDomain; className?: string }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const label = t(DOMAIN_LABEL[domain]);

  async function take() {
    setBusy(true);
    try {
      const res: OkEnvelope = await confirmPlacementDefault(domain, []).catch((e: unknown) => ({
        ok: false,
        error: e instanceof Error ? e.message : undefined,
      }));
      if (!res.ok) {
        push(placementErrorText(t, lang, res, "settings.error"), "fail");
        return;
      }
      push(t("copyDecision.taken").replaceAll("{domain}", () => label), "success");
      placementChanged();
      storageLocationsChanged();
      window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Button
        className={className}
        label={t("copyDecision.take")}
        labelKey="copyDecision.take"
        glyph={<IconConfirm />}
        tone="accent"
        onClick={() => void take()}
        disabled={busy}
        busy={busy}
      />
      <Button
        className={className}
        label={t("copyDecision.check")}
        labelKey="copyDecision.check"
        glyph={<IconForward />}
        tone="neutral"
        onClick={() => navigate(rulesRoute(domain))}
        disabled={busy}
      />
    </>
  );
}

/** CopyDecision asks, on a storage location's page, whether the backups found
 *  of a section are this server's own. */
export function CopyDecision({ domain }: { domain: PlacementDomain }) {
  const { title, body } = useCopyDecisionText(domain);
  return (
    <section
      data-testid="copy-decision"
      aria-label={title}
      className="flex flex-col gap-3 rounded-card bg-statusWarnBgSoft px-4 py-3 text-sm leading-relaxed text-carbon-text"
    >
      <div className="flex flex-col gap-1">
        <h2 className="font-semibold text-statusWarn">{title}</h2>
        <p>{body}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <CopyDecisionActions domain={domain} className="glim-btn-wrap" />
      </div>
    </section>
  );
}

/** CopyWaitsLink leads from a backup whose off-site copy waits to the
 *  decision it waits for. */
export function CopyWaitsLink({ run }: { run: Run }) {
  const { t } = useT();
  const locations = useContext(LocationsContext);
  const domain = runSection(run);
  if (!run.copyWaits || !domain) return null;
  return (
    <Link
      to={decisionRoute(locations, domain)}
      className="flex-none text-xs text-accentText underline decoration-carbon-border underline-offset-[3px] hover:decoration-current"
    >
      {t("copyDecision.decide")}
    </Link>
  );
}
