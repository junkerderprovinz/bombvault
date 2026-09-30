// SettingsSearch is the field above every Settings page. It searches the page
// names, the card titles, the caption beside each control and the text behind
// its (i), in the reader's language, and a result opens its page and marks the
// card or row (see useSearchJump).
import { useId, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { IconSearch } from "../../components/glyphs";
import { IconClose } from "../../components/navGlyphs";
import { Badge } from "../../components/Badge";
import { InfoBubble } from "../../components/InfoBubble";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useRevealOnScrollUp } from "./revealOnScrollUp";
import { SETTINGS_INDEX, type SearchCard } from "./searchIndex";
import type { SettingsPageDef, SettingsPageId } from "./settingsPages";

/** Where a result leads: its page, and the card and row to mark there. */
export interface SearchJump {
  card?: string;
  row?: string;
}

interface Item {
  id: string;
  page: SettingsPageId;
  pageName: string;
  name: string;
  cardName?: string;
  /** Lower tiers sort first where the match is equally good. */
  tier: number;
  foldedName: string;
  foldedProse: string;
}

interface Hit {
  item: Item;
  score: number;
  inHint: boolean;
}

/** How many results are drawn; the count read out covers all of them. */
const SHOWN = 40;
/** Added to a match found only in explanations, so every name match ranks first. */
const PROSE = 1000;

/** fold lowercases and drops accents, so "zeitplane" finds "Zeitpläne". */
export function fold(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").replace(/ß/g, "ss").toLowerCase();
}

export function cardName(t: (k: TranslationKey) => string, card: SearchCard): string {
  let name = t(card.title);
  for (const [k, v] of Object.entries(card.vars ?? {})) name = name.replace(`{${k}}`, t(v));
  return name;
}

export function buildItems(t: (k: TranslationKey) => string, pages: SettingsPageDef[]): Item[] {
  const items: Item[] = [];
  for (const p of pages) {
    const pageName = t(p.label);
    const add = (item: Omit<Item, "page" | "pageName" | "foldedName">) =>
      items.push({ ...item, page: p.id, pageName, foldedName: fold(item.name) });
    add({ id: `page:${p.id}`, name: pageName, tier: 0, foldedProse: "" });
    for (const card of SETTINGS_INDEX[p.id]) {
      const title = cardName(t, card);
      const prose = [card.hint, ...(card.body ?? [])].map((k) => (k ? t(k) : "")).join(" ");
      add({ id: `card:${p.id}:${title}`, name: title, tier: 1, foldedProse: fold(prose) });
      for (const row of card.rows) {
        add({
          id: `row:${p.id}:${title}:${row.key}`,
          name: t(row.key),
          cardName: title,
          tier: 2,
          foldedProse: row.hint ? fold(t(row.hint)) : "",
        });
      }
    }
  }
  return items;
}

/**
 * search ranks every item whose name holds all the words, a word at the start
 * of the name ranking higher, and after them the items whose explanation
 * holds them.
 */
export function search<T extends { foldedName: string; foldedProse: string; tier: number }>(
  items: T[],
  query: string,
): { item: T; score: number; inHint: boolean }[] {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) return [];
  const hits: { item: T; score: number; inHint: boolean }[] = [];
  items.forEach((item, order) => {
    const inName = words.every((w) => item.foldedName.includes(w));
    const inHint = !inName && words.every((w) => item.foldedName.includes(w) || item.foldedProse.includes(w));
    if (!inName && !inHint) return;
    const early = words.filter((w) => item.foldedName.startsWith(w) || item.foldedName.includes(` ${w}`)).length;
    hits.push({ item, inHint, score: (inHint ? PROSE : 0) - early * 10 + item.tier + order / 1e6 });
  });
  return hits.sort((a, b) => a.score - b.score);
}

