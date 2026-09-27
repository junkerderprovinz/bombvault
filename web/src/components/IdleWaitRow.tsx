import { useEffect, useRef, useState } from "react";
import { setIdleWait } from "../lib/api";
import { useT } from "../lib/i18n";
import { useIdleWaits, IDLE_REASON_KEYS } from "../lib/idleWaits";
import { formatClockTime } from "../lib/reltime";
import { useToast } from "../lib/toast";
import { ToggleRow } from "../pages/settings/shared";
import { NumberField } from "./NumberField";

const DEFAULT_HOURS = 4;
const MAX_HOURS = 24;
const DEBOUNCE_MS = 800;

// IdleWaitRow is a container's "wait until the app is idle" option: a switch,
// and once it is on, the most hours a scheduled backup waits.
export function IdleWaitRow({ name, initial }: { name: string; initial: number }) {
  const { t } = useT();
  const { push } = useToast();
  const [hours, setHours] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  // The hours the switch brings back after it was off.
  const remembered = useRef(initial > 0 ? initial : DEFAULT_HOURS);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The hours a pending debounce would save, flushed when the card goes away.
  const pending = useRef<number | null>(null);

  useEffect(() => setHours(initial), [initial]);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
      if (pending.current !== null) void setIdleWait(name, pending.current).catch(() => undefined);
    },
    [name]
  );

  async function save(next: number): Promise<boolean> {
    try {
      const res = await setIdleWait(name, next);
      if (res.ok) return true;
      push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
    return false;
  }

  async function toggle(on: boolean) {
    const prev = hours;
    const next = on ? remembered.current : 0;
    if (timer.current) clearTimeout(timer.current);
    pending.current = null;
    setHours(next);
    setBusy(true);
    const ok = await save(next);
    setBusy(false);
    if (!ok) {
      setHours(prev);
      setShake((n) => n + 1);
    }
  }

  function changeHours(value: number) {
    const next = Math.min(MAX_HOURS, Math.max(1, value));
    remembered.current = next;
    setHours(next);
    if (timer.current) clearTimeout(timer.current);
    pending.current = next;
    timer.current = setTimeout(() => {
      pending.current = null;
      void save(next);
    }, DEBOUNCE_MS);
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <ToggleRow
        label={t("idle.toggle")}
        hint={t("idle.toggleHint")}
        checked={hours > 0}
        onChange={(v) => void toggle(v)}
        disabled={busy}
        shakeNonce={shake}
      />
      {hours > 0 && (
        <label className="flex items-center gap-2 text-xs text-carbon-textSub">
          {t("idle.maxHours")}
          <NumberField
            min={1}
            max={MAX_HOURS}
            value={hours}
            onChange={(e) => changeHours(parseInt(e.target.value, 10) || 1)}
            wrapperClassName="w-20"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1 w-full glim-field-focus"
          />
        </label>
      )}
    </div>
  );
}

// IdleWaitLine says, on a card, that its scheduled backup waits for the app.
export function IdleWaitLine({ name }: { name: string }) {
  const { t } = useT();
  const wait = useIdleWaits().find((w) => w.domain === "containers" && w.name === name);
  if (!wait) return null;
  return (
    <p className="text-xs text-statusWarn text-end" role="status">
      {t("idle.waiting")
        .replace("{reason}", t(IDLE_REASON_KEYS[wait.reason] ?? IDLE_REASON_KEYS.measuring))
        .replace("{time}", formatClockTime(wait.deadline, false))}
    </p>
  );
}
