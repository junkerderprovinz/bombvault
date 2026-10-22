import { useEffect, useState, type ReactNode } from "react";
import {
  eraseLocation,
  getSettings,
  previewLocationErasure,
  previewLocationRemoval,
  removeLocation,
  type LocationErasure,
  type LocationRemoval,
  type OkEnvelope,
  type StorageLocation,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { secretsSentence } from "../../lib/secretsRefusal";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { credSetsChanged } from "../../lib/useCloudCredSets";
import { destinationsChanged } from "../../lib/useDestinations";
import { reposChanged } from "../../lib/useNamedRepos";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { RadioMark } from "../RadioMark";
import { IconClose, IconTrash } from "../navGlyphs";
import { DetachedList, ErasureDetails, erasable } from "./removalParts";

type Answer = OkEnvelope & { removal?: LocationRemoval; erasure?: LocationErasure };

function failed(e: unknown): Answer {
  return { ok: false, error: e instanceof Error ? e.message : undefined };
}

/** The sentence for a refusal over the login. The shared one for a missing
 *  password speaks of keys, so deleting has its own. */
function refusalSentence(t: (key: TranslationKey) => string, code: string | undefined): string | undefined {
  return code === "password-not-set" ? t("storage.remove.passwordNeeded") : secretsSentence(t, code);
}

function trimmedFolder(path: string): string {
  return path.trim().replace(/\/+$/, "");
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
 * RemovalWindow takes a storage location away in one of two ways: it only
 * detaches the sections and leaves the backups there, or it deletes the
 * backups first. A folder is the home of its sections, so either way asks
 * for the folder they move to. The server acts only on what was shown: when
 * the sections or the entries that lose their only backup changed in between,
 * it answers with them as they are, and the window asks again.
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
  const id = location.id;
  const homes = location.object === "path";
  const [how, setHow] = useState<How>("keep");
  const [removal, setRemoval] = useState<LocationRemoval | null>(null);
  // Counts the readings of what would be detached, so a failed one is read
  // again when its way is picked.
  const [removalReads, setRemovalReads] = useState(0);
  const [erasure, setErasure] = useState<LocationErasure | null>(null);
  // Counts the readings of what would be deleted. None is taken before the
  // way is picked, because a reading lists every repository at the location.
  const [readings, setReadings] = useState(0);
  const [home, setHome] = useState("");
  const [mountRoot, setMountRoot] = useState("");
  const [typed, setTyped] = useState("");
  const [lastToo, setLastToo] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    void previewLocationRemoval(id)
      .catch(failed)
      .then((res) => {
        if (!alive) return;
        if (res.ok && res.removal) {
          const suggest = res.removal.suggest ?? "";
          setRemoval(res.removal);
          setHome((typedHome) => typedHome || suggest);
        } else setProblem(placementErrorText(t, lang, res, "settings.error"));
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, removalReads]);

  useEffect(() => {
    if (readings === 0) return;
    let alive = true;
    void previewLocationErasure(id)
      .catch(failed)
      .then((res) => {
        if (!alive) return;
        if (res.ok && res.erasure) setErasure(res.erasure);
        else setProblem(refusalSentence(t, res.code) ?? res.error ?? t("settings.error"));
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, readings]);

  useEffect(() => {
    if (!homes) return;
    let alive = true;
    void getSettings()
      .then((res) => {
        if (alive && res.ok) setMountRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [homes]);

  function pick(next: How) {
    setHow(next);
    setProblem(null);
    if (next === "delete" && !erasure) setReadings((n) => n + 1);
    if (next === "keep" && !removal) setRemovalReads((n) => n + 1);
  }

  function announce(done: string) {
    push(done, "success");
    storageLocationsChanged();
    destinationsChanged();
    offsiteTargetsChanged();
    reposChanged();
    placementChanged();
    credSetsChanged();
    // A folder's sections and a repository's defaults live in the settings.
    window.dispatchEvent(new Event("bv:settings-changed"));
    onRemoved();
  }

  const newHome = trimmedFolder(home);
  const sentHome = homes ? newHome : undefined;
  const homeReady = !homes || (newHome !== "" && newHome !== trimmedFolder(location.where));
  // A section's own folder put into a storage location keeps it until the own
  // folder moves on, which the location's page does.
  const ownHere = !homes && removal ? removal.sections.filter((section) => section.use === "home") : [];

  async function remove() {
    if (!removal) return;
    setBusy(true);
    const res = await removeLocation(id, removal, sentHome).catch(failed);
    setBusy(false);
    if (!res.ok) {
      if (res.removal) setRemoval(res.removal);
      else if (res.code === "stale") setRemovalReads((n) => n + 1);
      setProblem(placementErrorText(t, lang, res, "settings.error"));
      return;
    }
    announce(t("storage.remove.done").replace("{name}", () => location.name));
  }

  async function erase() {
    if (!erasure) return;
    setBusy(true);
    const named = lastToo ? erasure.lastCopies.map((entry) => entry.identity) : [];
    const res = await eraseLocation(id, erasure.removal, named, sentHome).catch(failed);
    setBusy(false);
    if (!res.ok) {
      setProblem(refusalSentence(t, res.code) ?? placementErrorText(t, lang, res, "settings.error"));
      setLastToo(false);
      if (res.erasure) setErasure(res.erasure);
      else if (res.removal || res.code === "stale") {
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
      facts={homes ? undefined : t("dest.removeAsk").replace("{name}", () => location.name)}
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
              disabled={busy || !confirmed || !homeReady || ownHere.length > 0}
              onClick={() => void erase()}
            />
          ) : (
            <Button
              label={t("storage.remove.action")}
              labelKey="storage.remove.action"
              glyph={<IconTrash />}
              tone="accent"
              busy={busy}
              disabled={busy || !removal || !homeReady || ownHere.length > 0}
              onClick={() => void remove()}
            />
          )}
        </>
      }
    >
      {homes && (
        <FolderBrowser
          label={t("storage.remove.home")}
          hint={t("storage.remove.homeTip")}
          value={home}
          hostMountRoot={mountRoot}
          onChange={setHome}
          inDialog
        />
      )}
      {ownHere.length > 0 ? (
        <div role="alert" className="flex flex-col gap-1 rounded-card bg-statusWarnBg px-3 py-2 text-sm leading-relaxed text-statusWarn wrap-break-word">
          <p>{t("placementCode.ownFolderThere")}</p>
          <p className="font-semibold">{formatList(lang, ownHere.map((section) => t(DOMAIN_LABEL[section.domain])))}</p>
        </div>
      ) : (
        <div role="radiogroup" aria-label={t("storage.remove.action")} className="flex flex-col gap-3">
          <Way
            title={t("storage.remove.keep")}
            text={t(homes ? "storage.remove.keepHome" : "storage.remove.keepText")}
            picked={!deleting}
            onPick={() => pick("keep")}
          >
            {removal ? <DetachedList removal={removal} home={newHome} /> : !problem && <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>}
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
      )}
      {problem && <p className="text-xs text-statusWarn wrap-break-word">{problem}</p>}
    </InstanceWindow>
  );
}
