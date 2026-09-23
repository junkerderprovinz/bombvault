import { useEffect, useRef, useState, type ReactNode } from "react";
import { useT } from "../lib/i18n";
import { Badge } from "./Badge";
import { Button } from "./Button";

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

// renderInline turns **bold** and [text](url) into React nodes; everything else
// stays plain text, which React escapes.
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
        // A plain link rather than a Badge, since it sits mid-sentence.
        nodes.push(
          <a
            key={`${keyBase}-a${i}`}
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            // Flat accent gold is 1.61:1 on the light background, too faint for
            // body text; text-accentText meets 4.5:1.
            className="text-accentText underline hover:no-underline"
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

// renderMarkdown handles the subset release notes use: headings, bullet lists,
// horizontal rules and paragraphs.
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
      <ul key={`ul${k}`} className="my-2 ms-5 list-disc space-y-1 text-sm leading-relaxed text-carbon-textSub">
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
    // # and ## render large, deeper headings small.
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
    const bullet = line.match(/^[-*]\s+(.*)$/);
    if (bullet) {
      list.push(bullet[1]);
      continue;
    }
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

// WhatsNewDialog shows the release notes of the running version. app/Layout.tsx
// decides when to mount it.
export function WhatsNewDialog({ version, onClose }: { version: string; onClose: () => void }) {
  const { t } = useT();
  const [state, setState] = useState<"loading" | "ok" | "error">("loading");
  const [info, setInfo] = useState<ReleaseInfo | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const autoTriesRef = useRef(0);
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    autoTriesRef.current = 0;
  }, [version]);

  const tagUrl = `${RELEASES_PAGE}/tag/${encodeURIComponent(version)}`;

  // The notes come from our own backend because the CSP (connect-src 'self')
  // blocks api.github.com.
  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    setState("loading");

    // The first request can land while the container restarts after an update,
    // so a few retries with backoff come before the error and the Retry button.
    function fail() {
      if (!active) return;
      const MAX_AUTO = 3;
      if (autoTriesRef.current < MAX_AUTO) {
        const delay = 600 * 2 ** autoTriesRef.current; // 0.6s → 1.2s → 2.4s
        autoTriesRef.current += 1;
        timer = setTimeout(() => {
          if (active) setReloadKey((k) => k + 1);
        }, delay);
      } else {
        setState("error");
      }
    }

    fetch(`/api/release-notes?version=${encodeURIComponent(version)}`, {
      headers: { Accept: "application/json" },
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((data: { ok?: boolean; body?: string; htmlUrl?: string }) => {
        if (!active) return;
        if (!data.ok || !data.body) {
          fail();
          return;
        }
        setInfo({ body: data.body.trim(), htmlUrl: data.htmlUrl ?? tagUrl });
        setState("ok");
      })
      .catch(() => {
        fail();
      });
    return () => {
      active = false;
      if (timer) clearTimeout(timer);
    };
  }, [version, tagUrl, reloadKey]);

  // Focus the close button on open and close on Escape.
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
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="whatsnew-title"
        className="glim-modal-card relative flex max-h-[85vh] w-full max-w-3xl flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        {/* No dividers between header, body and footer; the padding separates them. */}
        <div className="flex items-start justify-between gap-4 px-5 py-4">
          <h2 id="whatsnew-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{t("whatsnew.title").replace("{version}", version)}</Badge>
          </h2>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {state === "loading" && (
            <div className="flex items-center gap-3 py-6 text-sm text-carbon-textSub">
              <span className="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-carbon-surface3 border-t-transparent" />
              {t("whatsnew.loading")}
            </div>
          )}
          {state === "error" && (
            <div className="py-4">
              <p className="text-sm text-carbon-textSub">{t("whatsnew.loadFailed")}</p>
              <Button
                label={t("whatsnew.retry")}
                labelKey="whatsnew.retry"
                tone="neutral"
                onClick={() => {
                  autoTriesRef.current = 0;
                  setReloadKey((k) => k + 1);
                }}
                className="mt-3"
              />
            </div>
          )}
          {state === "ok" &&
            (info && info.body ? (
              <div>{renderMarkdown(info.body)}</div>
            ) : (
              <p className="py-4 text-sm text-carbon-textSub">{t("whatsnew.loadFailed")}</p>
            ))}
        </div>

        {/* size="large" gives the link the same weight as the Close button. */}
        <div className="flex items-center justify-between gap-3 px-5 py-4">
          <Badge
            as="a"
            href={fullUrl}
            target="_blank"
            rel="noopener noreferrer"
            tone="neutral"
            size="large"
          >
            {t("whatsnew.viewOnGitHub")}
          </Badge>
          <Button
            ref={closeRef}
            label={t("whatsnew.close")}
            labelKey="whatsnew.close"
            tone="neutral"
            onClick={onClose}
          />
        </div>
      </div>
    </div>
  );
}
