// The search for a file over every backup: a name goes in, and every backup
// holding a file of that name comes out, each a click away from the restore
// window.
import { useEffect, useRef, useState, type CSSProperties, type FormEvent } from "react";

import { searchFiles, type BackupItem, type FileSearchResponse, type FoundFile, type SearchedPlace } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { kindOfDomain } from "../../lib/backupEntry";
import { repoDisplayName } from "../../lib/directRepo";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";
import { useLocalNames } from "../../lib/useLocalName";
import { Card, ToggleRow } from "../../pages/settings/shared";
import { KIND_LABEL, KindGlyph, entryName, kindHue } from "../backups/kinds";
import { Button } from "../Button";
import { fileKindGlyph } from "../fileKindGlyph";
import { IconSearch, IconStop } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { Rows } from "../storage/rows";
import type { RestoreAsk } from "./RestoreWindow";

/** How many backups of a file show before the rest are asked for. */
const VERSIONS_SHOWN = 4;

const PLACE_STATE: Record<Exclude<SearchedPlace["state"], "searched">, TranslationKey> = {
  covered: "fileSearch.place.covered",
  notAsked: "fileSearch.place.notAsked",
  failed: "fileSearch.place.failed",
};

function unix(time: string): number {
  return Date.parse(time) / 1000;
}

/** PlaceList says what the search read where. */
function PlaceList({ places }: { places: SearchedPlace[] }) {
  const { t } = useT();
  const localName = useLocalNames();
  return (
    <ul aria-label={t("fileSearch.places")} className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
      {places.map((p, i) => (
        <li key={i} className="flex min-w-0 items-center gap-1.5">
          <span className="font-semibold text-carbon-text wrap-anywhere">
            {repoDisplayName(t, p.label, p.directOf) || localName(p.domain)}
          </span>
          <span className={p.state === "failed" ? "text-statusWarn" : "text-carbon-textMuted"}>
            {p.state === "searched" ? t("fileSearch.place.searched", p.backups) : t(PLACE_STATE[p.state])}
          </span>
          {p.error && <InfoBubble tip={p.error} />}
        </li>
      ))}
    </ul>
  );
}

/** FoundRow is one file with the backups that hold it, newest first. */
function FoundRow({
  file,
  item,
  places,
  onOpen,
}: {
  file: FoundFile;
  item: BackupItem;
  places: SearchedPlace[];
  onOpen: (item: BackupItem, ask: RestoreAsk) => void;
}) {
  const { t, lang } = useT();
  const localName = useLocalNames();
  const [all, setAll] = useState(false);
  const shown = all ? file.versions : file.versions.slice(0, VERSIONS_SHOWN);
  const hidden = file.versions.length - shown.length;
  const placeName = (at: number) => {
    const p = places[at];
    return p ? repoDisplayName(t, p.label, p.directOf) || localName(p.domain) : "";
  };
  return (
    <li data-testid="found-file" className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0">
      <div className="flex min-w-0 items-start gap-3">
        <span
          style={hueVars(kindHue(item.kind)) as CSSProperties}
          className="glim-hue flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]"
        >
          <KindGlyph kind={item.kind} />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex flex-wrap items-baseline gap-x-2">
            <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{entryName(item, t)}</span>
            <span className="text-xs text-carbon-textMuted">{t(KIND_LABEL[item.kind])}</span>
          </span>
          <span dir="ltr" className="flex min-w-0 items-start gap-1.5 text-start text-xs text-carbon-textSub">
            <span className="mt-px flex flex-none text-carbon-textMuted [&>svg]:size-3.5" aria-hidden="true">
              {fileKindGlyph(file.path, file.type === "dir")}
            </span>
            <span className="font-mono wrap-anywhere">{file.dataset ? `${file.dataset}${file.path}` : file.path}</span>
          </span>
          {item.kind === "flash" && <span className="text-xs text-carbon-textMuted">{t("fileSearch.flash")}</span>}
        </span>
      </div>
      <ul className="flex flex-col ps-10 max-sm:ps-0">
        {shown.map((v, i) => {
          const older = file.versions[i + 1];
          const unchanged = older !== undefined && older.size === v.size && older.modified === v.modified;
          return (
            <li key={`${v.row}-${i}`}>
              <button
                type="button"
                onClick={() =>
                  onOpen(item, {
                    snapshot: v.snapshot,
                    at: unix(v.time),
                    place: v.place,
                    path: file.path,
                    ...(file.dataset ? { dataset: file.dataset } : {}),
                  })
                }
                className="flex w-full flex-wrap items-baseline gap-x-3 gap-y-0.5 rounded-control px-2 py-1.5 text-start transition-colors hover:bg-carbon-surface2 glim-field-focus"
              >
                <span className="text-sm text-carbon-text">{formatRecent(unix(v.time), lang)}</span>
                <span className="text-sm tabular-nums text-carbon-textSub">{file.type === "dir" ? "" : humanBytes(v.size)}</span>
                <span className="text-xs text-carbon-textMuted wrap-anywhere">{placeName(v.at)}</span>
                {unchanged && <span className="text-xs text-carbon-textMuted">{t("fileSearch.unchanged")}</span>}
              </button>
            </li>
          );
        })}
      </ul>
      {(hidden > 0 || all) && file.versions.length > VERSIONS_SHOWN && (
        <span className="flex ps-12 max-sm:ps-2">
          <Button
            label={all ? t("fileSearch.fewer") : t("fileSearch.allVersions", hidden)}
            labelKey={null}
            tone="subtle"
            keepLabel
            ariaExpanded={all}
            onClick={() => setAll(!all)}
          />
        </span>
      )}
    </li>
  );
}

