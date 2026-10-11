import { useEffect, useState, type ReactNode } from "react";
import {
  eraseDestination,
  previewDestinationErasure,
  previewDestinationRemoval,
  removeDestination,
  type DestinationErasure,
  type DestinationRemoval,
  type OkEnvelope,
  type StorageLocation,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { objectId } from "../../lib/storageFlow";
import { useToast } from "../../lib/toast";
import { credSetsChanged } from "../../lib/useCloudCredSets";
import { destinationsChanged } from "../../lib/useDestinations";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { RadioMark } from "../RadioMark";
import { IconClose, IconTrash } from "../navGlyphs";
import { DetachedList, ErasureDetails, erasable } from "./removalParts";

type Answer = OkEnvelope & { removal?: DestinationRemoval; erasure?: DestinationErasure };

function failed(e: unknown): Answer {
  return { ok: false, error: e instanceof Error ? e.message : undefined };
}

type How = "keep" | "delete";

/** One way to remove a location. What it means for the backups there shows
 *  once it is the way picked. */
function Way({
  title,
  text,
  picked,
  onPick,
  children,
}: {
  title: string;
  text: string;
  picked: boolean;
  onPick: () => void;
  children: ReactNode;
}) {
  return (
    <section className={`flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4 ${picked ? "ring-2 ring-accent" : ""}`}>
      <button type="button" role="radio" aria-checked={picked} onClick={onPick} className="flex items-start gap-3 rounded-control text-start">
        <RadioMark on={picked} className="mt-0.5" />
        <span className="flex min-w-0 flex-col gap-1">
          <span className="text-sm font-semibold text-carbon-text">{title}</span>
          <span className="text-xs leading-relaxed text-carbon-textSub">{text}</span>
        </span>
      </button>
      {picked && children}
    </section>
  );
}

/**
 * RemovalWindow takes a destination away in one of two ways: it only detaches
 * the sections and leaves the backups there, or it deletes the backups first.
 * Either way the server acts only on what was shown: when the sections or the
 * entries that lose their only backup changed in between, it answers with
 * them as they are, and the window asks again.
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
  const [how, setHow] = useState<How>("keep");
  const [removal, setRemoval] = useState<DestinationRemoval | null>(null);
  const [erasure, setErasure] = useState<DestinationErasure | null>(null);
  // Counts the readings of what would be deleted. None is taken before the
  // way is picked, because a reading lists every repository at the destination.
  const [readings, setReadings] = useState(0);
  const [typed, setTyped] = useState("");
  const [lastToo, setLastToo] = useState(false);
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

  useEffect(() => {
    if (readings === 0) return;
    let alive = true;
    void previewDestinationErasure(id)
      .catch(failed)
      .then((res) => {
        if (!alive) return;
        if (res.ok && res.erasure) setErasure(res.erasure);
        else setProblem(res.error ?? t("settings.error"));
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, readings]);

  function pick(next: How) {
    setHow(next);
    setProblem(null);
    if (next === "delete" && !erasure) setReadings((n) => n + 1);
  }

  function announce(done: string) {
    push(done, "success");
    storageLocationsChanged();
    destinationsChanged();
    offsiteTargetsChanged();
    placementChanged();
    credSetsChanged();
    onRemoved();
  }

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
    announce(t("storage.remove.done").replace("{name}", () => location.name));
  }

  async function erase() {
    if (!erasure) return;
    setBusy(true);
    const named = lastToo ? erasure.lastCopies.map((entry) => entry.identity) : [];
    const res = await eraseDestination(id, erasure.removal, named).catch(failed);
    setBusy(false);
    if (!res.ok) {
      setProblem(placementErrorText(t, lang, res, "settings.error"));
      setLastToo(false);
      if (res.erasure) setErasure(res.erasure);
      else if (res.removal) {
        // The sections changed, so what would be deleted has to be read again.
        setErasure(null);
        setReadings((n) => n + 1);
      }
      return;
    }
    announce(t("storage.remove.erasing").replace("{name}", () => location.name));
  }

  const deleting = how === "delete";
  const confirmed = erasure !== null && erasable(erasure) && typed === location.name && (erasure.lastCopies.length === 0 || lastToo);

  return (
    <InstanceWindow
      title={t("storage.remove.title").replace("{name}", () => location.name)}
      facts={t("dest.removeAsk").replace("{name}", () => location.name)}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />
          {deleting ? (
            <Button
              label={t("storage.remove.deleteAction")}
              labelKey="storage.remove.deleteAction"
              glyph={<IconTrash />}
              tone="accent"
              busy={busy}
              disabled={busy || !confirmed}
              onClick={() => void erase()}
            />
          ) : (
            <Button
              label={t("storage.remove.action")}
              labelKey="storage.remove.action"
              glyph={<IconTrash />}
              tone="accent"
              busy={busy}
              disabled={busy || !removal}
              onClick={() => void remove()}
            />
          )}
        </>
      }
    >
      <div role="radiogroup" aria-label={t("storage.remove.action")} className="flex flex-col gap-3">
        <Way title={t("storage.remove.keep")} text={t("storage.remove.keepText")} picked={!deleting} onPick={() => pick("keep")}>
          {removal ? <DetachedList removal={removal} /> : !problem && <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>}
        </Way>
        <Way title={t("storage.remove.delete")} text={t("storage.remove.deleteText")} picked={deleting} onPick={() => pick("delete")}>
          {erasure ? (
            <ErasureDetails erasure={erasure} name={location.name} typed={typed} onType={setTyped} lastToo={lastToo} onLastToo={setLastToo} />
          ) : (
            !problem && <p className="text-sm text-carbon-textSub">{t("storage.remove.reading")}</p>
          )}
          {erasure && !erasable(erasure) && <p className="text-xs text-statusWarn">{t("placementCode.nothingToErase")}</p>}
        </Way>
      </div>
      {problem && <p className="text-xs text-statusWarn wrap-break-word">{problem}</p>}
    </InstanceWindow>
  );
}
