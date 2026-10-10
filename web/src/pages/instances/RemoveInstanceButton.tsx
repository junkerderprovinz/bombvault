import { useState } from "react";

import { Button } from "../../components/Button";
import { IconTrash } from "../../components/navGlyphs";
import { deleteFleetPeer } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

/** RemoveInstanceButton drops an instance's row on the second click.
 *  Removing never contacts the instance and is undone by pairing it again,
 *  so a confirm on the button itself is enough. */
export function RemoveInstanceButton({ peerId, onRemoved }: { peerId: string; onRemoved: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [armed, setArmed] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [shake, setShake] = useState(0);

  async function remove() {
    setRemoving(true);
    try {
      const res = await deleteFleetPeer(peerId);
      if (res.ok) {
        onRemoved();
        return;
      }
      // The button stays armed, so a retry needs no second click for a
      // failure that was not the user's mistake.
      push(res.error ?? t("fleet.saveError"), "fail");
      setShake((n) => n + 1);
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  if (!armed) {
    return (
      <Button label={t("fleet.remove")} labelKey="fleet.remove" glyph={<IconTrash />} tone="neutral" onClick={() => setArmed(true)} />
    );
  }
  return (
    <Button
      key={shake}
      label={t("fleet.confirmRemove")}
      labelKey="fleet.confirmRemove"
      glyph={<IconTrash />}
      tone="neutral"
      onClick={() => void remove()}
      disabled={removing}
      busy={removing}
      title={removing ? t("fleet.removing") : undefined}
      className={shake ? "glim-shake" : ""}
    />
  );
}
