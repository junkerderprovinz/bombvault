import { useState, type ReactNode } from "react";
import type { FileSetPresetResponse } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { ToggleRow } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { createPlacedFileSet } from "../files/createPlacedFileSet";
import { FolderBrowser } from "../FolderBrowser";
import { InfoBubble } from "../InfoBubble";
import { IconAdd, IconCheck, IconFolder } from "../navGlyphs";
import { PlacementDraft, type PlacementDraftValue } from "../placement/PlacementDraft";

/** A folder the server offers for a new set. `path` is relative to the host
 *  mount root, as a set stores it. */
export interface FoundFolder {
  name: string;
  path: string;
  /** Why the folder is offered, where its name does not say it. */
  hint?: string;
}

/** What FolderAddForm hands to the window it stands in. */
export interface FolderAddParts {
  body: ReactNode;
  cancel: ReactNode;
  forward: ReactNode;
}

// A set's name becomes a restic tag, which takes letters, digits, dot,
// underscore and hyphen and has to start with a letter or digit.
function nameFor(folder: string): string {
  const clean = folder.replace(/[^A-Za-z0-9._-]+/g, "-").replace(/^[^A-Za-z0-9]+/, "");
  return clean.charAt(0).toUpperCase() + clean.slice(1);
}

const FIELD = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus";

/**
 * FolderAddForm creates a folder set: what the server found to choose from,
 * and the name, folder, schedule, storage location and exclude patterns of
 * the new set. The window around it is the caller's.
 */
export function FolderAddForm({
  found,
  foundUnder,
  taken,
  preset,
  hostMountRoot,
  hostRoot,
  onClose,
  onAdded,
  children,
}: {
  found: FoundFolder[];
  /** The folder the found ones were read from, relative like their paths. */
  foundUnder: string;
  /** The folders a set backs up already. */
  taken: ReadonlySet<string>;
  /** The system configuration, where the platform has one to suggest. */
  preset: FileSetPresetResponse | null;
  hostMountRoot: string;
  /** What the host calls the folder mounted at hostMountRoot. */
  hostRoot: string;
  onClose: () => void;
  onAdded: () => void;
  children: (parts: FolderAddParts) => ReactNode;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [excludes, setExcludes] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [placement, setPlacement] = useState<PlacementDraftValue>({});
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);

  const offered: FoundFolder[] = [
    ...found,
    ...(preset?.offered ? [{ name: t("backups.add.systemConfig"), path: preset.path, hint: t("files.addPresetHint") }] : []),
  ];

  function choose(folder: FoundFolder) {
    const fromPreset = preset?.offered && folder.path === preset.path;
    setName(fromPreset ? preset.name : nameFor(folder.name));
    setPath(folder.path);
    if (fromPreset) setExcludes(preset.excludes.join("\n"));
  }

  async function create() {
    setSaving(true);
    try {
      const res = await createPlacedFileSet(
        {
          name: name.trim(),
          path: path.trim(),
          excludes: excludes
            .split("\n")
            .map((line) => line.trim())
            .filter((line) => line !== ""),
          enabled,
        },
        placement,
        (repo) => setPlacement((prev) => ({ ...prev, home: { repo } })),
        (refused) => placementErrorText(t, lang, refused, "settings.error")
      );
      if (res.ok) {
        push(t("backups.add.created").replace("{name}", name.trim()), "success");
        onAdded();
        onClose();
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

  const body = (
    <>
      {offered.length > 0 && (
        <section className="flex flex-col gap-2">
          <h3 className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
            {t("backups.add.found")}
            {found.length > 0 && <InfoBubble tip={t("backups.add.foundHint").replace("{path}", `${hostRoot}/${foundUnder}`)} />}
          </h3>
          <ul data-testid="found-folders" className="max-h-44 overflow-y-auto">
            {offered.map((folder) => {
              const has = taken.has(folder.path);
              const on = path === folder.path;
              return (
                <li key={folder.path} className="flex items-center gap-2 border-t border-carbon-border first:border-t-0">
                  <button
                    type="button"
                    disabled={has}
                    aria-pressed={on}
                    onClick={() => choose(folder)}
                    className={`flex min-h-11 min-w-0 flex-1 items-center gap-3 rounded-control px-1.5 py-1.5 text-start hover:bg-carbon-hover disabled:opacity-55 disabled:hover:bg-transparent ${
                      on ? "bg-accentSoft" : ""
                    }`}
                  >
                    <span className="flex flex-none text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
                      <IconFolder />
                    </span>
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{folder.name}</span>
                      <span dir="ltr" className="text-start text-xs text-carbon-textMuted wrap-anywhere">
                        {`${hostRoot}/${folder.path}`}
                      </span>
                    </span>
                    {has ? (
                      <Badge tone="neutral">{t("backups.add.taken")}</Badge>
                    ) : (
                      on && (
                        <span className="flex flex-none text-accentText">
                          <IconCheck />
                        </span>
                      )
                    )}
                  </button>
                  {folder.hint && <InfoBubble tip={folder.hint} />}
                </li>
              );
            })}
          </ul>
        </section>
      )}

      <div className="flex flex-col divide-y divide-carbon-border/60 [&>*]:py-3 [&>*:first-child]:pt-0 [&>*:last-child]:pb-0">
        <label className="flex flex-wrap items-center justify-between gap-3">
          <span className="flex items-center gap-1.5 text-sm text-carbon-text">
            {t("files.name")}
            <InfoBubble tip={t("backups.add.nameHint")} />
          </span>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("backups.add.namePlaceholder")}
            spellCheck={false}
            autoComplete="off"
            className={`w-56 max-w-full ${FIELD}`}
          />
        </label>

        <FolderBrowser inDialog label={t("files.path")} value={path} hostMountRoot={hostMountRoot} onChange={setPath} hint={t("files.pathHint")} />

        <ToggleRow
          checked={enabled}
          onChange={setEnabled}
          label={t("entry.settings.inSchedule")}
          hint={t("schedule.pausedHint")
            .replace("{everything}", t("settings.everythingName"))
            .replace("{backupNow}", t("containers.backupNow"))}
        />

        <PlacementDraft value={placement} onChange={setPlacement} />

        <label className="flex flex-col gap-1.5">
          <span className="flex items-center gap-1.5 text-sm text-carbon-text">
            {t("files.excludes")}
            <InfoBubble tip={t("files.excludesHint")} />
          </span>
          <textarea
            value={excludes}
            onChange={(e) => setExcludes(e.target.value)}
            spellCheck={false}
            rows={3}
            placeholder={"*.tmp\ncache/"}
            dir="ltr"
            className={`font-mono text-start ${FIELD}`}
          />
        </label>
      </div>
    </>
  );

  const cancel = <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} disabled={saving} />;
  const forward = (
    <Button
      key={shake}
      label={t("dest.folder.create")}
      labelKey={null}
      glyph={<IconAdd />}
      tone="accent"
      onClick={() => void create()}
      disabled={saving || name.trim() === "" || path.trim() === ""}
      busy={saving}
      className={shake ? "glim-shake" : ""}
    />
  );

  return children({ body, cancel, forward });
}
