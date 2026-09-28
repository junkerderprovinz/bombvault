import { useRef, useState, type CSSProperties } from "react";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { PlaceMark } from "../placeMarks";
import { PlaceDetails, type PlaceDetailsHandle } from "./PlaceDetails";
import { hueVars } from "../../lib/appearance";
import { useT } from "../../lib/i18n";
import { domainNames, kindName, placeErrorText, probeFailureText, providerName } from "../../lib/placeText";
import { deletePlace, placesChanged, testPlace, type CatalogProvider, type Place } from "../../lib/places";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";

/** PlaceRow is one place on the Storage tab, with its details folded out below it. */
export function PlaceRow({
  place,
  provider,
  hueIndex,
  hostMountRoot,
  onSaved,
}: {
  place: Place;
  provider?: CatalogProvider;
  hueIndex: number;
  hostMountRoot: string;
  onSaved: (place: Place) => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [open, setOpen] = useState(false);
  const [testing, setTesting] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [shake, setShake] = useState({ test: 0, remove: 0 });
  const details = useRef<PlaceDetailsHandle>(null);

  const usage = [
    place.usage.homeDomains.length > 0 && t("places.row.stores").replace("{domains}", domainNames(t, lang, place.usage.homeDomains)),
    place.usage.defaults.length > 0 && t("places.row.defaultFor").replace("{domains}", domainNames(t, lang, place.usage.defaults)),
    place.usage.copyDomains.length > 0 && t("places.row.copies").replace("{domains}", domainNames(t, lang, place.usage.copyDomains)),
    place.usage.items > 0 && t("places.row.items", place.usage.items),
  ].filter((part): part is string => typeof part === "string");

  const last = place.lastTest;
  const lastText = !last
    ? t("places.row.untested")
    : (last.source === "test"
        ? t(last.ok ? "places.row.testedOk" : "places.row.testedFail")
        : t(last.ok ? "places.row.runOk" : "places.row.runFail")
      ).replace("{when}", relativeTime(t, last.at));

  // A question about what was just typed needs the details on screen.
  async function closeDetails() {
    await details.current?.flush();
    setOpen(false);
  }

  async function test() {
    setTesting(true);
    try {
      const res = await testPlace(place.id);
      if (res.ok) {
        push(t("places.row.testOk").replace("{name}", () => place.name), "success");
      } else {
        push(probeFailureText(t, lang, res), "fail");
        setShake((s) => ({ ...s, test: s.test + 1 }));
      }
      // The list shows the outcome as the last test.
      placesChanged();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      setShake((s) => ({ ...s, test: s.test + 1 }));
    } finally {
      setTesting(false);
    }
  }

  async function remove() {
    const question =
      place.usage.copies > 0
        ? t("places.row.removeAskCopies", place.usage.copies).replace("{name}", () => place.name)
        : t("places.row.removeAsk").replace("{name}", () => place.name);
    if (!(await confirm(question, { confirmKey: "places.row.remove", cancelTone: "neutral" }))) return;
    setRemoving(true);
    try {
      const res = await deletePlace(place.id);
      if (res.ok) {
        push(t("places.row.removed").replace("{name}", () => place.name), "success");
        placesChanged();
        return;
      }
      push(placeErrorText(t, lang, res, "common.removeFailed"), "fail");
      setShake((s) => ({ ...s, remove: s.remove + 1 }));
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.removeFailed"), "fail");
      setShake((s) => ({ ...s, remove: s.remove + 1 }));
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div className="glim-hue flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2" style={hueVars(hueIndex) as CSSProperties}>
      {confirmDialog}
      <div className="flex flex-wrap items-center gap-3">
        <PlaceMark provider={place.provider} size={32} />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="truncate text-sm font-semibold text-carbon-text max-md:whitespace-normal max-md:wrap-anywhere">
              {place.name}
            </span>
            {place.offPremises && <Badge tone="neutral">{t("places.row.otherSite")}</Badge>}
            {!place.enabled && <Badge tone="neutral">{t("places.row.off")}</Badge>}
          </div>
          <span className="text-xs text-carbon-textSub">
            {providerName(t, place.provider)} · {kindName(t, place.kind)}
          </span>
          <span className="text-xs text-carbon-textSub">{usage.length > 0 ? usage.join(" · ") : t("places.row.unused")}</span>
          <span className={`text-xs ${last && !last.ok ? "text-statusFail" : "text-carbon-textMuted"}`}>{lastText}</span>
        </div>
        {/* On a phone the actions take their own row under the name, which
            beside them shrinks to a few characters. */}
        <div className="flex flex-wrap items-center gap-2 max-md:w-full">
          <Button
            label={t(open ? "places.row.closeDetails" : "places.row.showDetails")}
            labelKey={open ? "places.row.closeDetails" : "places.row.showDetails"}
            tone="neutral"
            onClick={() => (open ? void closeDetails() : setOpen(true))}
          />
          <Button
            key={`test-${shake.test}`}
            label={t("places.row.test")}
            labelKey="places.row.test"
            tone="neutral"
            busy={testing}
            disabled={testing}
            onClick={() => void test()}
            className={shake.test ? "glim-shake" : ""}
          />
          <Button
            key={`remove-${shake.remove}`}
            label={t("places.row.remove")}
            labelKey="places.row.remove"
            tone="neutral"
            busy={removing}
            disabled={removing}
            onClick={() => void remove()}
            className={shake.remove ? "glim-shake" : ""}
          />
        </div>
      </div>
      {open && (
        <PlaceDetails
          ref={details}
          place={place}
          provider={provider}
          hueIndex={hueIndex}
          hostMountRoot={hostMountRoot}
          onSaved={onSaved}
        />
      )}
    </div>
  );
}
