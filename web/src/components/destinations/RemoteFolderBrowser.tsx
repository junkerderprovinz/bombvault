import { useState } from "react";
import { draftFolders, draftMakeFolder } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconBack } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconClose, IconFolder } from "../navGlyphs";
import { FIELD } from "./steps";

function split(path: string): string[] {
  return path.split("/").filter(Boolean);
}

/**
 * RemoteFolderBrowser is the folder field of a bucket or an rclone remote: the
 * path can be typed, or picked in a window that walks the remote's folders and
 * makes a new one. The remote is reached with the connection as it stands in
 * the form, so a saved secret has to be typed again before the walk works.
 */
export function RemoteFolderBrowser({
  label,
  hint,
  value,
  onChange,
  provider,
  settings,
  needsSecret,
}: {
  label: string;
  hint: string;
  value: string;
  onChange: (next: string) => void;
  provider: string;
  settings: Record<string, string>;
  /** A saved secret is not in the form, so a failed walk says to type it. */
  needsSecret: boolean;
}) {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const [dir, setDir] = useState<string[]>([]);
  const [folders, setFolders] = useState<string[] | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);

  async function list(path: string[]) {
    setProblem(null);
    setFolders(null);
    const res = await draftFolders({ provider, settings, dir: path.join("/") }).catch((e: unknown) => ({
      ok: false as const,
      error: e instanceof Error ? e.message : undefined,
      folders: undefined,
    }));
    if (!res.ok) {
      const reason = res.error ?? t("folder.couldNotRead");
      setProblem(needsSecret ? `${reason} ${t("storage.conn.browseNeedsSecret")}` : reason);
      setFolders([]);
      return;
    }
    setDir(path);
    setFolders(res.folders ?? []);
  }

  function browse() {
    setOpen(true);
    void list(split(value));
  }

  async function create() {
    const name = newName.trim();
    if (!name || creating) return;
    setCreating(true);
    const res = await draftMakeFolder({ provider, settings, dir: dir.join("/"), folder: name }).catch((e: unknown) => ({
      ok: false as const,
      error: e instanceof Error ? e.message : undefined,
    }));
    setCreating(false);
    if (!res.ok) {
      setProblem(res.error ?? t("folder.createFailed"));
      return;
    }
    setNewName("");
    void list([...dir, name]);
  }

  return (
    // bv-convention-exception: setting-row-beside-label: a folder browser keeps its name above it, like FolderBrowser.
    <div className="flex items-end gap-2 max-md:flex-wrap">
      <label className="flex min-w-0 flex-1 flex-col gap-1 max-md:basis-full">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {label}
          <InfoBubble tip={hint} />
        </span>
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          autoComplete="off"
          spellCheck={false}
          dir="ltr"
          className={`${FIELD} font-mono text-start`}
        />
      </label>
      <Button
        label={t("folder.browseTitle")}
        labelKey="folder.browseTitle"
        glyph={<IconFolder />}
        tone="subtle"
        onClick={browse}
        className="glim-btn-wrap shrink-0"
      />
      {open && (
        <InstanceWindow
          title={label}
          onClose={() => setOpen(false)}
          footer={
            <>
              <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="neutral" onClick={() => setOpen(false)} />
              <Button
                label={t("folder.use")}
                labelKey="folder.use"
                glyph={<IconCheck />}
                tone="accent"
                disabled={folders === null}
                onClick={() => {
                  onChange(dir.join("/"));
                  setOpen(false);
                }}
              />
            </>
          }
        >
          <p dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textSub">
            {provider}:/{dir.join("/")}
          </p>
          {problem && <p className="text-xs text-statusFail wrap-break-word">{problem}</p>}
          {folders === null ? (
            <p className="text-xs text-carbon-textMuted">{t("folder.loading")}</p>
          ) : (
            <div className="flex flex-col gap-0.5">
              {dir.length > 0 && (
                <Button
                  // The file-manager convention, not a phrase to translate.
                  label=".."
                  labelKey={null}
                  glyph={<IconBack />}
                  tone="neutral"
                  keepLabel
                  onClick={() => void list(dir.slice(0, -1))}
                  className="w-full !justify-start min-h-8"
                />
              )}
              {folders.length === 0 && !problem && <p className="px-2 text-xs text-carbon-textMuted">{t("folder.none")}</p>}
              {folders.map((f) => (
                <Button
                  key={f}
                  label={f}
                  // A folder name is data, not a translation key.
                  labelKey={null}
                  glyph={<IconFolder />}
                  tone="neutral"
                  keepLabel
                  onClick={() => void list([...dir, f])}
                  className="w-full !justify-start min-h-8"
                />
              ))}
            </div>
          )}
          <span className="flex items-center gap-2">
            <input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  void create();
                }
              }}
              aria-label={t("folder.newFolderPlaceholder")}
              placeholder={t("folder.newFolderPlaceholder")}
              spellCheck={false}
              dir="ltr"
              className={`${FIELD} min-w-0 flex-1 font-mono text-start`}
            />
            <Button
              label={t("folder.newFolder")}
              labelKey="folder.newFolder"
              tone="neutral"
              disabled={creating || newName.trim() === "" || folders === null}
              busy={creating}
              onClick={() => void create()}
              className="shrink-0"
            />
          </span>
        </InstanceWindow>
      )}
    </div>
  );
}
