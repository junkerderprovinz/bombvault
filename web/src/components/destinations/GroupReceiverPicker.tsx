import { useEffect, useState } from "react";
import { listGroupReceivers, type GroupReceiver } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { IconShieldOn } from "../glyphs";
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

/**
 * The receivers other instances of the pairing group run, above the providers:
 * picking one fills in the rest-server form for it. Shows nothing outside a
 * group or when no member runs one.
 */
export function GroupReceiverPicker({
  picked,
  onPick,
  matches = () => true,
}: {
  picked?: string;
  onPick: (r: GroupReceiver) => void;
  /** Narrows the receivers to the ones a search asks for. */
  matches?: (name: string) => boolean;
}) {
  const { t } = useT();
  const [all, setReceivers] = useState<GroupReceiver[]>([]);
  const receivers = all.filter((r) => matches(r.name));

  useEffect(() => {
    let alive = true;
    void listGroupReceivers()
      .then((r) => alive && r.ok && setReceivers(r.receivers ?? []))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  if (receivers.length === 0) return null;

  return (
    <PickerGroup title={t("dest.group.fromGroup")} fit={{ label: t("dest.fit.best"), tone: "ok" }} tip={t("dest.groupTip.fromGroup")}>
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
              hint={r.url ? undefined : t("dest.receiver.needsAddressTip")}
              mark={<IconShieldOn />}
              markClass="text-carbon-textSub"
            />
          </li>
        ))}
      </ul>
    </PickerGroup>
  );
}
