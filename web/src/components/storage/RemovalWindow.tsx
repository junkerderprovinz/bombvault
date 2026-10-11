import { useEffect, useState, type ReactNode } from "react";
import {
  previewDestinationRemoval,
  removeDestination,
  type DestinationRemoval,
  type OkEnvelope,
  type StorageLocation,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { objectId } from "../../lib/storageFlow";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { credSetsChanged } from "../../lib/useCloudCredSets";
import { destinationsChanged } from "../../lib/useDestinations";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { IconClose, IconTrash } from "../navGlyphs";
import { DomainGlyph } from "./rows";

type Answer = OkEnvelope & { removal?: DestinationRemoval };

function failed(e: unknown): Answer {
  return { ok: false, error: e instanceof Error ? e.message : undefined };
}

/** One way to remove a location, with what it means for what is stored there. */
function Way({ title, text, children }: { title: string; text: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4">
      <div className="flex flex-col gap-1">
        <h3 className="text-sm font-semibold text-carbon-text">{title}</h3>
        <p className="text-xs leading-relaxed text-carbon-textSub">{text}</p>
      </div>
      {children}
    </section>
  );
}

/**
 * RemovalWindow takes a destination away after showing which sections stop
 * backing up to it. The server removes only what was shown: when the sections
 * changed in between, it answers with them as they are, and the window asks
 * again.
 */
export function RemovalWindow({
  location,
  onClose,
  onRemoved,
}: {
  location: StorageLocation;
  onClose: () => void;
  onRemoved: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const id = objectId(location);
  const [removal, setRemoval] = useState<DestinationRemoval | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    void previewDestinationRemoval(id)
      .catch(failed)
      .then((res) => {
        if (!alive) return;
        if (res.ok && res.removal) setRemoval(res.removal);
        else setProblem(res.error ?? t("settings.error"));
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function remove() {
    if (!removal) return;
    setBusy(true);
    const res = await removeDestination(id, removal).catch(failed);
    setBusy(false);
    if (!res.ok) {
      if (res.removal) setRemoval(res.removal);
      setProblem(placementErrorText(t, lang, res, "settings.error"));
      return;
    }
    push(t("storage.remove.done").replace("{name}", () => location.name), "success");
    storageLocationsChanged();
    destinationsChanged();
    offsiteTargetsChanged();
    placementChanged();
    credSetsChanged();
    onRemoved();
  }

  return (
    <InstanceWindow
      title={t("storage.remove.title").replace("{name}", () => location.name)}
      facts={t("dest.removeAsk").replace("{name}", () => location.name)}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />
          <Button
            label={t("storage.remove.action")}
            labelKey="storage.remove.action"
            glyph={<IconTrash />}
            tone="accent"
            busy={busy}
            disabled={busy || !removal}
            onClick={() => void remove()}
          />
        </>
      }
    >
      <Way title={t("storage.remove.keep")} text={t("storage.remove.keepText")}>
        {!removal && !problem && <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>}
        {removal && removal.sections.length === 0 && <p className="text-xs text-carbon-textMuted">{t("storage.rules.none")}</p>}
        {removal && removal.sections.length > 0 && (
          <div className="flex flex-col gap-2">
            <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">{t("storage.remove.detached")}</span>
            <ul className="flex flex-col divide-y divide-carbon-border/60">
              {removal.sections.map((section) => (
                <li key={section.targetId} className="flex items-start gap-3 py-2.5 first:pt-0 last:pb-0">
                  <span className="mt-0.5 flex">
                    <DomainGlyph domain={section.domain} />
                  </span>
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <span className="flex items-center gap-1.5 text-sm text-carbon-text">
                      {t(DOMAIN_LABEL[section.domain])}
                      {section.primary && <InfoBubble tip={t("storage.remove.primary")} />}
                    </span>
                    <span dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textMuted">
                      {section.repo}
                    </span>
                    {section.listedAt > 0 && (
                      <span className="text-xs text-carbon-textMuted">
                        {t("backups.tile.count", section.entries)} · {t("entry.places.count", section.backups)}
                      </span>
                    )}
                    {section.direct?.kept && <span className="text-xs text-carbon-textSub">{t("storage.remove.directKept")}</span>}
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}
      </Way>
      {problem && <p className="text-xs text-statusWarn wrap-break-word">{problem}</p>}
    </InstanceWindow>
  );
}
