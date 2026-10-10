import { useT } from "../../../lib/i18n";
import { hueCounter } from "../shared";
import { ParleyPortCard, PhoneAppCard, WidgetAppCard } from "../AppsCards";

export function AppsPage() {
  const { t } = useT();

  const nextHue = hueCounter();

  return (
    <>
      <PhoneAppCard t={t} hueIndex={nextHue()} />
      <ParleyPortCard t={t} hueIndex={nextHue()} />
      <WidgetAppCard t={t} hueIndex={nextHue()} />
    </>
  );
}
