import { useId, useRef, useState } from "react";
import { useT } from "../../lib/i18n";
import { useStreaming, type StreamingNumberKey, type StreamingState } from "../../lib/useStreaming";
import { Card, ToggleRow } from "../../pages/settings/shared";
import { SelectDot } from "../backups/SelectMark";
import { Button } from "../Button";
import { DropdownListbox } from "../DropdownListbox";
import { NumberField } from "../NumberField";
import { Rows, SettingRow } from "./rows";

const NUMBER_FIELDS: { key: StreamingNumberKey; label: "streaming.threshold" | "streaming.limit" | "streaming.hold"; hint: "streaming.thresholdHint" | "streaming.limitHint" | "streaming.holdHint"; max: number }[] = [
  { key: "thresholdMbit", label: "streaming.threshold", hint: "streaming.thresholdHint", max: 10000 },
  { key: "limitKiB", label: "streaming.limit", hint: "streaming.limitHint", max: 10000000 },
  { key: "holdMin", label: "streaming.hold", hint: "streaming.holdHint", max: 120 },
];

/** StreamingFields are the media servers and the three numbers that say when
 *  a stream counts and how far a copy slows down for it. */
function StreamingFields({ t, state }: { t: ReturnType<typeof useT>["t"]; state: StreamingState }) {
  const { cfg, candidates, setNumber, toggleServer } = state;
  const [pickerOpen, setPickerOpen] = useState(false);
  const pickerRef = useRef<HTMLButtonElement>(null);
  const id = useId();
  if (!cfg) return null;

  const byName = new Map(candidates.map((c) => [c.name, c]));
  const chosen = cfg.mediaServers;

  return (
    <>
      <>
        <SettingRow label={t("streaming.servers")} hint={t("streaming.serversHint")}>
        <div className="inline-block">
          <button
            ref={pickerRef}
            type="button"
            aria-haspopup="listbox"
            aria-expanded={pickerOpen}
            onClick={() => setPickerOpen((v) => !v)}
            className="flex items-center gap-2 w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-surface3 transition-colors text-start"
          >
            <span className="min-w-0 flex-1 truncate">{t("streaming.pick")}</span>
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none" aria-hidden="true" className={`shrink-0 transition-transform ${pickerOpen ? "rotate-90" : "rtl:rotate-180"}`}>
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
          </button>
          <DropdownListbox
            open={pickerOpen}
            onClose={() => setPickerOpen(false)}
            triggerRef={pickerRef}
            label={t("streaming.servers")}
            multiselectable
          >
            <>
              {candidates.map((c) => {
                const checked = chosen.includes(c.name);
                return (
                  <button
                    key={c.name}
                    type="button"
                    role="option"
                    aria-selected={checked}
                    onClick={() => void toggleServer(c.name)}
                    className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-start transition-colors ${
                      checked ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                    }`}
                  >
                    <SelectDot checked={checked} />
                    <span className="min-w-0 flex-1 truncate">{c.name}</span>
                    {c.hostNetwork && <span className="shrink-0 text-caption text-statusWarn">{t("streaming.hostNetwork")}</span>}
                  </button>
                );
              })}
            </>
          </DropdownListbox>
        </div>
        </SettingRow>
        {chosen.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("streaming.none")}</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {chosen.map((n) => (
              <span key={n} className="inline-flex items-center gap-1.5 rounded-pill bg-carbon-surface2 px-2 py-0.5 text-xs text-carbon-textSub">
                {n}
                {byName.get(n)?.hostNetwork && <span className="text-caption text-statusWarn">{t("streaming.hostNetwork")}</span>}
                {!byName.has(n) && <span className="text-caption text-statusFail">{t("containers.notInstalled")}</span>}
                <Button
                  label={t("streaming.remove").replace("{name}", n)}
                  labelKey="streaming.remove"
                  variant="chip"
                  onClick={() => void toggleServer(n)}
                />
              </span>
            ))}
          </div>
        )}
      </>

      <Rows>
        {NUMBER_FIELDS.map(({ key, label, hint, max }) => (
          <SettingRow key={key} label={t(label)} hint={t(hint)} htmlFor={`${id}-${key}`}>
            <NumberField
              id={`${id}-${key}`}
              wrapperClassName="w-28"
              min={1}
              max={max}
              value={cfg[key]}
              onChange={(e) => setNumber(key, Math.min(max, Math.max(1, parseInt(e.target.value, 10) || 1)))}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
            />
          </SettingRow>
        ))}
      </Rows>
    </>
  );
}

/**
 * StreamingCard lets off-site copies give way to a media server's streams.
 * The media servers it picks also decide when such a container counts as idle
 * for a backup that waits for it. It is left out when the settings cannot be
 * read.
 */
export function StreamingCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const state = useStreaming(t);
  const { cfg, failed, streaming, busy, shake, pulse, toggle } = state;
  if (failed || !cfg) return null;
  return (
    <Card title={t("storage.streaming.title")} hint={t("storage.streaming.hint")} hueIndex={hueIndex}>
      <ToggleRow
        label={t("streaming.toggle")}
        hint={t("storage.streaming.toggleHint")}
        checked={cfg.enabled}
        onChange={(next) => void toggle(next)}
        disabled={busy}
        shakeNonce={shake}
        pulseNonce={pulse}
      />
      {cfg.enabled && streaming && (
        <p className="text-xs text-statusWarn" role="status">
          {t("streaming.now").replace("{name}", () => streaming)}
        </p>
      )}
      {cfg.enabled && <StreamingFields t={t} state={state} />}
    </Card>
  );
}
