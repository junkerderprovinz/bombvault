import { useEffect, useState } from "react";
import {
  moveSection,
  previewSectionMove,
  type OffsiteDomain,
  type OkEnvelope,
  type SectionMove,
  type StorageLocation,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { objectId } from "../../lib/storageFlow";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { useStorageLocations } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { RadioMark } from "../RadioMark";
import { IconForward } from "../glyphs";
import { IconClose } from "../navGlyphs";
import { LocationMark } from "./LocationMark";

type Answer = OkEnvelope & { move?: SectionMove; started?: boolean };

function failed(e: unknown): Answer {
  return { ok: false, error: e instanceof Error ? e.message : undefined };
}

const CODE = "break-all text-start font-mono text-xs text-carbon-textSub";
const NOTE = "rounded-control bg-carbon-surface2 px-3 py-2 text-xs leading-relaxed text-carbon-textSub";

/** The destinations a section's copy can move to: every other one that has no
 *  copy of the section yet. */
export function moveCandidates(locations: StorageLocation[], from: StorageLocation, domain: OffsiteDomain): StorageLocation[] {
  return locations.filter(
    (location) =>
      location.object === "destination" && location.id !== from.id && !location.sections.some((section) => section.domain === domain)
  );
}

/**
 * MoveWindow moves the copy one section keeps at a storage location to
 * another destination, history included. Picking the new place asks the
 * server what the move would copy, which lists both places. The move itself
 * runs in the background, so the window closes once it has started.
 */
export function MoveWindow({
  location,
  domain,
  targetId,
  onClose,
}: {
  location: StorageLocation;
  domain: OffsiteDomain;
  targetId: string;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { locations, loaded } = useStorageLocations();
  const [to, setTo] = useState<StorageLocation | null>(null);
  const [plan, setPlan] = useState<SectionMove | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const section = t(DOMAIN_LABEL[domain]);
  const candidates = moveCandidates(locations, location, domain);
  const toId = to && objectId(to);

  useEffect(() => {
    if (!toId) return;
    let alive = true;
    void previewSectionMove(targetId, toId)
      .catch(failed)
      .then((res: Answer) => {
        if (!alive) return;
        if (res.ok && res.move) setPlan(res.move);
        else setProblem(placementErrorText(t, lang, res, "settings.error"));
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetId, toId]);

  function pick(next: StorageLocation) {
    if (next.id === to?.id) return;
    setTo(next);
    setPlan(null);
    setProblem(null);
  }

  async function start() {
    if (!to || !toId) return;
    setBusy(true);
    const res: Answer = await moveSection(targetId, toId).catch(failed);
    setBusy(false);
    if (!res.ok) {
      setProblem(placementErrorText(t, lang, res, "settings.error"));
      return;
    }
    push(t("storage.move.started").replace("{domain}", () => section).replace("{name}", () => to.name), "success");
    onClose();
  }

  return (
    <InstanceWindow
      title={t("storage.move.title").replace("{domain}", () => section)}
      facts={t("storage.move.hint")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />
          <Button
            label={t("storage.move.action")}
            labelKey="storage.move.action"
            glyph={<IconForward />}
            tone="accent"
            busy={busy}
            disabled={busy || !plan}
            onClick={() => void start()}
          />
        </>
      }
    >
      {!loaded ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : candidates.length === 0 ? (
        <p className="text-sm text-carbon-textSub">{t("storage.move.none").replace("{domain}", () => section)}</p>
      ) : (
        <div className="flex flex-col gap-2">
          <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">{t("storage.move.to")}</span>
          <div role="radiogroup" aria-label={t("storage.move.to")} className="flex flex-col gap-1.5">
            {candidates.map((candidate) => {
              const picked = candidate.id === to?.id;
              return (
                <button
                  key={candidate.id}
                  type="button"
                  role="radio"
                  aria-checked={picked}
                  aria-label={candidate.name}
                  disabled={busy}
                  onClick={() => pick(candidate)}
                  className={`flex items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2.5 text-start hover:bg-carbon-surface3 ${picked ? "ring-2 ring-accent" : ""}`}
                >
                  <RadioMark on={picked} />
                  <LocationMark location={candidate} />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="truncate text-sm font-semibold text-carbon-text">{candidate.name}</span>
                    <span dir="ltr" className="truncate text-start text-xs text-carbon-textMuted">
                      {candidate.where}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}
      {to && !plan && !problem && <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>}
      {plan && (
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1 rounded-control bg-carbon-background px-3 py-2">
            <code dir="ltr" className={CODE}>
              {plan.from}
            </code>
            <code dir="ltr" className={`${CODE} text-carbon-text`}>
              {plan.to}
            </code>
          </div>
          <p className="text-sm text-carbon-text">
            {t("entry.places.count", plan.backups)}
            {" · "}
            {plan.pending > 0 ? t("storage.move.pending", plan.pending) : t("storage.move.allThere")}
          </p>
          <p className={NOTE}>{t("storage.move.stays")}</p>
          <p className={NOTE}>{t("storage.move.duration")}</p>
        </div>
      )}
      {problem && <p className="text-xs text-statusWarn wrap-break-word">{problem}</p>}
    </InstanceWindow>
  );
}
