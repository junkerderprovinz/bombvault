import { useEffect, useState } from "react";
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
import { PickerGroup } from "../Picker";
import { ReadmeButton } from "../ReadmeButton";

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

/**
 * The receivers other instances of the pairing group run, above the providers:
 * picking one fills in the rest-server form for it. The shared repositories
 * they hold stand beside them, and picking one asks its owner to let this
 * instance in. Shows nothing outside a group or when no member offers either.
 */
export function GroupReceiverPicker({
  picked,
  onPick,
  onAsked,
  matches = () => true,
}: {
  picked?: string;
  onPick: (r: GroupReceiver) => void;
  /** An ask to join was sent. The location follows once the owner allows it. */
  onAsked: () => void;
  /** Narrows the receivers to the ones a search asks for. */
  matches?: (name: string) => boolean;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [all, setReceivers] = useState<GroupReceiver[]>([]);
  const [allOffers, setOffers] = useState<SharedOffer[]>([]);
  const [asking, setAsking] = useState("");
  const offerName = (o: SharedOffer) => t("shared.offer.tile").replace("{name}", () => o.name);
  const receivers = all.filter((r) => matches(r.name));
  const offers = allOffers.filter((o) => matches(o.name) || matches(offerName(o)));

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

  if (receivers.length + offers.length === 0) return null;

  const tip = [t("dest.groupTip.fromGroup"), offers.length > 0 && t("shared.offer.groupTip")].filter(Boolean).join(" ");

  return (
    <PickerGroup title={t("dest.group.fromGroup")} fit={{ label: t("dest.fit.best"), tone: "ok" }} tip={tip}>
      <ul className="glim-provider-pick">
        {receivers.map((r) => (
          <li key={r.memberId}>
            <ReadmeButton
              tile={`glim-tile-provider${picked === r.memberId ? " glim-provider-picked" : ""}`}
              parts={[
                r.url
                  ? { name: r.name, sub: shownAddress(r.url), onClick: () => onPick(r) }
                  : { name: r.name },
              ]}
              soonLabel={t("dest.receiver.needsAddress")}
              hint={r.url ? undefined : t(r.needsHost ? "dest.receiver.needsHostTip" : "dest.receiver.needsAddressTip")}
              mark={<IconShieldOn />}
              markClass="text-carbon-textSub"
            />
          </li>
        ))}
        {offers.map((o) => {
          const settled = o.state ? SETTLED[o.state] : undefined;
          const url = settled || asking === o.memberId ? undefined : o.url;
          return (
            <li key={`shared:${o.memberId}`} data-offer={o.memberId}>
              <ReadmeButton
                tile="glim-tile-provider"
                parts={[url ? { name: offerName(o), sub: shownAddress(url), onClick: () => void ask(o) } : { name: offerName(o) }]}
                soonLabel={t(settled ?? (o.url ? "shared.offer.asking" : "dest.receiver.needsAddress"))}
                hint={
                  o.url || settled
                    ? t("shared.offer.tip").replace("{name}", () => o.name)
                    : t(o.needsHost ? "dest.receiver.needsHostTip" : "dest.receiver.needsAddressTip")
                }
                mark={<IconLink />}
                markClass="text-carbon-textSub"
              />
            </li>
          );
        })}
      </ul>
    </PickerGroup>
  );
}