/**
 * FileSearch is the Find a file card of the Restore page. A search reads
 * every backup, which takes a while, so it runs only when asked and can be
 * stopped. Leaving the page stops it as well.
 */
export function FileSearch({
  items,
  hueIndex,
  onOpen,
}: {
  items: BackupItem[];
  hueIndex?: number;
  onOpen: (item: BackupItem, ask: RestoreAsk) => void;
}) {
  const { t } = useT();
  const [name, setName] = useState("");
  const [offsite, setOffsite] = useState(false);
  const [result, setResult] = useState<FileSearchResponse | null>(null);
  const [failed, setFailed] = useState<string | null>(null);
  const [slash, setSlash] = useState(false);
  const running = useRef<AbortController | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => () => running.current?.abort(), []);

  function stop() {
    running.current?.abort();
    running.current = null;
    setBusy(false);
  }

  async function start(e?: FormEvent) {
    e?.preventDefault();
    const text = name.trim();
    if (text === "") return;
    setSlash(text.includes("/"));
    if (text.includes("/")) return;
    stop();
    const ctl = new AbortController();
    running.current = ctl;
    setBusy(true);
    setFailed(null);
    setResult(null);
    try {
      const res = await searchFiles(text, offsite, ctl.signal);
      if (ctl.signal.aborted) return;
      if (res.ok) setResult(res);
      else setFailed(res.error || t("fileSearch.failed"));
    } catch (err) {
      if (!ctl.signal.aborted) setFailed(err instanceof Error && err.message ? err.message : t("fileSearch.failed"));
    } finally {
      if (running.current === ctl) {
        running.current = null;
        setBusy(false);
      }
    }
  }

  const vmsSaved = items.some((item) => item.kind === "vm" && item.lastBackup > 0);
  const files = result?.files ?? [];
  const places = result?.places ?? [];
  const itemOf = (file: FoundFile) => {
    const kind = kindOfDomain(file.domain);
    return items.find((item) => item.kind === kind && item.key === file.key);
  };

  return (
    <Card title={t("fileSearch.title")} hint={t("fileSearch.hint")} hueIndex={hueIndex} newId="restore-file-search">
      <form className="flex items-center gap-2" onSubmit={(e) => void start(e)}>
        <label className="flex h-11 min-w-0 flex-1 items-center gap-2.5 rounded-control bg-carbon-surface2 px-3.5 glim-field-focus-within">
          <span className="shrink-0 text-carbon-textMuted [&_svg]:h-[18px] [&_svg]:w-[18px]">
            <IconSearch />
          </span>
          <input
            type="search"
            aria-label={t("fileSearch.placeholder")}
            placeholder={t("fileSearch.placeholder")}
            value={name}
            spellCheck={false}
            autoComplete="off"
            maxLength={255}
            dir="auto"
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape" && busy) {
                e.stopPropagation();
                stop();
              }
            }}
            className="min-w-0 flex-1 bg-transparent text-base text-carbon-text outline-none placeholder:text-carbon-textMuted"
          />
        </label>
        {busy ? (
          <Button label={t("fileSearch.stop")} labelKey={null} glyph={<IconStop />} tone="subtle" onClick={stop} />
        ) : (
          <Button
            label={t("fileSearch.search")}
            labelKey={null}
            glyph={<IconSearch />}
            tone="accent"
            type="submit"
            disabled={name.trim() === ""}
          />
        )}
      </form>
      <ToggleRow label={t("fileSearch.offsite")} hint={t("fileSearch.offsiteHint")} checked={offsite} onChange={setOffsite} />
      {slash && <p className="text-sm text-statusWarn">{t("fileSearch.noSlash")}</p>}
      {busy && (
        <p role="status" className="text-sm text-carbon-textMuted">
          {t("fileSearch.running")}
        </p>
      )}
      {failed !== null && <p className="text-sm text-statusFail">{failed}</p>}
      {result && (
        <>
          <PlaceList places={places} />
          {files.length === 0 ? (
            <p className="text-sm text-carbon-textMuted">{t("fileSearch.none")}</p>
          ) : (
            <Rows as="ul">
              {files.map((file) => {
                const item = itemOf(file);
                return item ? (
                  <FoundRow
                    key={`${file.domain}/${file.key}/${file.dataset ?? ""}/${file.path}`}
                    file={file}
                    item={item}
                    places={places}
                    onOpen={onOpen}
                  />
                ) : null;
              })}
            </Rows>
          )}
          {result.more && <p className="text-sm text-carbon-textMuted">{t("fileSearch.more").replace("{n}", String(files.length))}</p>}
        </>
      )}
      {vmsSaved && <p className="text-xs text-carbon-textMuted">{t("fileSearch.vms")}</p>}
    </Card>
  );
}
