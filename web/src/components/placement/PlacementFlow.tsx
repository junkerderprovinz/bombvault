import { Fragment } from "react";
import { PlaceMark } from "../placeMarks";
import { useT } from "../../lib/i18n";
import { useHostLabel } from "../../lib/useHostLabel";
import { offsiteTargetLabel, useOffsiteTargets } from "../../lib/useOffsiteTargets";
import { usePlaces } from "../../lib/usePlaces";

function Named({ provider, name }: { provider?: string; name: string }) {
  return (
    <span className="inline-flex items-center gap-1 align-middle">
      {provider && <PlaceMark provider={provider} />}
      {name}
    </span>
  );
}

/** PlacementFlow is the one line Flash and self-backup get instead of a bar:
 *  the place the domain is stored at, and the enabled targets it copies to,
 *  each with its place's mark. Neither domain has a home to choose, so there
 *  is nothing else to show. */
export function PlacementFlow({ domain }: { domain: "flash" | "config" }) {
  const { t, lang } = useT();
  const host = useHostLabel();
  const targets = useOffsiteTargets(domain);
  const places = usePlaces();
  if (targets.length === 0) return null;
  const home = places.find((p) => p.usage.homeDomains.includes(domain));
  const providerOf = (placeId: string | undefined) => places.find((p) => p.id === placeId)?.provider;
  // The targets are joined the way the language lists things, with a mark
  // beside each name, so the list is built from its parts.
  const to = new Intl.ListFormat(lang, { type: "conjunction" })
    .formatToParts(targets.map((x) => x.id))
    .map((part, i) => {
      const target = part.type === "element" ? targets.find((x) => x.id === part.value) : undefined;
      return target ? (
        <Named key={target.id} provider={providerOf(target.placeId)} name={offsiteTargetLabel(target)} />
      ) : (
        <Fragment key={i}>{part.value}</Fragment>
      );
    });
  return (
    <p className="text-xs text-carbon-textSub">
      {t("placement.flow")
        .split(/(\{from\}|\{to\})/)
        .map((part, i) =>
          part === "{from}" ? (
            <Named key={i} provider={home?.provider} name={home?.name ?? host} />
          ) : (
            <Fragment key={i}>{part === "{to}" ? to : part}</Fragment>
          )
        )}
    </p>
  );
}
