import { useEffect, useRef, useState, type ReactNode } from "react";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// WhatsNewDialog (#48) — a "What's new" modal shown once when a NEW BombVault
// version is running since this browser last opened the app. It fetches the
// GitHub release notes for the running version and renders them with a tiny,
// dependency-free Markdown renderer (headings / bold / bullet lists / links /
// horizontal rules). On any fetch failure it degrades to a short message plus
// the "View on GitHub" link. Version-change detection + when to mount this lives
// in app/Layout.tsx; this component just renders + fetches once it is shown.
// ---------------------------------------------------------------------------

const REPO = "junkerderprovinz/bombvault";
const RELEASES_PAGE = `https://github.com/${REPO}/releases`;

interface ReleaseInfo {
  body: string;
  htmlUrl: string;
}

// Only http(s)/mailto links are rendered as anchors; anything else (e.g. a
// javascript: URI slipped into a release body) falls back to plain text.
function safeHref(url: string): string | null {
  const u = url.trim();
  return /^(https?:|mailto:)/i.test(u) ? u : null;
}

// Inline formatter: turns **bold** and [text](url) into React nodes; all other
// text stays as (auto-escaped) plain strings. Deliberately minimal — no library.
function renderInline(text: string, keyBase: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  const re = /\[([^\]]+)\]\(([^)]+)\)|\*\*([^*]+)\*\*/g;
  let last = 0;
  let i = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) nodes.push(text.slice(last, m.index));
    if (m[0].startsWith("[")) {
      const href = safeHref(m[2]);
      if (href) {
        nodes.push(
          <a
            key={`${keyBase}-a${i}`}
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent underline hover:no-underline"
          >
            {m[1]}
          </a>
        );
      } else {
        nodes.push(m[1]);
      }
    } else {
      nodes.push(
        <strong key={`${keyBase}-b${i}`} className="font-semibold text-carbon-text">
          {m[3]}
        </strong>
      );
    }
    last = re.lastIndex;
    i++;
  }
  if (last < text.length) nodes.push(text.slice(last));
  return nodes;
}

// Block-level renderer: splits into lines, groups consecutive bullet lines into
// a list, and maps headings / rules / paragraphs to Carbon-styled elements.
function renderMarkdown(md: string): ReactNode[] {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let list: string[] = [];
  let key = 0;

  const flushList = () => {
    if (list.length === 0) return;
    const items = list;
    const k = key++;
    blocks.push(
      <ul key={`ul${k}`} className="my-2 ml-5 list-disc space-y-1 text-sm leading-relaxed text-carbon-textSub">
        {items.map((it, idx) => (
          <li key={idx}>{renderInline(it, `ul${k}-${idx}`)}</li>
        ))}
      </ul>
    );
    list = [];
  };

  for (const raw of lines) {
    const line = raw.trim();
    if (line === "") {
      flushList();
      continue;
    }
    // Horizontal rule: --- *** ___
    if (/^([-*_])\1{2,}$/.test(line)) {
      flushList();
      blocks.push(<hr key={`hr${key++}`} className="my-4 border-carbon-border" />);
      continue;
    }
    // Headings: # … ###### (## and higher → larger; ### and deeper → smaller)
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    if (h) {
      flushList();
      const k = key++;
      if (h[1].length <= 2) {
        blocks.push(
          <h3 key={`h${k}`} className="mb-2 mt-4 text-base font-semibold text-carbon-text first:mt-0">
            {renderInline(h[2], `h${k}`)}
          </h3>
        );
      } else {
        blocks.push(
          <h4 key={`h${k}`} className="mb-1 mt-3 text-sm font-semibold text-carbon-textSub">
            {renderInline(h[2], `h${k}`)}
          </h4>
        );
      }
      continue;
    }
    // Bullet list item: "- " or "* "
    const bullet = line.match(/^[-*]\s+(.*)$/);
    if (bullet) {
      list.push(bullet[1]);
      continue;
    }
    // Everything else → paragraph
    flushList();
    const k = key++;
    blocks.push(
      <p key={`p${k}`} className="my-2 text-sm leading-relaxed text-carbon-textSub">
        {renderInline(line, `p${k}`)}
      </p>
    );
  }
  flushList();
  return blocks;
}

export function WhatsNewDialog({ version, onClose }: { version: string; onClose: () => void }) {
  const { t } = useT();
  const [state, setState] = useState<"loading" | "ok" | "error">("loading");
  const [info, setInfo] = useState<ReleaseInfo | null>(null);
  const closeRef = useRef<HTMLButtonElement>(null);

  const tagUrl = `${RELEASES_PAGE}/tag/${encodeURIComponent(version)}`;

  // Fetch the notes from BombVault's OWN backend, not api.github.com — the app's
  // Content-Security-Policy (connect-src 'self') blocks the cross-origin call, so
  // the dialog always failed (#54). The backend serves its embedded release notes
  // same-origin; the GitHub releases page stays only as the "view full" link.
  useEffect(() => {
    let active = true;
    setState("loading");
    fetch(`/api/release-notes?version=${encodeURIComponent(version)}`, {
      headers: { Accept: "application/json" },
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((data: { ok?: boolean; body?: string; htmlUrl?: string }) => {
        if (!active) return;
        if (!data.ok || !data.body) {
          setState("error");
          return;
        }
        setInfo({ body: data.body.trim(), htmlUrl: data.htmlUrl ?? tagUrl });
        setState("ok");
      })
      .catch(() => {
        if (active) setState("error");
      });
    return () => {
      active = false;
    };
  }, [version, tagUrl]);

  // Focus the close button on open + dismiss on Escape.
  useEffect(() => {
    closeRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const fullUrl = info?.htmlUrl ?? tagUrl;

  return (
    <div
      className="bv-modal-backdrop fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="whatsnew-title"
        className="bv-modal-card flex max-h-[85vh] w-full max-w-lg flex-col rounded-card border border-carbon-border bg-carbon-surface shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 border-b border-carbon-border px-5 py-4">
          <h2 id="whatsnew-title" className="text-lg font-semibold text-carbon-text">
            {t("whatsnew.title").replace("{version}", version)}
          </h2>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            aria-label={t("whatsnew.close")}
            className="shrink-0 rounded p-1 text-carbon-textMuted hover:bg-carbon-hover hover:text-carbon-text"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {/* Body (scrolls) */}
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {state === "loading" && (
            <div className="flex items-center gap-3 py-6 text-sm text-carbon-textSub">
              <span className="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-carbon-surface3 border-t-transparent" />
              {t("whatsnew.loading")}
            </div>
          )}
          {state === "error" && (
            <p className="py-4 text-sm text-carbon-textSub">{t("whatsnew.loadFailed")}</p>
          )}
          {state === "ok" &&
            (info && info.body ? (
              <div>{renderMarkdown(info.body)}</div>
            ) : (
              <p className="py-4 text-sm text-carbon-textSub">{t("whatsnew.loadFailed")}</p>
            ))}
        </div>

        {/* Footer — the prominent "view full release" link is always present. */}
        <div className="flex items-center justify-between gap-3 border-t border-carbon-border px-5 py-4">
          <a
            href={fullUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium text-accent underline hover:no-underline"
          >
            {t("whatsnew.viewOnGitHub")}
          </a>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg bg-carbon-surface3 px-4 py-2 text-sm font-medium text-carbon-text hover:bg-carbon-hover"
          >
            {t("whatsnew.close")}
          </button>
        </div>
      </div>
    </div>
  );
}
