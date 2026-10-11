import { useState } from "react";
import {
  deleteBackupsZFSDataset,
  deleteFileSet,
  deleteFileSetBackups,
  deleteZFSDataset,
  type FileSetView,
  type OkEnvelope,
  type ZFSDatasetView,
  type ZFSDeleteResult,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { ToggleRow } from "../../pages/settings/shared";
import { Button } from "../Button";
import { IconTrash } from "../navGlyphs";
import { Choice, Choices } from "./Choice";

type Way = "keep" | "drop";

/** What the window removes: a folder set or a ZFS entry. */
export type Removable = { kind: "files"; set: FileSetView } | { kind: "zfs"; dataset: ZFSDatasetView };

/**
 * RemoveWindow takes an entry off the list in one of two ways: the entry
 * alone, which leaves its backups where they are, or the entry with its
 * backups. An entry without backups has only the first.
 */
export function RemoveWindow({
  target,
  name,
  hasBackups,
  onRemoved,
  onClose,
}: {
  target: Removable;
  name: string;
  hasBackups: boolean;
  /** The entry is gone, so the page has nothing left to show. */
  onRemoved: () => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [way, setWay] = useState<Way>("keep");
  const [safetyToo, setSafetyToo] = useState(false);
  const [busy, setBusy] = useState(false);
  const safety = target.kind === "zfs" ? target.dataset.safetyCount : 0;

  function call(): Promise<OkEnvelope & ZFSDeleteResult> {
    if (target.kind === "files") {
      return way === "keep" ? deleteFileSet(target.set.id) : deleteFileSetBackups(target.set.id);
    }
    return way === "keep" ? deleteZFSDataset(target.dataset.id, safetyToo) : deleteBackupsZFSDataset(target.dataset.id);
  }

  async function remove() {
    setBusy(true);
    try {
      const res = await call();
      if (!res.ok) {
        push(res.error ?? t("common.removeFailed"), "fail");
        return;
      }
      push(t(way === "keep" ? "entry.remove.kept" : "entry.remove.dropped").replace("{name}", () => name), "success");
      if (res.safetyRemaining) push(t("zfs.deleteKeptSafety", res.safetyRemaining), "warn");
      if (res.leftoversRemaining) push(t("zfs.sweepRemaining", res.leftoversRemaining), "warn");
      onRemoved();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.removeFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  const title = t("entry.remove.title").replace("{name}", () => name);
  return (
    <InstanceWindow
      title={title}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} disabled={busy} />
          <Button
            label={t(way === "keep" ? "entry.remove.keepGo" : "entry.remove.dropGo")}
            labelKey={way === "keep" ? "entry.remove.keepGo" : "entry.remove.dropGo"}
            glyph={<IconTrash />}
            tone="accent"
            disabled={busy}
            busy={busy}
            onClick={() => void remove()}
          />
        </>
      }
    >
      <Choices label={title}>
        <Choice
          on={way === "keep"}
          title={t("entry.remove.keep")}
          text={t("entry.remove.keepText").replace("{discover}", t("containers.discover"))}
          onPick={() => setWay("keep")}
        />
        {safety > 0 && way === "keep" && (
          <ToggleRow
            label={t("entry.remove.safety", safety)}
            hint={t("entry.remove.safetyHint")}
            checked={safetyToo}
            onChange={setSafetyToo}
            disabled={busy}
          />
        )}
        {hasBackups && (
          <Choice
            on={way === "drop"}
            title={t("entry.remove.drop")}
            text={t(target.kind === "zfs" ? "entry.remove.dropTextZFS" : "entry.remove.dropTextLocal")}
            onPick={() => setWay("drop")}
          />
        )}
      </Choices>
    </InstanceWindow>
  );
}
