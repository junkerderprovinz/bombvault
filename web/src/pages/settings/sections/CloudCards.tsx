import { RcloneCard } from "../RcloneCard";
import { CloudCard } from "../CloudCard";
import { useT } from "../../../lib/i18n";
import { CloudCredSetsCard } from "../CloudCredSetsCard";

/** CloudCards hold what reaches a cloud: the rclone remotes, the shared
 *  credentials and the named sets. */
export function CloudCards({ nextHue }: { nextHue: () => number }) {
  const { t } = useT();

  return (
    <div id="cloud" className="flex scroll-mt-6 flex-col gap-6 md:gap-10">
      <RcloneCard t={t} hueIndex={nextHue()} />

      <CloudCard t={t} hueIndex={nextHue()} />
      <CloudCredSetsCard t={t} hueIndex={nextHue()} />
    </div>
  );
}
