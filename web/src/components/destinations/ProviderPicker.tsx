import type { CSSProperties, ReactNode } from "react";
import type { Provider, ProviderGroup } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { PROVIDER_MARKS, PROVIDER_TILES } from "../../lib/providerMarks";
import { Badge } from "../Badge";
import { IconCloud, IconDatabase, IconFolder, IconLocal, IconTabSystem, IconSync } from "../navGlyphs";
import { IconLink, IconShieldOn } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { BrandMark, ReadmeButton } from "../ReadmeButton";

const GROUPS: { id: ProviderGroup; title: TranslationKey; fit: TranslationKey; tip: TranslationKey; tone: "ok" | "warn" }[] = [
  { id: "storage", title: "dest.group.storage", fit: "dest.fit.best", tip: "dest.groupTip.storage", tone: "ok" },
  { id: "selfs3", title: "dest.group.selfs3", fit: "dest.fit.good", tip: "dest.groupTip.selfs3", tone: "ok" },
  { id: "server", title: "dest.group.server", fit: "dest.fit.good", tip: "dest.groupTip.server", tone: "ok" },
  { id: "cloud", title: "dest.group.cloud", fit: "dest.fit.slower", tip: "dest.groupTip.cloud", tone: "warn" },
];

// The house glyphs for what has no brand of its own.
const HOUSE: Record<string, () => ReactNode> = {
  IconShield: () => <IconShieldOn />,
  IconServer: () => <IconTabSystem />,
  IconFolder: () => <IconFolder />,
  IconLink: () => <IconLink />,
  IconTransfer: () => <IconSync />,
  IconDrive: () => <IconLocal />,
  IconBuckets: () => <IconDatabase />,
};

/** The mark a provider is drawn with, a generic cloud where it has none. */
export function ProviderMark({ provider }: { provider: Pick<Provider, "mark"> }) {
  const mark = provider.mark ?? "";
  if (PROVIDER_MARKS[mark]) return <BrandMark svg={PROVIDER_MARKS[mark]} />;
  return <>{(HOUSE[mark] ?? (() => <IconCloud />))()}</>;
}

/**
 * The providers in their four groups, each as a README button that lights up
 * in its brand's colour. A group's heading says how well it suits backups.
 */
export function ProviderPicker({
  providers,
  picked,
  onPick,
}: {
  providers: Provider[];
  picked?: string;
  onPick: (p: Provider) => void;
}) {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-4">
      {GROUPS.map((g) => {
        const inGroup = providers.filter((p) => p.group === g.id);
        if (inGroup.length === 0) return null;
        return (
          <section key={g.id} className="flex flex-col gap-2">
            <h4 className="flex flex-wrap items-center gap-2 text-sm font-semibold text-carbon-text">
              {t(g.title)}
              <Badge tone={g.tone} size="small">
                {t(g.fit)}
              </Badge>
              <InfoBubble tip={t(g.tip)} />
            </h4>
            <ul className="glim-provider-pick">
              {inGroup.map((p) => {
                const lit = p.mark ? PROVIDER_TILES[p.mark] : undefined;
                return (
                  <li
                    key={p.id}
                    style={lit && ({ "--provider-tile": lit.tile, "--provider-ink": lit.ink } as CSSProperties)}
                  >
                    <ReadmeButton
                      tile={`glim-tile-provider${picked === p.id ? " glim-provider-picked" : ""}`}
                      parts={[{ name: providerName(p, t), onClick: () => onPick(p) }]}
                      mark={<ProviderMark provider={p} />}
                      markClass="text-carbon-textSub"
                    />
                  </li>
                );
              })}
            </ul>
          </section>
        );
      })}
    </div>
  );
}

const GENERIC: Record<string, TranslationKey> = { path: "dest.provider.path", s3: "dest.provider.s3", smb: "dest.provider.smb" };

/** A provider's name: its own for a product, translated for a generic kind. */
export function providerName(p: Pick<Provider, "id" | "name">, t: ReturnType<typeof useT>["t"]): string {
  return GENERIC[p.id] ? t(GENERIC[p.id]) : p.name;
}