export function SettingsSearch({ pages }: { pages: SettingsPageDef[] }) {
  const { t } = useT();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const field = useRef<HTMLInputElement>(null);
  const listId = useId();
  // Pinned while in use, so a wheel tick cannot take it from somebody typing.
  const pinned =
    open || query !== "" || (typeof document !== "undefined" && document.activeElement === field.current);
  const { pathname } = useLocation();
  const { revealed, barRef } = useRevealOnScrollUp(pathname, pinned);

  const items = useMemo(() => buildItems(t, pages), [t, pages]);
  const hits: Hit[] = useMemo(() => search(items, query), [items, query]);
  const shown = hits.slice(0, SHOWN);
  const showList = open && query.trim() !== "";

  function pick(hit: Hit) {
    const { item } = hit;
    const jump: SearchJump =
      item.tier === 0 ? {} : item.tier === 1 ? { card: item.name } : { card: item.cardName, row: item.name };
    setQuery("");
    setOpen(false);
    field.current?.blur();
    navigate(`/settings/${item.page}`, { state: { jump } });
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Escape") {
      if (query) {
        e.preventDefault();
        setQuery("");
      }
      return;
    }
    if (!showList || shown.length === 0) return;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((a) => (a + step + shown.length) % shown.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      pick(shown[Math.min(active, shown.length - 1)]);
    }
  }

  // Results stay grouped by page, in rank order within each page.
  const groups: { page: SettingsPageId; name: string; hits: { hit: Hit; index: number }[] }[] = [];
  shown.forEach((hit, index) => {
    let group = groups.find((g) => g.page === hit.item.page);
    if (!group) {
      group = { page: hit.item.page, name: hit.item.pageName, hits: [] };
      groups.push(group);
    }
    group.hits.push({ hit, index });
  });

  // Not rendered until summoned, so it is out of the tab order and hidden from
  // screen readers.
  if (!revealed) return null;

  return (
    <div ref={barRef} data-settings-search className="relative">
      <div className="flex h-(--btn-h) items-center gap-2 rounded-control bg-carbon-surface2 ps-3 pe-1 glim-field-focus-within">
        <span className="shrink-0 text-carbon-textMuted [&_svg]:h-4 [&_svg]:w-4">
          <IconSearch />
        </span>
        <input
          ref={field}
          type="search"
          role="combobox"
          aria-expanded={showList}
          aria-controls={listId}
          aria-activedescendant={showList && shown.length > 0 ? `${listId}-${Math.min(active, shown.length - 1)}` : undefined}
          aria-label={t("settings.search.placeholder")}
          placeholder={t("settings.search.placeholder")}
          value={query}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(0);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}
          onKeyDown={onKeyDown}
          className="h-full min-w-0 flex-1 bg-transparent text-sm text-carbon-text outline-none placeholder:text-carbon-textMuted [&::-webkit-search-cancel-button]:hidden"
        />
        {query && (
          <Badge
            as="button"
            shape="square"
            size="icon"
            tone="neutral"
            tip={t("settings.search.clear")}
            onClick={() => {
              setQuery("");
              field.current?.focus();
            }}
          >
            <IconClose />
          </Badge>
        )}
        <InfoBubble tip={t("settings.search.hint")} />
      </div>
      {showList && (
        <div
          id={listId}
          role="listbox"
          aria-label={t("settings.search.results")}
          // A press on a result would blur the field and close the list first.
          onMouseDown={(e) => e.preventDefault()}
          className="absolute inset-x-0 top-full z-30 mt-2 max-h-[60vh] overflow-y-auto rounded-card bg-carbon-surface py-1.5 shadow-xl glim-fade"
        >
          {shown.length === 0 && (
            <p className="px-4 py-6 text-center text-xs text-carbon-textMuted">{t("settings.search.noMatch")}</p>
          )}
          {groups.map((g) => (
            <div key={g.page} role="group" aria-label={g.name} className="py-1">
              <div className="px-4 pb-1 text-[11px] font-semibold uppercase tracking-wide text-carbon-textMuted">
                {g.name}
              </div>
              {g.hits.map(({ hit, index }) => (
                <div
                  key={hit.item.id}
                  id={`${listId}-${index}`}
                  role="option"
                  aria-selected={index === active}
                  onMouseEnter={() => setActive(index)}
                  onClick={() => pick(hit)}
                  className={`flex cursor-pointer flex-col gap-0.5 px-4 py-2 text-sm ${
                    index === active ? "bg-carbon-hover text-carbon-text" : "text-carbon-textSub"
                  }`}
                >
                  <span className="truncate">{hit.item.name}</span>
                  {(hit.item.cardName || hit.inHint || hit.item.tier === 0) && (
                    <span className="flex min-w-0 gap-2 text-[11px] text-carbon-textMuted">
                      <span className="truncate">
                        {hit.item.tier === 0 ? t("settings.search.page") : hit.item.cardName}
                      </span>
                      {hit.inHint && <span className="shrink-0">{t("settings.search.inHint")}</span>}
                    </span>
                  )}
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
      <span aria-live="polite" className="sr-only">
        {showList
          ? hits.length === 1
            ? t("settings.search.countOne")
            : t("settings.search.count").replace("{n}", String(hits.length))
          : ""}
      </span>
    </div>
  );
}

/**
 * jumpTarget finds what a result leads to on the open page: the row inside its
 * card, or the card itself when the row has no search mark (only ToggleRow
 * carries one) or is hidden.
 */
export function jumpTarget(root: ParentNode, jump: SearchJump): HTMLElement | null {
  const within = (scope: ParentNode, attr: string, name: string | undefined) =>
    name ? Array.from(scope.querySelectorAll<HTMLElement>(`[${attr}]`)).find((el) => el.getAttribute(attr) === name) : undefined;
  const card = within(root, "data-search-card", jump.card);
  const row = within(card ?? root, "data-search-row", jump.row);
  return row ?? card ?? null;
}

/** markHit rings the element for a moment, so the eye finds it after the scroll. */
export function markHit(el: HTMLElement): void {
  el.classList.remove("glim-search-hit");
  // A second jump to the same element restarts the animation.
  void el.offsetWidth;
  el.classList.add("glim-search-hit");
  // A card's own entrance animation ends inside it too, so only this one counts.
  const done = (e: AnimationEvent) => {
    if (e.animationName !== "glim-search-hit") return;
    el.classList.remove("glim-search-hit");
    el.removeEventListener("animationend", done);
  };
  el.addEventListener("animationend", done);
}
