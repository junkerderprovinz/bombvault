import { useCallback, useEffect, useRef, useState } from "react";
import { DomainRowView } from "./DomainRowView";
import { Card } from "../../pages/settings/shared";
import { useT } from "../../lib/i18n";
import { subscribePlacement } from "../../lib/placementEvents";
import {
  getStorageDomains,
  listPlaces,
  subscribePlaces,
  type DomainRow,
  type Place,
  type UnplacedRow,
} from "../../lib/places";
import { subscribeOffsiteTargets } from "../../lib/useOffsiteTargets";

interface DomainRowsState {
  rows: DomainRow[] | null;
  places: Place[];
  unplaced: UnplacedRow[];
  failed: boolean;
}

/** useDomainRows reads the rows and the places together, so a row always finds
 *  the places it names, and again after every write they depend on. The newest
 *  read wins, and reload resolves once it has landed, which is when a row lets
 *  go of a choice it was holding. */
function useDomainRows(): DomainRowsState & { reload: () => Promise<void> } {
  const [state, setState] = useState<DomainRowsState>({ rows: null, places: [], unplaced: [], failed: false });
  const seq = useRef(0);
  const newest = useRef<Promise<void>>(Promise.resolve());

  const reload = useCallback((): Promise<void> => {
    const mine = ++seq.current;
    const read = (async () => {
      try {
        const [d, p] = await Promise.all([getStorageDomains(), listPlaces()]);
        if (mine !== seq.current) return;
        if (d.ok && p.ok) {
          setState({ rows: d.domains ?? [], places: p.places ?? [], unplaced: p.unplaced ?? [], failed: false });
        } else {
          setState((s) => ({ ...s, failed: true }));
        }
      } catch {
        if (mine === seq.current) setState((s) => ({ ...s, failed: true }));
      }
    })();
    const landed = read.then(() => (mine === seq.current ? undefined : newest.current));
    newest.current = landed;
    return landed;
  }, []);

  useEffect(() => {
    void reload();
    const again = () => void reload();
    const offs = [subscribePlaces(again), subscribePlacement(again), subscribeOffsiteTargets(again)];
    return () => offs.forEach((off) => off());
  }, [reload]);

  return { ...state, reload };
}

/** DomainRows is the Domains card without the card, for a page that already
 *  gives the rows a surface, such as a step of Recovery. */
export function DomainRows() {
  const { t } = useT();
  const { rows, places, unplaced, failed, reload } = useDomainRows();
  return (
    <>
      {failed && <p className="text-xs text-statusWarn">{t("storageDomains.loadFailed")}</p>}
      {rows && (
        <div className="flex flex-col gap-3">
          {rows.map((row, i) => (
            <DomainRowView
              key={row.domain}
              row={row}
              places={places}
              unplacedTargets={unplaced.filter((u) => u.role === "target" && u.domain === row.domain)}
              index={i}
              onWritten={reload}
            />
          ))}
        </div>
      )}
    </>
  );
}

export function DomainsCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  return (
    <Card title={t("storageDomains.title")} hint={t("storageDomains.hint")} hueIndex={hueIndex}>
      <DomainRows />
    </Card>
  );
}
