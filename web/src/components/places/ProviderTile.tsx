import { useId, type CSSProperties } from "react";
import { Badge } from "../Badge";
import { PlaceMark, placeTile } from "../placeMarks";
import { hueVars } from "../../lib/appearance";
import { useGridNav } from "../../lib/gridNav";
import { useT, type TranslationKey } from "../../lib/i18n";
import { mergeRefs } from "../../lib/mergeRefs";
import { providerName } from "../../lib/placeText";
import type { CatalogProvider, PlaceGroup } from "../../lib/places";
import { useLabelMode } from "../../lib/useLabelMode";
import { useTipBubble } from "../../lib/useTipBubble";

// The provider tiles of the add window. A tile is the crypto window's coin
// tile (.glim-coin-tile), with a fixed size in place of aspect-square because
// provider names run longer than tickers: every tile is the same 7rem square,
// the mark is 48px and the name takes at most two lines of 12px. Under the
// pointer a tile lights up in its brand's colour, as the coin tiles do, in one
// frame, since a fade would leave its mark trailing. One without a brand mark
// has no colour to light up in and climbs the ramp.

const GROUPS: { id: PlaceGroup; key: TranslationKey }[] = [
  { id: "cloud", key: "places.group.cloud" },
  { id: "self", key: "places.group.self" },
  { id: "here", key: "places.group.here" },
];

export function ProviderTile({
  provider,
  hueIndex,
  selected,
  onPick,
  nav,
}: {
  provider: CatalogProvider;
  hueIndex: number;
  /** The tile the form was opened from, marked when the window comes back to the tiles. */
  selected: boolean;
  onPick: () => void;
  /** The grid's roving tab stop, from useGridNav's tileProps. */
  nav: { ref: (node: HTMLElement | null) => void; tabIndex: number; onFocus: () => void };
}) {
  const { t } = useT();
  // Reactive mode shows mark and name at rest: someone looking for a
  // provider should not have to hover every tile to read it.
  const mode = useLabelMode("buttons");
  const showMark = mode !== "text";
  const showName = mode !== "glyph";
  const name = providerName(t, provider.id);
  const tip = useTipBubble(showName ? undefined : name);
  const lit = placeTile(provider.id);
  let look = "bg-carbon-surface2 text-carbon-textSub transition-colors hover:bg-carbon-surface3";
  if (selected) look = "glim-active bg-accent text-accentContrast transition-colors";
  else if (lit) look = "glim-brand-tile bg-carbon-surface2 text-carbon-textSub";

  return (
    <>
      <button
        ref={mergeRefs(nav.ref, tip.ref)}
        type="button"
        role="option"
        aria-selected={selected}
        aria-label={name}
        aria-describedby={tip.describedBy}
        tabIndex={nav.tabIndex}
        onClick={onPick}
        onFocus={() => {
          nav.onFocus();
          tip.handlers.onFocus();
        }}
        onBlur={tip.handlers.onBlur}
        onMouseEnter={tip.handlers.onMouseEnter}
        onMouseLeave={tip.handlers.onMouseLeave}
        style={{ ...hueVars(hueIndex), ...(lit && { "--tile": lit.color, "--tile-ink": lit.ink }) } as CSSProperties}
        className={`glim-coin-tile glim-hue flex h-28 w-28 shrink-0 flex-col items-center justify-center gap-2 rounded-control px-2 max-md:aspect-square max-md:h-auto max-md:w-full ${look}`}
      >
        {showMark && <PlaceMark provider={provider.id} size={48} />}
        {showName && <span className="line-clamp-2 text-center text-xs font-medium leading-tight">{name}</span>}
      </button>
      {tip.bubble}
    </>
  );
}

/**
 * ProviderGrid is every provider in its group, in catalog order, as one
 * listbox with one tab stop. The arrow keys cross from one group into the next.
 */
export function ProviderGrid({
  providers,
  selected,
  onPick,
}: {
  providers: CatalogProvider[];
  selected: string | null;
  onPick: (provider: CatalogProvider) => void;
}) {
  const { t } = useT();
  const headingBase = useId();
  const ordered = GROUPS.flatMap((g) => providers.filter((p) => p.group === g.id));
  const nav = useGridNav(
    ordered.length,
    ordered.findIndex((p) => p.id === selected)
  );

  return (
    <div role="listbox" aria-label={t("places.pick")} onKeyDown={nav.onKeyDown} className="flex flex-col gap-6">
      {GROUPS.map((group, gi) => {
        const members = ordered.filter((p) => p.group === group.id);
        if (members.length === 0) return null;
        const headingId = `${headingBase}-${group.id}`;
        return (
          <div key={group.id} role="group" aria-labelledby={headingId} className="flex flex-col gap-3">
            <h3 id={headingId} className="flex items-center">
              <Badge tone="heading" size="heading" inFlow hueIndex={gi}>
                {t(group.key)}
              </Badge>
            </h3>
            {/* Columns as wide as a tile, so every group lines up under the
                one above, and the spare width split on both sides. On a phone
                two 7rem columns leave a wide margin, so there the tiles
                shrink until a third fits and fill the row. */}
            <div className="grid grid-cols-[repeat(auto-fill,7rem)] justify-center gap-2 max-md:grid-cols-[repeat(auto-fill,minmax(5.5rem,1fr))]">
              {members.map((p) => {
                const i = ordered.indexOf(p);
                return (
                  <ProviderTile
                    key={p.id}
                    provider={p}
                    hueIndex={i}
                    selected={p.id === selected}
                    onPick={() => onPick(p)}
                    nav={nav.tileProps(i)}
                  />
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
