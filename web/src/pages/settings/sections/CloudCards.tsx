import { RcloneCard } from "../RcloneCard";
import { useT } from "../../../lib/i18n";
import { SignInsCard } from "../SignInsCard";

/** CloudCards hold what reaches a cloud: the rclone remotes and the saved
 *  sign-ins. */
export function CloudCards({ nextHue }: { nextHue: () => number }) {
  const { t } = useT();

  return (
    <div id="cloud" className="flex scroll-mt-6 flex-col gap-6 md:gap-10">
      <RcloneCard t={t} hueIndex={nextHue()} />
      <SignInsCard hueIndex={nextHue()} />
    </div>
  );
}
