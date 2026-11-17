import { IconFileImage } from "../../../components/navGlyphs";
import { useT } from "../../../lib/i18n";
import { PageLinkCard, SidebarCard } from "../SidebarTile";
import { SnapshotPlansCard } from "../SnapshotPlansCard";
import { hueCounter } from "../shared";

export function SnapshotsPage() {
  const { t } = useT();
  const nextHue = hueCounter();
  return (
    <>
      <SidebarCard pin="snapshots" hueIndex={nextHue()} />
      <SnapshotPlansCard hueIndex={nextHue()} />
      <PageLinkCard to="/snapshots" label={t("hostSnap.title")} sub={t("hostSnap.link.sub")} glyph={<IconFileImage />} hueIndex={nextHue()} />
    </>
  );
}
