import { useRef, useState } from "react";
import { flushSync } from "react-dom";
import { Badge } from "./Badge";
import { Button, type ButtonTone } from "./Button";
import { SelectField } from "./SelectField";
import { useNewTargetQuestion } from "./placement/NewTargetQuestion";
import { acceptMeshOffer, declineMeshOffer, type MeshOffer, type OffsiteDomain } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { placementChanged } from "../lib/placementEvents";
import { domainName, placeErrorText } from "../lib/placeText";
import { PLACE_DOMAINS, placesChanged, type Place } from "../lib/places";
import { relativeTime } from "../lib/reltime";
import { useToast } from "../lib/toast";
import { credSetsChanged } from "../lib/useCloudCredSets";
import { offsiteTargetsChanged } from "../lib/useOffsiteTargets";

// An offer is a rest-server another BombVault runs for this one, for one
// domain. The Fleet page lists every offer, the add window the open ones.

type T = ReturnType<typeof useT>["t"];

function meshStatusTone(status: string): "ok" | "fail" | "warn" | "neutral" {
  switch (status) {
    case "accepted":
      return "ok";
    case "declined":
      return "fail";
    default:
      return "warn"; // pending
  }
}

function meshStatusLabelKey(status: string): TranslationKey {
  switch (status) {
    case "accepted":
      return "fleet.mesh.status.accepted";
    case "declined":
      return "fleet.mesh.status.declined";
    default:
      return "fleet.mesh.status.pending";
  }
}

/** MeshOfferRow is one offer with its domain, Decline and Accept. onAccepted
 *  gets the place the accept made. */
export function MeshOfferRow({
  offer,
  t,
  onChanged,
  onAccepted,
  acceptTone = "accent",
}: {
  offer: MeshOffer;
  t: T;
  onChanged: () => void;
  onAccepted?: (place: Place) => void;
  /** Neutral inside a window whose own buttons carry the accent. */
  acceptTone?: ButtonTone;
}) {
  const [domain, setDomain] = useState<string>(offer.suggestedDomain || "containers");
  const [busy, setBusy] = useState(false);
  const { lang } = useT();
  const { push } = useToast();
  const { ask, dialog } = useNewTargetQuestion();
  const [shakeAccept, setShakeAccept] = useState(0);
  const [shakeDecline, setShakeDecline] = useState(0);
  const acceptRef = useRef<HTMLButtonElement>(null);

  async function handleAccept() {
    // The question does a round trip of its own before its dialog appears, and
    // a disabled button is all that keeps a second accept from minting a
    // second target.
    setBusy(true);
    const answer = await ask({
      // The select offers only PLACE_DOMAINS, all of them off-site domains.
      domain: domain as OffsiteDomain,
      location: offer.repo,
      name: offer.from || t("fleet.mesh.unknownPeer"),
      moved: false,
    });
    if (!answer.go) {
      // Accept was locked while the question was open, so the question had
      // nothing to hand focus back to.
      flushSync(() => setBusy(false));
      acceptRef.current?.focus();
      return;
    }
    try {
      const res = await acceptMeshOffer(offer.id, domain, answer.alsoExclude ?? undefined);
      if (res.ok) {
        // Accepting makes a place with the peer's REST login as its
        // credential set and the domain's target there, so every mounted
        // reader of those lists reloads.
        credSetsChanged();
        offsiteTargetsChanged();
        placesChanged();
        if (answer.alsoExclude) placementChanged();
        onChanged();
        if (res.place) onAccepted?.(res.place);
      } else {
        push(placeErrorText(t, lang, res, "fleet.mesh.saveError"), "fail");
        setShakeAccept((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShakeAccept((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  async function handleDecline() {
    setBusy(true);
    try {
      const res = await declineMeshOffer(offer.id);
      if (res.ok) onChanged();
      else {
        push(res.error ?? t("fleet.mesh.saveError"), "fail");
        setShakeDecline((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShakeDecline((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  const pending = offer.status === "pending";

  return (
    <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-2">
      {dialog}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="font-semibold text-carbon-text text-sm truncate max-md:whitespace-normal max-md:wrap-break-word">{offer.from || t("fleet.mesh.unknownPeer")}</span>
        <Badge tone={meshStatusTone(offer.status)}>{t(meshStatusLabelKey(offer.status))}</Badge>
        <span className="text-xs text-carbon-textMuted ms-auto">{relativeTime(t, offer.receivedAt)}</span>
      </div>
      <p dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate text-start max-md:whitespace-normal max-md:break-all">{offer.repo}</p>
      {pending && (
        <div className="flex items-center gap-2 flex-wrap">
          <label className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("fleet.mesh.applyTo")}
            <SelectField
              value={domain}
              onChange={setDomain}
              label={t("fleet.mesh.applyTo")}
              options={PLACE_DOMAINS.map((d) => ({ value: d, label: domainName(t, d) }))}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-xs px-2 py-1 glim-field-focus-well"
            />
          </label>
          <Button
            key={`decline-${shakeDecline}`}
            label={t("fleet.mesh.decline")}
            labelKey="fleet.mesh.decline"
            tone="neutral"
            onClick={() => void handleDecline()}
            disabled={busy}
            className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs text-carbon-text disabled:opacity-50${
              shakeDecline ? " glim-shake" : ""
            }`}
          />
          <Button
            key={`accept-${shakeAccept}`}
            ref={acceptRef}
            label={t("fleet.mesh.accept")}
            labelKey="fleet.mesh.accept"
            tone={acceptTone}
            onClick={() => void handleAccept()}
            disabled={busy}
            className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs font-medium transition-opacity disabled:opacity-50${
              shakeAccept ? " glim-shake" : ""
            }`}
          />
        </div>
      )}
    </div>
  );
}
