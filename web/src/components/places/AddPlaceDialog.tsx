import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { WindowActions } from "../WindowActions";
import { PlaceForm } from "./PlaceForm";
import { ProviderGrid } from "./ProviderTile";
import { useT } from "../../lib/i18n";
import { placesChanged, type CatalogProvider, type Place } from "../../lib/places";
import { useToast } from "../../lib/toast";
import { useDialogKeys } from "../../lib/useConfirm";
import { usePlacesCatalog } from "../../lib/usePlacesCatalog";

// The "Add a place" window: provider tiles in three groups, then the chosen
// provider's form in the same window. Only the tiles or the form scroll; the
// title stays put. Cancel, Escape and the backdrop discard everything typed,
// which is why this window has an Add button where settings save themselves.

export function AddPlaceDialog({
  hostMountRoot,
  onClose,
  onAdded,
}: {
  hostMountRoot: string;
  onClose: () => void;
  onAdded?: (place: Place) => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const titleId = useId();
  const cardRef = useRef<HTMLDivElement>(null);
  const { providers, loaded, failed } = usePlacesCatalog();
  const [chosen, setChosen] = useState<CatalogProvider | null>(null);
  const [lastPicked, setLastPicked] = useState<string | null>(null);

  useDialogKeys(true, cardRef, onClose);

  useEffect(() => {
    cardRef.current?.focus();
  }, []);

  // Back unmounts the form and the focus in it, so the tile it came from
  // takes the focus.
  useEffect(() => {
    if (chosen || !lastPicked) return;
    cardRef.current?.querySelector<HTMLElement>('[role="option"][aria-selected="true"]')?.focus();
  }, [chosen, lastPicked]);

  function pick(provider: CatalogProvider) {
    setLastPicked(provider.id);
    setChosen(provider);
  }

  function added(place: Place) {
    // A warning, because the new place holds nothing until it is chosen, and
    // quiet mode would drop a success.
    push(t("places.added").replace("{name}", () => place.name), "warn");
    placesChanged();
    onAdded?.(place);
    onClose();
  }

  // The offer row has announced the new place already, and the place takes the
  // offered domain's copies without a choice, so a success says enough.
  function accepted(place: Place) {
    push(t("places.offerAccepted").replace("{name}", () => place.name), "success");
    onAdded?.(place);
    onClose();
  }

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={cardRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="glim-modal-card relative flex max-h-[85vh] w-full max-w-2xl flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex items-start px-5 py-4">
          <h2 id={titleId} className="flex items-center">
            <Badge tone="heading" size="heading" wrap>
              {t("places.addTitle")}
            </Badge>
          </h2>
        </div>

        {chosen ? (
          <PlaceForm
            key={chosen.id}
            provider={chosen}
            hostMountRoot={hostMountRoot}
            onBack={() => setChosen(null)}
            onCancel={onClose}
            onAdded={added}
            onAccepted={accepted}
          />
        ) : (
          <>
            <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4 pt-2">
              {failed !== null ? (
                <p className="text-sm text-statusFail">{t("places.catalogFailed")}</p>
              ) : loaded ? (
                <ProviderGrid providers={providers} selected={lastPicked} onPick={pick} />
              ) : (
                <p className="text-sm text-carbon-textMuted">{t("places.catalogLoading")}</p>
              )}
            </div>
            <WindowActions>
              <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} />
            </WindowActions>
          </>
        )}
      </div>
    </div>,
    document.body
  );
}
