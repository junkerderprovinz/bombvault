import { useState } from "react";

import { Button } from "../../components/Button";
import { IconTrash } from "../../components/navGlyphs";
import { deleteFleetPeer, removeMember } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import type { Instance } from "./instancesModel";

/** RemoveInstanceButton drops an instance's row on the second click.
 *  Removing never contacts the instance and is undone by pairing it again,
 *  so a confirm on the button itself is enough. Without the Instances module
 *  the page holds no row, and the member goes by its id. The server ends the
 *  Receiver and Fetcher requests of an instance that is away, its waiting key
 *  requests and the ZFS receive slots that let nothing in; ZFS replication and
 *  key deposits hold data, so they stay, and the card with them. */
export function RemoveInstanceButton({
  instance,
  onRemoved,
}: {
  instance: Pick<Instance, "memberId" | "peer" | "name" | "roles">;
  onRemoved: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [armed, setArmed] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [shake, setShake] = useState(0);

  async function remove() {
    setRemoving(true);
    try {
      const res = await (instance.peer ? deleteFleetPeer(instance.peer.id) : removeMember(instance.memberId));
      if (res.ok) {
        const { byMember, forMember } = instance.roles ?? {};
        // The server drops the receive slots that let nothing in.
        const receiving = forMember?.zfs?.items.some((i) => i.state === "allowed");
        if (byMember?.zfs || byMember?.keeper || receiving || forMember?.keeper) {
          push(t("instances.removeKeptByRoles").replaceAll("{peer}", () => instance.name), "warn");
        }
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
