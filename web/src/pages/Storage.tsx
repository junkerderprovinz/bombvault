// The Storage locations page: every place backups are kept, each with how full
// it is, which section is backed up to which of them, and what applies to the
// copies to all of them.
import { PageTitle } from "../components/PageTitle";
import { LocationCards } from "../components/storage/LocationCards";
import { StreamingCard } from "../components/storage/StreamingCard";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useStorageLocations } from "../lib/useStorageLocations";
import { hueCounter } from "./settings/shared";

export function StoragePage() {
  const { t } = useT();
  const list = useStorageLocations();
  const nextHue = hueCounter();

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("storage.title")}</PageTitle>
      <LocationCards list={list} nextHue={nextHue} newId="storage-list" />
      <StreamingCard hueIndex={nextHue()} />
    </div>
  );
}
