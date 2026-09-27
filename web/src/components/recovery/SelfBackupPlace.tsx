import { useEffect, useState } from "react";
import { PlaceMark } from "../placeMarks";
import { listOffsiteTargets } from "../../lib/api";
import { listPlaces, subscribePlaces, type Place } from "../../lib/places";
import { subscribeOffsiteTargets } from "../../lib/useOffsiteTargets";

export interface SelfBackupPlaces {
  /** The place the Self-Backup row names as Stored in. */
  home?: Place;
  /** The target the Self-Backup's off-site field stands for, and its place. */
  copy?: { targetId: string; name: string; place?: Place };
}

/** useSelfBackupPlaces reads where the Self-Backup lies and where its first
 *  copy goes, again after every write to a place or a target. Null until the
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
          const field = (o.ok ? (o.targets ?? []) : []).find((x) => x.sortOrder === 0);
          setState({
            home: places.find((x) => x.usage.homeDomains.includes("config")),
            copy: field && { targetId: field.id, name: field.name, place: places.find((x) => x.id === field.placeId) },
          });
        })
        .catch(() => {
          if (mine === seq) setState({});
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
 *  resolves to for the Self-Backup. */
export function PlaceLine({ label, place, name, address }: { label: string; place?: Place; name: string; address: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-carbon-textSub">{label}</span>
      <span className="flex items-center gap-2 text-sm text-carbon-text">
        {place && <PlaceMark provider={place.provider} />}
        {name}
      </span>
      {address && (
        <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">
          {address}
        </span>
      )}
    </div>
  );
}
