import { useEffect, useState } from "react";
import type { PickGroup } from "@glimstone/core";
import type { PickerItem } from "@glimstone/react/PickerWindow";
import {
  joinSharedRepo,
  listGroupReceivers,
  listSharedOffers,
  type GroupReceiver,
  type SharedOffer,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { IconLink, IconShieldOn } from "../glyphs";

/** Where a receiver's address points, without the scheme and the login. */
function shownAddress(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

// An offer this instance asked for or is in already has nothing left to pick.
const SETTLED: Partial<Record<NonNullable<SharedOffer["state"]>, TranslationKey>> = {
  asked: "shared.member.asked",
  allowed: "shared.member.allowed",
  joined: "shared.member.joined",
};

const GROUP = "fromGroup";
const RECEIVER = "receiver:";
const SHARED = "shared:";

/** The id of a member's receiver among the tiles. */
export const receiverTile = (memberId: string) => RECEIVER + memberId;

export interface GroupOffers {
  /** The tiles, none outside a group or when no member offers anything. */
  items: PickerItem[];
  group?: PickGroup;
  /** Takes a pick that is one of these tiles, and answers whether it was. */
  pick: (id: string) => boolean;
}

/**
 * useGroupOffers is the tiles that stand above the providers: the receivers
 * other instances of the pairing group run, where a pick fills in the
 * rest-server form, and the shared repositories they hold, where a pick asks
 * the owner to let this instance in. A tile that cannot be picked yet stays in
 * view and says why.
 */
export function useGroupOffers({
  onPick,
  onAsked,
}: {
  onPick: (r: GroupReceiver) => void;
  /** An ask to join was sent. The location follows once the owner allows it. */
  onAsked: () => void;
}): GroupOffers {
  const { t } = useT();
  const { push } = useToast();
  const [receivers, setReceivers] = useState<GroupReceiver[]>([]);
  const [offers, setOffers] = useState<SharedOffer[]>([]);
  const [asking, setAsking] = useState("");
  const offerName = (o: SharedOffer) => t("shared.offer.tile").replace("{name}", () => o.name);
  const needsTip = (o: { needsHost?: boolean }) => t(o.needsHost ? "dest.receiver.needsHostTip" : "dest.receiver.needsAddressTip");

  useEffect(() => {
    let alive = true;
    void listGroupReceivers()
      .then((r) => alive && r.ok && setReceivers(r.receivers ?? []))
      .catch(() => undefined);
    void listSharedOffers()
      .then((r) => alive && r.ok && setOffers(r.offers ?? []))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  async function ask(o: SharedOffer) {
    if (asking !== "") return;
    setAsking(o.memberId);
    try {
      const r = await joinSharedRepo(o.memberId);
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      push(t(r.state === "joined" ? "shared.offer.joined" : "shared.offer.asked").replace("{name}", () => o.name), "success");
      onAsked();
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    } finally {
      setAsking("");
    }
  }

  const items: PickerItem[] = [
    ...receivers.map((r) => ({
      id: RECEIVER + r.memberId,
      label: r.name,
      group: GROUP,
      mark: <IconShieldOn />,
      ...(r.url ? { sub: shownAddress(r.url) } : { status: t("dest.receiver.needsAddress"), hint: needsTip(r) }),
    })),
    ...offers.map((o) => {
      const settled = o.state ? SETTLED[o.state] : undefined;
      const url = settled || asking === o.memberId ? undefined : o.url;
      return {
        id: SHARED + o.memberId,
        label: offerName(o),
        group: GROUP,
        mark: <IconLink />,
        hint: o.url || settled ? t("shared.offer.tip").replace("{name}", () => o.name) : needsTip(o),
        ...(url ? { sub: shownAddress(url) } : { status: t(settled ?? (o.url ? "shared.offer.asking" : "dest.receiver.needsAddress")) }),
      };
    }),
  ];

  const group: PickGroup | undefined =
    items.length === 0
      ? undefined
      : {
          id: GROUP,
          title: t("dest.group.fromGroup"),
          badge: { label: t("dest.fit.best"), tone: "ok" },
          hint: [t("dest.groupTip.fromGroup"), offers.length > 0 && t("shared.offer.groupTip")].filter(Boolean).join(" "),
        };

  function pick(id: string): boolean {
    const receiver = receivers.find((r) => RECEIVER + r.memberId === id);
    if (receiver) {
      onPick(receiver);
      return true;
    }
    const offer = offers.find((o) => SHARED + o.memberId === id);
    if (offer) {
      void ask(offer);
      return true;
    }
    return false;
  }

  return { items, group, pick };
}
