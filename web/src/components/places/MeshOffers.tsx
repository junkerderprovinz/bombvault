import { useCallback, useEffect, useState } from "react";
import { InfoBubble } from "../InfoBubble";
import { MeshOfferRow } from "../MeshOfferRow";
import { listMeshOffers, type MeshOffer } from "../../lib/api";
import { useT } from "../../lib/i18n";
import type { Place } from "../../lib/places";

/** MeshOffers lists the open offers other BombVaults sent this one, under the
 *  Another BombVault tile. It shows nothing while there are none. */
export function MeshOffers({ onAccepted }: { onAccepted: (place: Place) => void }) {
  const { t } = useT();
  const [offers, setOffers] = useState<MeshOffer[]>([]);

  const load = useCallback(async () => {
    try {
      const res = await listMeshOffers();
      if (res.ok) setOffers((res.offers ?? []).filter((o) => o.status === "pending"));
    } catch {
      setOffers([]);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (offers.length === 0) return null;
  return (
    <div className="flex flex-col gap-2">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("places.offers.title")}
        <InfoBubble tip={t("places.offers.hint")} />
      </span>
      {offers.map((o) => (
        <MeshOfferRow key={o.id} offer={o} t={t} onChanged={() => void load()} onAccepted={onAccepted} />
      ))}
    </div>
  );
}
