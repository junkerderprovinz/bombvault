import type { TranslationKey, useT } from "../../lib/i18n";
import { Badge } from "../Badge";

// MobileSectionLabel is the heading of a phone card: the same notch every
// card in the app wears, straddling the card's top edge. It is a filled Badge
// rather than a letter-spaced caps label, since capitals and tracking mean
// nothing in the no-case scripts of ar, he, hi, th, zh, ja and ko.
//
// The notch is positioned against the section it sits in, so that section
// has to hug its card with no gap, and the card needs about 20px of top
// padding for the notch's lower half to clear its first line, as the desktop
// cards have. insetStart lines the notch up with the card's p-4 content edge.
// The Badge carries no hueIndex: the section's glim-hue rebinds --accent for
// the whole block, so the fill takes the block's position.
export function MobileSectionLabel({ t, labelKey }: { t: ReturnType<typeof useT>["t"]; labelKey: TranslationKey }) {
  return (
    <h2 className="flex items-center min-w-0">
      <Badge tone="heading" size="heading" wrap insetStart={4} className="min-w-0">
        {t(labelKey)}
      </Badge>
    </h2>
  );
}
