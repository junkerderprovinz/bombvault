import { useEffect, useRef, useState } from "react";
import { getStreaming, setStreaming } from "./api";
import type { MediaCandidate, StreamingSettings } from "./api";
import type { useT } from "./i18n";
import { useToast } from "./toast";

const DEBOUNCE_MS = 800;
// The card shows who streams right now, so it asks again while it is open.
const POLL_MS = 15000;

export type StreamingNumberKey = "thresholdMbit" | "limitKiB" | "holdMin";

/**
 * useStreaming holds the settings that let off-site copies give way to a
 * media server's streams, and saves each change as it is made.
 */
export function useStreaming(t: ReturnType<typeof useT>["t"]) {
  const { push } = useToast();
  const [cfg, setCfg] = useState<StreamingSettings | null>(null);
  const [failed, setFailed] = useState(false);
  const [candidates, setCandidates] = useState<MediaCandidate[]>([]);
  const [streaming, setStreamingNow] = useState("");
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [pulse, setPulse] = useState(0);
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

  function setNumber(key: StreamingNumberKey, value: number) {
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

  async function toggleServer(name: string) {
    const base = cfgRef.current;
    if (!base) return;
    const servers = base.mediaServers.includes(name)
      ? base.mediaServers.filter((n) => n !== name)
      : [...base.mediaServers, name].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
    if (await save({ mediaServers: servers, mediaServersAuto: false })) return;
    const reverted = cfgRef.current
      ? { ...cfgRef.current, mediaServers: base.mediaServers, mediaServersAuto: base.mediaServersAuto }
      : null;
    cfgRef.current = reverted;
    setCfg(reverted);
  }

  return { cfg, failed, candidates, streaming, busy, shake, pulse, toggle, setNumber, toggleServer };
}

export type StreamingState = ReturnType<typeof useStreaming>;
