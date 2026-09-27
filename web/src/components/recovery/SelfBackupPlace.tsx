import { useEffect, useState } from "react";
import { PlaceMark } from "../placeMarks";
import { SelectField } from "../SelectField";
import { listOffsiteTargets, type OffsiteTarget } from "../../lib/api";
import { listPlaces, subscribePlaces, type Place } from "../../lib/places";
import { subscribeOffsiteTargets } from "../../lib/useOffsiteTargets";

export interface SelfBackupCopy {
  targetId: string;
  name: string;
  repo: string;
  place?: Place;
}

export interface SelfBackupPlaces {
  /** The place the Self-Backup row names as Stored in. */
  home?: Place;
  copies: SelfBackupCopy[];
}

/** selfBackupCopies lists the switched-on Self-Backup targets, those at
 *  another site first: a recovery usually starts because this site is gone. */
export function selfBackupCopies(targets: OffsiteTarget[], places: Place[]): SelfBackupCopy[] {
  const copies = targets
    .filter((x) => x.enabled)
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .map((x) => ({ targetId: x.id, name: x.name, repo: x.repo, place: places.find((p) => p.id === x.placeId) }));
  const away = (c: SelfBackupCopy) => (c.place?.offPremises ? 0 : 1);
  return copies.sort((a, b) => away(a) - away(b));
}

/** useSelfBackupPlaces reads where the Self-Backup lies and where it is
 *  copied to, again after every write to a place or a target. Null until the
 *  first read lands. */
export function useSelfBackupPlaces(): SelfBackupPlaces | null {
  const [state, setState] = useState<SelfBackupPlaces | null>(null);
  useEffect(() => {
    let seq = 0;
    const load = () => {
      const mine = ++seq;
      Promise.all([listPlaces(), listOffsiteTargets("config")])
        .then(([p, o]) => {
          if (mine !== seq) return;
          const places = p.ok ? (p.places ?? []) : [];
          setState({
            home: places.find((x) => x.usage.homeDomains.includes("config")),
            copies: selfBackupCopies(o.ok ? (o.targets ?? []) : [], places),
          });
        })
        .catch(() => {
          if (mine === seq) setState({ copies: [] });
        });
    };
    load();
    const offs = [subscribePlaces(load), subscribeOffsiteTargets(load)];
    return () => {
      seq++;
      offs.forEach((off) => off());
    };
  }, []);
  return state;
}

/** PlaceLine shows a place read-only: its mark and name, and the address it
 *  resolves to for the Self-Backup. An address on no place, or one whose
 *  place has not loaded yet, shows alone. */
export function PlaceLine({ label, place, name, address }: { label: string; place?: Place; name: string; address: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-carbon-textSub">{label}</span>
      {name && (
        <span className="flex items-center gap-2 text-sm text-carbon-text">
          {place && <PlaceMark provider={place.provider} />}
          {name}
        </span>
      )}
      {address && (
        <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">
          {address}
        </span>
      )}
    </div>
  );
}

/** CopyPicker shows the Self-Backup's copies to restore from, as a PlaceLine
 *  for one and as a choice for several, with the chosen copy's address. */
export function CopyPicker({
  label,
  copies,
  value,
  onChange,
  disabled,
}: {
  label: string;
  copies: SelfBackupCopy[];
  value: SelfBackupCopy;
  onChange: (targetId: string) => void;
  disabled?: boolean;
}) {
  const name = (c: SelfBackupCopy) => c.place?.name ?? (c.name || c.repo);
  if (copies.length < 2) return <PlaceLine label={label} place={value.place} name={name(value)} address={value.repo} />;
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-carbon-textSub">{label}</span>
      <SelectField
        label={label}
        value={value.targetId}
        onChange={onChange}
        disabled={disabled}
        options={copies.map((c) => ({
          value: c.targetId,
          label: name(c),
          glyph: c.place && <PlaceMark provider={c.place.provider} />,
        }))}
        className="self-start rounded-control bg-carbon-surface2 px-2 py-1.5 text-sm text-carbon-text glim-field-focus"
      />
      <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">
        {value.repo}
      </span>
    </div>
  );
}
