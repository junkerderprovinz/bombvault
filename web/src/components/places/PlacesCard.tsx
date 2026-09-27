import { useCallback, useEffect, useId, useState } from "react";
import { AddPlaceDialog } from "./AddPlaceDialog";
import { PlaceRow } from "./PlaceRow";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { SelectField } from "../SelectField";
import { Toggle } from "../Toggle";
import { PlaceMark } from "../placeMarks";
import { Card } from "../../pages/settings/shared";
import { useT, type TranslationKey } from "../../lib/i18n";
import { domainName, placeErrorText } from "../../lib/placeText";
import {
  PLACE_DOMAINS,
  adoptRow,
  listPlaces,
  placesChanged,
  setUnplacedAppendOnly,
  subscribePlaces,
  type Place,
  type UnplacedRow,
} from "../../lib/places";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { usePlacesCatalog } from "../../lib/usePlacesCatalog";

// Everything the card shows comes from the database, so opening the Storage
// tab never lists a remote repository.

const ROLE_KEYS: Record<UnplacedRow["role"], TranslationKey> = {
  path: "places.unplaced.path",
  target: "places.unplaced.target",
  repository: "places.unplaced.repository",
  direct: "places.unplaced.direct",
};

const PICK_CLASS = "rounded-control bg-carbon-surface3 px-3 py-1.5 text-sm text-carbon-text glim-field-focus-well";

function AppendOnlySwitch({ row }: { row: UnplacedRow }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [on, setOn] = useState(row.immutable);
  const [shake, setShake] = useState(0);

  async function flip(next: boolean) {
    if (!next && row.items > 0 && !(await confirm(t("places.unplaced.appendOnlyOffAsk", row.items)))) return;
    setOn(next);
    try {
      const res = await setUnplacedAppendOnly(row.rowId, row.domain, next);
      if (res.ok) {
        placesChanged();
        return;
      }
      push(placeErrorText(t, lang, res, "common.actionFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
    }
    setOn(!next);
    setShake((n) => n + 1);
  }

  return (
    <>
      {confirmDialog}
      <span key={shake} className={`flex items-center gap-1.5 self-center ${shake ? "glim-shake" : ""}`}>
        <Toggle label={t("places.details.appendOnly")} checked={on} onChange={(v) => void flip(v)} />
        <InfoBubble tip={t("places.details.appendOnlyHint")} />
      </span>
    </>
  );
}

function UnplacedRowView({ row, places }: { row: UnplacedRow; places: Place[] }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [placeId, setPlaceId] = useState("");
  // Only a named repository has no domain of its own; "" lets every domain share it.
  const [domain, setDomain] = useState("");
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const id = useId();
  const title = row.name || domainName(t, row.domain);
  const role = t(ROLE_KEYS[row.role]).replace("{domain}", domainName(t, row.domain));

  async function link() {
    const place = places.find((p) => p.id === placeId);
    if (!place) return;
    setBusy(true);
    try {
      const res = await adoptRow(place.id, row.rowId, row.role === "path" ? row.domain : row.role === "repository" ? domain : "");
      if (res.ok) {
        push(
          t("places.unplaced.linked")
            .replace("{name}", () => title)
            .replace("{place}", () => place.name),
          "success"
        );
        placesChanged();
        return;
      }
      push(placeErrorText(t, lang, res, "common.actionFailed"), "fail");
      setShake((n) => n + 1);
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-wrap items-end gap-3 rounded-card bg-carbon-surface2 px-3 py-2">
      <div className="flex min-w-[12rem] flex-1 flex-col gap-0.5">
        <span className="text-sm font-semibold text-carbon-text">{title}</span>
        <span className="text-xs text-carbon-textSub">{role}</span>
        <span dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textSub">
          {row.repo}
        </span>
      </div>
      {row.protectable && <AppendOnlySwitch key={String(row.immutable)} row={row} />}
      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${id}-place`} className="text-xs text-carbon-textSub">
          {t("places.unplaced.place")}
        </label>
        <SelectField
          id={`${id}-place`}
          label={t("places.unplaced.place")}
          value={placeId}
          onChange={setPlaceId}
          options={[
            { value: "", label: t("places.unplaced.choose") },
            ...places.map((p) => ({ value: p.id, label: p.name, glyph: <PlaceMark provider={p.provider} /> })),
          ]}
          className={PICK_CLASS}
        />
      </div>
      {row.role === "repository" && (
        <div className="flex flex-col gap-1.5">
          <label htmlFor={`${id}-domain`} className="text-xs text-carbon-textSub">
            {t("places.unplaced.domain")}
          </label>
          <SelectField
            id={`${id}-domain`}
            label={t("places.unplaced.domain")}
            value={domain}
            onChange={setDomain}
            options={[
              { value: "", label: t("places.unplaced.allDomains") },
              ...PLACE_DOMAINS.map((d) => ({ value: d, label: domainName(t, d) })),
            ]}
            className={PICK_CLASS}
          />
        </div>
      )}
      <Button
        key={shake}
        label={t("places.unplaced.link")}
        labelKey="places.unplaced.link"
        tone="neutral"
        busy={busy}
        disabled={busy || placeId === ""}
        onClick={() => void link()}
        className={shake ? "glim-shake" : ""}
      />
    </div>
  );
}

export function PlacesCard({ hueIndex, hostMountRoot }: { hueIndex?: number; hostMountRoot: string }) {
  const { t } = useT();
  const { push } = useToast();
  const { providers } = usePlacesCatalog();
  const [places, setPlaces] = useState<Place[]>([]);
  const [unplaced, setUnplaced] = useState<UnplacedRow[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [adding, setAdding] = useState(false);

  // Only a list that was read can say there is no place yet.
  const load = useCallback(async () => {
    const res = await listPlaces().catch(() => null);
    if (!res?.ok) {
      push(t("places.loadFailed"), "fail");
      return;
    }
    setPlaces(res.places ?? []);
    setUnplaced(res.unplaced ?? []);
    setLoaded(true);
  }, [push, t]);

  useEffect(() => {
    void load();
    return subscribePlaces(() => void load());
  }, [load]);

  function saved(place: Place) {
    setPlaces((all) => all.map((p) => (p.id === place.id ? place : p)));
  }

  return (
    <Card title={t("places.title")} hint={t("places.hint")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-3">
        {loaded && places.length === 0 && <p className="text-sm text-carbon-textSub">{t("places.empty")}</p>}
        {places.map((p, i) => (
          <PlaceRow
            key={p.id}
            place={p}
            provider={providers.find((c) => c.id === p.provider)}
            hueIndex={i}
            hostMountRoot={hostMountRoot}
            onSaved={saved}
          />
        ))}
      </div>

      <Button
        label={t("places.add")}
        labelKey="places.add"
        tone="accent"
        onClick={() => setAdding(true)}
        className="glim-btn-key self-start"
      />

      {unplaced.length > 0 && (
        <section className="flex flex-col gap-3 pt-2">
          <h3 className="flex items-center">
            <Badge tone="heading" size="heading" inFlow hueIndex={hueIndex}>
              {t("places.unplaced.title")}
              <InfoBubble tip={t("places.unplaced.hint")} onAccent />
            </Badge>
          </h3>
          {unplaced.map((row) => (
            <UnplacedRowView key={`${row.role}-${row.rowId}-${row.domain}`} row={row} places={places} />
          ))}
        </section>
      )}

      {adding && <AddPlaceDialog hostMountRoot={hostMountRoot} onClose={() => setAdding(false)} />}
    </Card>
  );
}
