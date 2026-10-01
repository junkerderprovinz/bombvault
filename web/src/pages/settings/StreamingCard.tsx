import { useEffect, useRef, useState } from "react";
import { Button } from "../../components/Button";
import { DropdownListbox } from "../../components/DropdownListbox";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { getStreaming, setStreaming } from "../../lib/api";
import type { MediaCandidate, StreamingSettings } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card, ToggleRow } from "./shared";

const DEBOUNCE_MS = 800;
// The card shows who streams right now, so it asks again while it is open.
const POLL_MS = 15000;

type NumberKey = "thresholdMbit" | "limitKiB" | "holdMin";

const NUMBER_FIELDS: { key: NumberKey; label: "streaming.threshold" | "streaming.limit" | "streaming.hold"; hint: "streaming.thresholdHint" | "streaming.limitHint" | "streaming.holdHint"; max: number }[] = [
  { key: "thresholdMbit", label: "streaming.threshold", hint: "streaming.thresholdHint", max: 10000 },
  { key: "limitKiB", label: "streaming.limit", hint: "streaming.limitHint", max: 10000000 },
  { key: "holdMin", label: "streaming.hold", hint: "streaming.holdHint", max: 120 },
];

// StreamingCard lets off-site copies give way to a media server's streams.
// The media servers it picks also decide when such a container counts as idle
// for a backup that waits for it.
export function StreamingCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [cfg, setCfg] = useState<StreamingSettings | null>(null);
  const [failed, setFailed] = useState(false);
  const [candidates, setCandidates] = useState<MediaCandidate[]>([]);
  const [streaming, setStreamingNow] = useState("");
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [pulse, setPulse] = useState(0);
  const [pickerOpen, setPickerOpen] = useState(false);
  const pickerRef = useRef<HTMLButtonElement>(null);
  const cfgRef = useRef<StreamingSettings | null>(null);
  const timers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});

  useEffect(() => {
    let alive = true;
    const pending = timers.current;
    const load = (first: boolean) => {
      getStreaming()
        .then((res) => {
          if (!alive) return;
          if (!res.ok || !res.settings) {
            if (first) setFailed(true);
            return;
          }
          if (first) {
            cfgRef.current = res.settings;
            setCfg(res.settings);
            setCandidates(res.candidates ?? []);
          }
          setStreamingNow(res.streaming ?? "");
        })
        .catch(() => {
          if (alive && first) setFailed(true);
        });
    };
    load(true);
    const id = setInterval(() => load(false), POLL_MS);
    return () => {
      alive = false;
      clearInterval(id);
      // A number typed just before leaving the tab is saved, not dropped.
      const unsaved = Object.values(pending);
      for (const timer of unsaved) clearTimeout(timer);
      if (unsaved.length > 0) void save({});
    };
    // Mount only: save reads the latest card through cfgRef.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // save writes the whole card from the latest state, so two quick edits in a
  // row never write an older value over a newer one.
  async function save(patch: Partial<StreamingSettings>): Promise<boolean> {
    const base = cfgRef.current;
    if (!base) return false;
    const next = { ...base, ...patch };
    cfgRef.current = next;
    setCfg(next);
    try {
      const res = await setStreaming(next);
      if (res.ok) {
        push(t("settings.saved"), "success");
        return true;
      }
      push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
    return false;
  }

  async function toggle(next: boolean) {
    setBusy(true);
    const ok = await save({ enabled: next });
    setBusy(false);
    if (ok) {
      setPulse((n) => n + 1);
      return;
    }
    const reverted = cfgRef.current ? { ...cfgRef.current, enabled: !next } : null;
    cfgRef.current = reverted;
    setCfg(reverted);
    setShake((n) => n + 1);
  }

  function setNumber(key: NumberKey, value: number) {
    const base = cfgRef.current;
    if (!base) return;
    const next = { ...base, [key]: value };
    cfgRef.current = next;
    setCfg(next);
    const existing = timers.current[key];
    if (existing) clearTimeout(existing);
    timers.current[key] = setTimeout(() => {
      delete timers.current[key];
      void save({});
    }, DEBOUNCE_MS);
  }

  function toggleServer(name: string) {
    const base = cfgRef.current;
    if (!base) return;
    const servers = base.mediaServers.includes(name)
      ? base.mediaServers.filter((n) => n !== name)
      : [...base.mediaServers, name].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
    void save({ mediaServers: servers, mediaServersAuto: false });
  }

  if (failed) return null;
  // Drawn before its settings arrive, so a search result that opens this page
  // finds the card to mark.
  if (!cfg) return <Card title={t("streaming.title")} hint={t("streaming.hint")} hueIndex={hueIndex}>{null}</Card>;

  const byName = new Map(candidates.map((c) => [c.name, c]));
  const chosen = cfg.mediaServers;

  return (
    <Card title={t("streaming.title")} hint={t("streaming.hint")} hueIndex={hueIndex}>
      <ToggleRow
        label={t("streaming.toggle")}
        hint={t("streaming.toggleHint")}
        checked={cfg.enabled}
        onChange={(v) => void toggle(v)}
        disabled={busy}
        shakeNonce={shake}
        pulseNonce={pulse}
      />
      {cfg.enabled && streaming && (
        <p className="text-xs text-statusWarn" role="status">
          {t("streaming.now").replace("{name}", streaming)}
        </p>
      )}

      <div className="flex flex-col gap-2">
        <span className="flex items-center gap-1 text-xs text-carbon-textSub">
          {t("streaming.servers")}
          <InfoBubble tip={t("streaming.serversHint")} />
        </span>
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
                    onClick={() => toggleServer(c.name)}
                    className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-start transition-colors ${
                      checked ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                    }`}
                  >
                    <input type="checkbox" checked={checked} readOnly tabIndex={-1} className="pointer-events-none" style={{ accentColor: "var(--accent)" }} />
                    <span className="min-w-0 flex-1 truncate">{c.name}</span>
                    {c.hostNetwork && <span className="shrink-0 text-caption text-statusWarn">{t("streaming.hostNetwork")}</span>}
                  </button>
                );
              })}
            </>
          </DropdownListbox>
        </div>
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
                  onClick={() => toggleServer(n)}
                />
              </span>
            ))}
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {NUMBER_FIELDS.map(({ key, label, hint, max }) => (
          <label key={key} className="flex flex-col gap-1">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t(label)}
              <InfoBubble tip={t(hint)} />
            </span>
            <NumberField
              min={1}
              max={max}
              value={cfg[key]}
              onChange={(e) => setNumber(key, Math.min(max, Math.max(1, parseInt(e.target.value, 10) || 1)))}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
            />
          </label>
        ))}
      </div>
    </Card>
  );
}
