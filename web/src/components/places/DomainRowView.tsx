import { useState, type CSSProperties } from "react";
import { PlaceMark } from "../placeMarks";
import { cadenceLabel } from "../ScheduleBadge";
import type { SelectOption } from "../SelectField";
import { HomeSelect } from "../placement/HomeSelect";
import { ToggleRow } from "../../pages/settings/shared";
import { hueVars } from "../../lib/appearance";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { placementChanged } from "../../lib/placementEvents";
import { domainName, placeErrorText } from "../../lib/placeText";
import {
  placesChanged,
  previewDomainHome,
  setDomainHome,
  type DomainRow,
  type HomePreview,
  type Place,
  type PlaceRefusal,
} from "../../lib/places";
import { homeExpect, impactLines } from "../../lib/storageDomains";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";

// A choice shows at once and stays through its question and its write,
// whatever list read lands meanwhile; the first read after the write replaces
// it, and a refusal takes it back with a shake.

function Lines({ lines }: { lines: string[] }) {
  return (
    <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
      {lines.map((line, i) => (
        <p key={i}>{line}</p>
      ))}
    </div>
  );
}

// Holds the switch while the question is open; the answer is read once it closes.
function ApplyToOpenSwitch({ onChange }: { onChange: (on: boolean) => void }) {
  const { t } = useT();
  const [on, setOn] = useState(false);
  return (
    <ToggleRow
      label={t("placementDefaults.apply")}
      checked={on}
      onChange={(next) => {
        setOn(next);
        onChange(next);
      }}
    />
  );
}

export function DomainRowView({
  row,
  places,
  index,
  onWritten,
}: {
  row: DomainRow;
  places: Place[];
  index: number;
  /** Reads the rows again and resolves once that read has landed. */
  onWritten: () => Promise<void>;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [pendingHome, setPendingHome] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [homeShake, setHomeShake] = useState(0);
  const domain = domainName(t, row.domain);
  const placeName = (id: string) => places.find((p) => p.id === id)?.name ?? id;

  const storedIn = pendingHome ?? row.storedIn;
  const homeOptions: SelectOption<string>[] = places
    .filter((p) => p.enabled && p.folders[row.domain] !== undefined)
    .map((p) => ({ value: p.id, label: p.name, glyph: <PlaceMark provider={p.provider} /> }));
  if (!homeOptions.some((o) => o.value === storedIn)) {
    // The field never pretends the domain is stored anywhere else.
    const current = places.find((p) => p.id === storedIn);
    homeOptions.push({
      value: storedIn,
      label: current?.name ?? t("places.unplaced.title"),
      glyph: current ? <PlaceMark provider={current.provider} /> : undefined,
      disabled: true,
    });
  }

  function refused(res: PlaceRefusal) {
    push(placeErrorText(t, lang, res, "settings.error"), "fail");
    setHomeShake((n) => n + 1);
  }

  async function askHome(p: HomePreview): Promise<{ applyToOpen: boolean } | null> {
    const place = placeName(p.placeId);
    const fill = (key: "storageDomains.homePlaceAsk" | "storageDomains.homeMoveAsk") =>
      t(key).replace("{domain}", () => domain).replace("{place}", () => place);
    if (p.mode === "home-place") {
      return (await confirm(fill("storageDomains.homePlaceAsk"), { confirmKey: "placement.saveHome" }))
        ? { applyToOpen: false }
        : null;
    }
    if (p.mode === "home-move") {
      const extra = p.backups > 0 ? <Lines lines={[t("storageDomains.homeMoveStays", p.backups)]} /> : undefined;
      return (await confirm(fill("storageDomains.homeMoveAsk"), { confirmKey: "placement.saveHome", extra }))
        ? { applyToOpen: false }
        : null;
    }
    const lines = p.impact ? impactLines(t, lang, p.impact, place) : [];
    if (p.creates === "repository") {
      lines.unshift(t("storageDomains.createsRepository").replace("{domain}", () => domain).replace("{place}", () => place));
    } else if (p.creates === "direct") {
      lines.unshift(t("storageDomains.createsDirect").replace("{place}", () => place));
    }
    let applyToOpen = false;
    const extra = (
      <div className="flex flex-col gap-3">
        {lines.length > 0 && <Lines lines={lines} />}
        <ApplyToOpenSwitch onChange={(on) => (applyToOpen = on)} />
      </div>
    );
    const question = t("placementDefaults.confirmHome").replace("{domain}", () => domain).replace("{home}", () => place);
    return (await confirm(question, { confirmKey: "placement.saveHome", extra })) ? { applyToOpen } : null;
  }

  async function chooseHome(placeId: string) {
    setPendingHome(placeId);
    setBusy(true);
    try {
      let preview = await previewDomainHome(row.domain, placeId);
      for (;;) {
        if (!preview.ok) return refused(preview);
        const answer = await askHome(preview);
        if (!answer) return;
        const res = await setDomainHome(row.domain, { placeId, expect: homeExpect(preview), applyToOpen: answer.applyToOpen });
        if (res.ok) {
          const changed = (res.kept ?? []).filter((k) => k.reason === "changed");
          if (changed.length > 0) {
            push(t("placementDefaults.applyKeptChanged").replace("{list}", () => formatList(lang, changed.map((k) => k.label))), "warn");
          }
          placesChanged();
          placementChanged();
          await onWritten();
          return;
        }
        if (res.code !== "stale" || !res.preview) return refused(res);
        preview = { ok: true, ...res.preview };
      }
    } catch (err) {
      refused({ ok: false, error: err instanceof Error ? err.message : undefined });
    } finally {
      setPendingHome(null);
      setBusy(false);
    }
  }

  return (
    <section
      aria-label={domain}
      className="glim-hue flex flex-col gap-3 rounded-card bg-carbon-surface2 px-3 py-2"
      style={hueVars(index) as CSSProperties}
    >
      {confirmDialog}
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="text-sm font-semibold text-carbon-text">{domain}</span>
        <a href="#schedules" className="text-xs text-carbon-textSub hover:underline">
          {cadenceLabel(row.schedule, t)}
        </a>
      </div>
      {row.unreadable ? (
        <p className="text-xs text-statusWarn">{t("placement.unreadable")}</p>
      ) : (
        <div key={homeShake} className={homeShake ? "glim-shake" : undefined}>
          <HomeSelect
            label={t("storageDomains.storedIn")}
            value={storedIn}
            options={homeOptions}
            locked={false}
            disabled={busy}
            well
            onCommit={(id) => void chooseHome(id)}
          />
        </div>
      )}
    </section>
  );
}
