import { useState } from "react";
import { createPortal } from "react-dom";
import { createFileSet, createDirectRepo, patchFileSet } from "../../lib/api";
import type { FileSetView, OkEnvelope } from "../../lib/api";
import { PlacementDraft, type PlacementDraftValue } from "../placement/PlacementDraft";
import { placementErrorText } from "../../lib/placementCodes";
import { reposChanged } from "../../lib/useNamedRepos";
import { FolderBrowser } from "../FolderBrowser";
import { useT } from "../../lib/i18n";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { ToggleRow } from "../../pages/settings/shared";
import { useToast } from "../../lib/toast";

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

  // A remembered direct repository is made before the set. If the set then
  // fails, the draft keeps the new id so a second try does not make another.
  async function createSet(excludes: string[]): Promise<OkEnvelope & { id?: string }> {
    let home = placement.home;
    if (home && "direct" in home) {
      const made = await createDirectRepo(home.direct.targetId, home.direct.name, home.direct.location);
      if (!made.ok || !made.repo) return { ok: false, error: placementErrorText(t, lang, made, "settings.error") };
      reposChanged();
      const created = { repo: made.repo.id };
      setPlacement((prev) => ({ ...prev, home: created }));
      home = created;
    }
    return createFileSet({
      name: name.trim(),
      path: path.trim(),
      excludes,
      enabled,
      ...(home ? { repo: home.repo } : {}),
      ...(placement.copies ? { copies: placement.copies } : {}),
    });
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

  // Portal to <body> so no ancestor's CSS transform can trap the fixed
  // overlay. Centred rather than top-anchored, which would push the heading
  // notch against the viewport edge; the box is capped at 90vh, so it never
  // clips.
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={onClose}
    >
      {/* The heading notch sits on a non-scrolling shell around the
          scrollable box, as in Receiver.tsx's ReceiverDialog. */}
      <div className="relative w-full max-w-lg">
      {/* px-5 matches the box's p-5 so the notch lands where a Card's does;
          FolderBrowser.tsx explains why the notch has no offset of its own. */}
      <h2 className="flex items-center px-5">
        <Badge tone="heading" size="heading" wrap>{initial ? t("files.editSet") : t("files.addSet")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={initial ? t("files.editSet") : t("files.addSet")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
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
          />
          {/* Saving a new path clears the ticked sub-folder selection on the
              server, so the hint says so beforehand, whether or not a
              selection exists. The tree itself lives on the card. */}
          <p className="text-caption text-carbon-textMuted">{t("files.pathChangeHint")}</p>
          <p className="text-caption text-carbon-textMuted">{t("files.pathHint")}</p>
        </div>

        {/* Exclude patterns, one per line */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("files.excludes")}</label>
          <textarea
            value={excludesText}
            onChange={(e) => setExcludesText(e.target.value)}
            spellCheck={false}
            rows={4}
            placeholder={"*.tmp\ncache/"}
            dir="ltr"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
          />
          <p className="text-caption text-carbon-textMuted">{t("files.excludesHint")}</p>
        </div>

        {/* A new set has no card yet, so its placement is chosen here; an
            existing set changes it on its card. */}
        {!initial && <PlacementDraft value={placement} onChange={setPlacement} />}

        {/* ToggleRow, not a bare Toggle: every setting row in this app puts
            the words at the start and the switch at the end. */}
        <ToggleRow checked={enabled} onChange={setEnabled} label={t("files.enabled")} />

        <div className="flex items-center justify-end gap-2 pt-1">
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
        </div>
      </div>
      </div>
    </div>,
    document.body,
  );
}
