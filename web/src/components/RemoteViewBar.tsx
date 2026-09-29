// The bar remote view shows above every page while a paired member's
// instance is open, so what is on screen is never mistaken for this one's own
// data. Sticky at the top of the scroller, in the accent colour so it reads
// as a mode rather than a card among the others.

import { useT } from "../lib/i18n";
import { useInstanceScope } from "../lib/instanceScope";
import { Button } from "./Button";

export function RemoteViewBar() {
  const { t } = useT();
  const { remote, instanceName, leave } = useInstanceScope();
  if (!remote) return null;
  return (
    <div className="sticky top-0 z-20 -mx-4 mb-4 flex flex-wrap items-center gap-2 bg-accent px-4 py-2 text-sm text-accentContrast md:-mx-6">
      <span className="font-medium">{t("remoteView.viewing").replace("{name}", instanceName || t("remoteView.unnamed"))}</span>
      <Button
        label={t("remoteView.back")}
        labelKey="remoteView.back"
        tone="neutral"
        onClick={leave}
        className="ms-auto rounded-pill px-3 py-1 text-xs"
      />
    </div>
  );
}
