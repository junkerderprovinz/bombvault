import type { useT } from "../lib/i18n";
import { Badge } from "./Badge";
import { InfoBubble } from "./InfoBubble";

type T = ReturnType<typeof useT>["t"];

interface NotInstalledHeadingProps {
  /** The page's own explanation, already translated (containers and VMs word it differently). */
  tip: string;
  /** The page's `nextHue()`, so this notch never shares a colour with another on the page. */
  hueIndex: number;
  t: T;
}

// The heading over the "Not installed (backups only)" list on the Containers
// and VMs pages. The explanation sits in the badge's (i), as in Recovery.tsx's
// card-less group headings, because the notch is pulled up by half its height
// and would cover a line under the heading. The h2 is `relative` because
// nothing else here anchors the notch.
export function NotInstalledHeading({ tip, hueIndex, t }: NotInstalledHeadingProps) {
  return (
    <h2 className="relative flex items-center">
      <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
        {t("containers.notInstalledTitle")}
        <InfoBubble tip={tip} onAccent />
      </Badge>
    </h2>
  );
}
