import { useEffect, useId, useState } from "react";

import { getSizeBreakdown, type BreakdownDomain, type SizeBreakdown as Breakdown } from "../lib/api";
import { humanBytes } from "../lib/forecast";
import type { useT } from "../lib/i18n";
import { Button } from "./Button";
import { IconDisclosure } from "./IconDisclosure";
import { InfoBubble } from "./InfoBubble";
import { IconFolder } from "./Sidebar";

type T = ReturnType<typeof useT>["t"];

// How often an open panel asks again while the breakdown is being worked out.
const POLL_MS = 2000;

/**
 * SizeBreakdown shows which folders and files take the space in an item's
 * newest backup and how much of each the latest backup added. Opening it is
 * what asks the server to work it out; a folder row opens one level further.
 */
export function SizeBreakdown({ domain, item, t }: { domain: BreakdownDomain; item: string; t: T }) {
  const [open, setOpen] = useState(false);
  const [path, setPath] = useState("");
  const [data, setData] = useState<Breakdown | null>(null);
  // null while the request went through; otherwise the server's reason, empty
  // when the request itself failed.
  const [failure, setFailure] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const panelId = useId();

  useEffect(() => {
    if (!open) return;
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const load = (again: boolean) => {
      getSizeBreakdown(domain, item, path, again)
        .then((res) => {
          if (!alive) return;
          if (!res.ok || !res.breakdown) {
            setFailure(res.error ?? "");
            return;
          }
          setFailure(null);
          setData(res.breakdown);
          if (res.breakdown.state === "running") timer = setTimeout(() => load(false), POLL_MS);
        })
        .catch(() => {
          if (alive) setFailure("");
        });
    };
    load(retry > 0);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [open, domain, item, path, retry]);

  const failed = failure ?? (data?.state === "failed" ? (data.error ?? "") : null);
  const crumbs = path === "" ? [] : path.split("/");
  return (
    <div className="py-2 border-b border-carbon-border flex flex-col gap-2">
      <div className="flex items-center gap-1.5">
        <Button
          label={t("breakdown.title")}
          labelKey="breakdown.title"
          glyph={<IconDisclosure open={open} />}
          tone="subtle"
          onClick={() => setOpen((o) => !o)}
          ariaExpanded={open}
          ariaControls={panelId}
          keepLabel
        />
        <InfoBubble tip={t("breakdown.hint")} />
      </div>
      {open && (
        <div id={panelId} className="flex flex-col gap-2 text-sm">
          {failed !== null && (
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-caption text-statusFail">{t("breakdown.failed").replace("{error}", failed)}</p>
              <Button
                label={t("breakdown.retry")}
                labelKey="breakdown.retry"
                tone="neutral"
                onClick={() => setRetry((n) => n + 1)}
              />
            </div>
          )}
          {failed === null && (!data || data.state === "running") && (
            <p role="status" className="text-caption text-carbon-textMuted">
              {t("breakdown.running")}
            </p>
          )}
          {failed === null && data?.state === "none" && (
            <p className="text-caption text-carbon-textMuted">{t("breakdown.none")}</p>
          )}
          {failed === null && data?.state === "ready" && (
            <>
              <nav aria-label={t("breakdown.title")} className="flex flex-wrap items-center gap-1">
                <Button
                  label={data.root ?? ""}
                  labelKey={null}
                  glyph={<IconFolder />}
                  tone="subtle"
                  onClick={() => setPath("")}
                  disabled={path === ""}
                  keepLabel
                />
                {crumbs.map((c, i) => (
                  <Button
                    key={crumbs.slice(0, i + 1).join("/")}
                    label={c}
                    labelKey={null}
                    glyph={<IconFolder />}
                    tone="subtle"
                    onClick={() => setPath(crumbs.slice(0, i + 1).join("/"))}
                    disabled={i === crumbs.length - 1}
                    keepLabel
                  />
                ))}
              </nav>
              <p className="text-caption text-carbon-textSub">
                {t("breakdown.total")
                  .replace("{size}", humanBytes(data.size))
                  .replace("{files}", t("breakdown.files", data.files))
                  .replace("{added}", humanBytes(data.added))}
                {data.time && <> {t("breakdown.from").replace("{time}", new Date(data.time).toLocaleString())}</>}
              </p>
              {data.first && <p className="text-caption text-carbon-textMuted">{t("breakdown.first")}</p>}
              {data.partial && <p className="text-caption text-statusWarn">{t("breakdown.partial")}</p>}
              <BreakdownRows data={data} onOpen={(name) => setPath(path === "" ? name : `${path}/${name}`)} t={t} />
            </>
          )}
        </div>
      )}
    </div>
  );
}

function BreakdownRows({ data, onOpen, t }: { data: Breakdown; onOpen: (name: string) => void; t: T }) {
  const largest = Math.max(1, ...data.children.map((c) => c.size), data.other?.size ?? 0);
  return (
    <ul className="flex flex-col gap-1.5">
      {data.children.map((c) => (
        <li key={c.name} className="flex flex-col gap-0.5">
          <div className="flex flex-wrap items-center gap-x-2">
            {c.open ? (
              <Button
                label={c.name}
                labelKey={null}
                glyph={<IconFolder />}
                tone="subtle"
                onClick={() => onOpen(c.name)}
                keepLabel
                className="min-w-[8rem] !justify-start"
              />
            ) : (
              <span dir="ltr" className="min-w-[8rem] flex-1 truncate font-mono text-caption text-carbon-text text-start">
                {c.name}
              </span>
            )}
            <span className="ms-auto shrink-0 tabular-nums text-caption text-carbon-text">{humanBytes(c.size)}</span>
            {c.added > 0 && (
              <span className="shrink-0 tabular-nums text-caption text-accentText">
                {t("breakdown.new").replace("{size}", humanBytes(c.added))}
              </span>
            )}
          </div>
          <SizeBar size={c.size} added={c.added} largest={largest} />
        </li>
      ))}
      {data.other && (
        <li className="flex flex-col gap-0.5">
          <div className="flex items-center gap-2 text-caption text-carbon-textMuted">
            <span>{t("breakdown.more", data.other.count)}</span>
            <span className="ms-auto tabular-nums">{humanBytes(data.other.size)}</span>
            {data.other.added > 0 && (
              <span className="tabular-nums text-accentText">
                {t("breakdown.new").replace("{size}", humanBytes(data.other.added))}
              </span>
            )}
          </div>
          <SizeBar size={data.other.size} added={data.other.added} largest={largest} />
        </li>
      )}
    </ul>
  );
}

// SizeBar draws a row's share of the largest row, with the part the latest
// backup changed in the accent at its start. It repeats the figures beside it,
// so screen readers skip it.
function SizeBar({ size, added, largest }: { size: number; added: number; largest: number }) {
  const width = (100 * size) / largest;
  const addedWidth = size > 0 ? (100 * Math.min(added, size)) / size : 0;
  return (
    <div aria-hidden className="h-1.5 rounded-pill bg-carbon-surface2 overflow-hidden">
      <div className="h-full rounded-pill bg-carbon-textMuted flex overflow-hidden" style={{ width: `${width}%` }}>
        <div className="h-full bg-accent" style={{ width: `${addedWidth}%` }} />
      </div>
    </div>
  );
}
