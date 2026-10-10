import { RcloneCard } from "../RcloneCard";
import { CloudCard } from "../CloudCard";
import { useT } from "../../../lib/i18n";
import { hueCounter } from "../shared";
import { CloudCredSetsCard } from "../CloudCredSetsCard";

export function CloudPage() {
  const { t } = useT();

  const nextHue = hueCounter();

  return (
    <>
      <RcloneCard t={t} hueIndex={nextHue()} />

      <CloudCard t={t} hueIndex={nextHue()} />
      <CloudCredSetsCard t={t} hueIndex={nextHue()} />
    </>
  );
}
