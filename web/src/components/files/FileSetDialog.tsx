import { useState } from "react";
import { patchFileSet } from "../../lib/api";
import type { FileSetView, OkEnvelope } from "../../lib/api";
import { PlacementDraft, type PlacementDraftValue } from "../placement/PlacementDraft";
import { placementErrorText } from "../../lib/placementCodes";
import { createPlacedFileSet } from "./createPlacedFileSet";
import { FolderBrowser } from "../FolderBrowser";
import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { ToggleRow } from "../../pages/settings/shared";
import { useToast } from "../../lib/toast";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";

type T = ReturnType<typeof useT>["t"];

// Exported for the page's dom tests, which render it against a mocked client.
export function FileSetDialog({
  initial,
  presetSeed,
  hostMountRoot,
  t,
  onClose,
  onSaved,
}: {
  /** null = create a new set; a view = edit that set. */
  initial: FileSetView | null;
  /** Pre-fill for a new set opened through "Add preset: Host system config",
   *  the counterpart of the flash domain on generic hosts and TrueNAS.
   *  Ignored when editing; every field stays editable. */
  presetSeed: { name: string; path: string; excludes: string[] } | null;
  hostMountRoot: string;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const { lang } = useT();
  const [name, setName] = useState(initial?.name ?? presetSeed?.name ?? "");
  const [path, setPath] = useState(initial?.path ?? presetSeed?.path ?? "");
  const [excludesText, setExcludesText] = useState(
    (initial?.excludes ?? presetSeed?.excludes ?? []).join("\n")
  );
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [placement, setPlacement] = useState<PlacementDraftValue>({});
  // A set with backups keeps its name, since nothing re-tags its snapshots.
  // Locked here as well as refused by the server, so the reason shows before
  // the attempt.
  const hasBackups = Boolean(initial) && (initial?.lastBackup ?? 0) > 0;
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);

  const canSave = name.trim() !== "" && path.trim() !== "" && !saving;

  function createSet(excludes: string[]): Promise<OkEnvelope & { id?: string }> {
    return createPlacedFileSet(
      { name: name.trim(), path: path.trim(), excludes, enabled },
      placement,
      (repo) => setPlacement((prev) => ({ ...prev, home: { repo } })),
      (refused) => placementErrorText(t, lang, refused, "settings.error")
    );
  }

  // The dialog closes on success, so every outcome is reported as a toast.
  async function handleSave() {
    if (!canSave) return;
    setSaving(true);
    const excludes = excludesText
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "");
    try {
      const res = initial
        ? await patchFileSet(initial.id, { name: name.trim(), path: path.trim(), excludes, enabled })
        : await createSet(excludes);
      if (res.ok) {
        push(t("settings.saved"), "success");
        onSaved();
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  return (
    <InstanceWindow
      title={initial ? t("files.editSet") : t("files.addSet")}
      onClose={onClose}
      footer={
        <>
          <Button
            label={t("files.cancel")}
            labelKey="files.cancel"
            tone="neutral"
            onClick={onClose}
            disabled={saving}
          />
          <Button
            key={shake}
            label={t("settings.save")}
            labelKey="settings.save"
            tone="accent"
            onClick={() => void handleSave()}
            disabled={!canSave}
            busy={saving}
            title={saving ? t("common.saving") : undefined}
            className={shake ? "glim-shake" : ""}
          />
        </>
      }
    >
      {/* The name becomes a restic tag, so the server validates it strictly. */}
      <div className="flex flex-col gap-1.5">
        <label className="flex items-center gap-1 text-xs text-carbon-textSub">
          {t("files.name")}
          {hasBackups && <InfoBubble tip={t("files.nameLocked")} />}
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={hasBackups}
          spellCheck={false}
          autoComplete="off"
          placeholder="documents"
          className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus disabled:opacity-50"
        />
      </div>

      {/* Source folder (relative subpath under the host mount root) */}
      <div className="flex flex-col gap-1.5">
        <FolderBrowser
          inDialog
          label={t("files.path")}
          value={path}
          hostMountRoot={hostMountRoot}
          onChange={setPath}
          hint={`${t("files.pathHint")} ${t("files.pathChangeHint")}`}
        />
      </div>

      {/* Exclude patterns, one per line */}
      <div className="flex flex-col gap-1.5">
        <label className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("files.excludes")}
          <InfoBubble tip={t("files.excludesHint")} />
        </label>
        <textarea
          value={excludesText}
          onChange={(e) => setExcludesText(e.target.value)}
          spellCheck={false}
          rows={4}
          placeholder={"*.tmp\ncache/"}
          dir="ltr"
          className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
        />
      </div>

      {/* A new set has no card yet, so its placement is chosen here; an
          existing set changes it on its card. */}
      {!initial && <PlacementDraft value={placement} onChange={setPlacement} />}

      {/* ToggleRow, not a bare Toggle: every setting row in this app puts
          the words at the start and the switch at the end. */}
      <ToggleRow checked={enabled} onChange={setEnabled} label={t("files.enabled")} />
    </InstanceWindow>
  );
}
