import type { CSSProperties, ReactNode } from "react";
import type { Provider } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { PROVIDER_MARKS, PROVIDER_TILES } from "../../lib/providerMarks";
import { IconCloud, IconDatabase, IconFolder, IconLocal, IconTabSystem, IconSync } from "../navGlyphs";
import { IconLink, IconShieldOn } from "../glyphs";
import { BrandMark, ReadmeButton } from "../ReadmeButton";

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

/** The colour pair a provider's tile lights up in, its brand's where it has one. */
export function providerTileStyle(mark: string | undefined): CSSProperties | undefined {
  const lit = mark ? PROVIDER_TILES[mark] : undefined;
  return lit && ({ "--provider-tile": lit.tile, "--provider-ink": lit.ink } as CSSProperties);
}

/** Providers as README buttons, each lighting up in its brand's colour. */
export function ProviderTiles({
  providers,
  picked,
  onPick,
  nameOf,
}: {
  providers: Provider[];
  picked?: string;
  onPick: (p: Provider) => void;
  /** A name other than the provider's own, such as a folder on this host. */
  nameOf?: (p: Provider) => string;
}) {
  const { t } = useT();
  return (
    <ul className="glim-provider-pick">
      {providers.map((p) => (
        <li key={p.id} style={providerTileStyle(p.mark)}>
          <ReadmeButton
            tile={`glim-tile-provider${picked === p.id ? " glim-provider-picked" : ""}`}
            parts={[{ name: nameOf ? nameOf(p) : providerName(p, t), onClick: () => onPick(p) }]}
            mark={<ProviderMark provider={p} />}
            markClass="text-carbon-textSub"
          />
        </li>
      ))}
    </ul>
  );
}

const GENERIC: Record<string, TranslationKey> = { path: "dest.provider.path", s3: "dest.provider.s3", smb: "dest.provider.smb" };

/** A provider's name: its own for a product, translated for a generic kind. */
export function providerName(p: Pick<Provider, "id" | "name">, t: ReturnType<typeof useT>["t"]): string {
  return GENERIC[p.id] ? t(GENERIC[p.id]) : p.name;
}
