import { useId, useState } from "react";
import { setDeferral, type ActivityPreset, type AtDeadline, type DeferralPatch, type DeferralPolicy, type DeferredBackup } from "../../lib/api";
import { deferralSentence } from "../../lib/deferral";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { useReveal } from "../../lib/useReveal";
import { Card, ToggleRow } from "../../pages/settings/shared";
import { NumberField } from "../NumberField";
import { RevealInput } from "../RevealInput";
import { SelectField } from "../SelectField";
import { Selector } from "../Selector";
import { Rows, SettingRow } from "../storage/rows";
import { TimePicker } from "../TimePicker";

/** What a container that never saved a policy starts from, as the server
 *  has it. */
export const DEFAULT_DEFERRAL: DeferralPolicy = {
  retryMin: 30,
  windowFrom: "",
  windowTo: "",
  maxHours: 12,
  atDeadline: "give-up",
  preset: "",
  apiKeySet: false,
  pausedMin: 0,
  failOpen: true,
  netKiB: 0,
  netMin: 5,
};

// The media servers have their own names in every language.
const PRESETS: { value: ActivityPreset; name?: string }[] = [
  { value: "" },
  { value: "plex", name: "Plex" },
  { value: "jellyfin", name: "Jellyfin" },
  { value: "emby", name: "Emby" },
];

const NEW_WINDOW = { windowFrom: "00:00", windowTo: "08:00" };
const NEW_NET_KIB = 100;

const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus";

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, Number.isFinite(n) ? Math.round(n) : lo));
}

/** The policy as the server takes it, with every number inside its limits. */
function patchOf(p: DeferralPolicy): DeferralPatch {
  return {
    retryMin: clamp(p.retryMin, 5, 720),
    windowFrom: p.windowFrom,
    windowTo: p.windowTo,
    maxHours: clamp(p.maxHours, 1, 168),
    atDeadline: p.atDeadline,
    preset: p.preset,
    pausedMin: clamp(p.pausedMin, 0, 1440),
    failOpen: p.failOpen,
    netKiB: clamp(p.netKiB, 0, 1 << 20),
    netMin: clamp(p.netMin, 1, 60),
  };
}

/**
 * DeferralCard sets how a container's backup waits while its app is busy: a
 * media server asked whether anybody watches, the container's own traffic,
 * and when a waiting backup is tried again and until when. Every change saves
 * the whole policy, 800 ms after the last one, the way the commands above it
 * save; a failed save keeps what was typed and says so.
 */
export function DeferralCard({
  name,
  initial,
  waiting,
  hueIndex,
}: {
  name: string;
  initial: DeferralPolicy;
  /** The backup that waits now, if any. */
  waiting?: DeferredBackup;
  hueIndex: number;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { debouncedSave } = useDebouncedSave();
  const reveal = useReveal();
  const uid = useId();
  const [policy, setPolicy] = useState(initial);
  const [apiKey, setApiKey] = useState("");
  // The rate the switch brings back after it was off.
  const [lastKiB, setLastKiB] = useState(initial.netKiB > 0 ? initial.netKiB : NEW_NET_KIB);

  async function save(next: DeferralPatch): Promise<boolean> {
    try {
      const res = await setDeferral(name, next);
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

  function change(patch: Partial<DeferralPolicy>, extra: Partial<DeferralPatch> = {}) {
    const next = { ...policy, ...patch };
    setPolicy(next);
    debouncedSave(() => void save({ ...patchOf(next), ...extra }));
  }

  function changeKey(value: string) {
    setApiKey(value);
    debouncedSave(() => {
      if (value.trim() === "") return;
      void save({ ...patchOf(policy), apiKey: value.trim() }).then((ok) => {
        if (ok) setPolicy((p) => ({ ...p, apiKeySet: true }));
      });
    });
  }

  const preset = policy.preset;
  const windowOn = policy.windowFrom !== "";
  const netOn = policy.netKiB > 0;
  const needsKey = preset === "jellyfin" || preset === "emby";

  return (
    <Card title={t("deferral.title")} hint={t("deferral.hint")} hueIndex={hueIndex}>
      <Rows>
        <SettingRow
          label={t("deferral.preset")}
          hint={t("deferral.presetHint")}
          htmlFor={`${uid}-preset`}
          note={preset === "plex" ? t("deferral.plexToken") : undefined}
          newId="deferral-preset"
        >
          <SelectField
            id={`${uid}-preset`}
            label={t("deferral.preset")}
            value={preset}
            onChange={(value) => {
              // Without a server to ask, a stored key has nothing to open.
              change({ preset: value, apiKeySet: value === "" ? false : policy.apiKeySet }, value === "" ? { apiKeyClear: true } : {});
              if (value === "") setApiKey("");
            }}
            options={PRESETS.map((p) => ({ value: p.value, label: p.name ?? t("deferral.presetNone") }))}
            className={`${FIELD} w-48`}
          />
        </SettingRow>
        {needsKey && (
          <SettingRow label={t("deferral.apiKey")} hint={t("deferral.apiKeyHint")} htmlFor={`${uid}-key`}>
            <RevealInput
              {...reveal}
              id={`${uid}-key`}
              value={apiKey}
              onChange={(e) => changeKey(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder={policy.apiKeySet ? t("cloud.secretSet") : ""}
              wrapperClassName="w-72 max-w-full"
              className={FIELD}
            />
          </SettingRow>
        )}
        {preset !== "" && (
          <>
            <SettingRow label={t("deferral.paused")} hint={t("deferral.pausedHint")} htmlFor={`${uid}-paused`} narrow>
              <NumberField
                id={`${uid}-paused`}
                min={0}
                max={1440}
                value={policy.pausedMin}
                onChange={(e) => change({ pausedMin: parseInt(e.target.value, 10) || 0 })}
                wrapperClassName="w-28"
                className={FIELD}
              />
            </SettingRow>
            <div className="py-3">
              <ToggleRow
                label={t("deferral.failOpen")}
                hint={t("deferral.failOpenHint")}
                checked={policy.failOpen}
                onChange={(on) => change({ failOpen: on })}
              />
            </div>
          </>
        )}
        <div data-new="deferral-net" className="py-3">
          <ToggleRow
            label={t("deferral.net")}
            hint={t("deferral.netHint")}
            checked={netOn}
            onChange={(on) => change({ netKiB: on ? lastKiB : 0 })}
          />
        </div>
        {netOn && (
          <>
            <SettingRow label={t("deferral.netKiB")} hint={t("deferral.netKiBHint")} htmlFor={`${uid}-kib`} narrow>
              <NumberField
                id={`${uid}-kib`}
                min={1}
                max={1 << 20}
                value={policy.netKiB}
                onChange={(e) => {
                  const kib = Math.max(1, parseInt(e.target.value, 10) || 1);
                  setLastKiB(kib);
                  change({ netKiB: kib });
                }}
                wrapperClassName="w-32"
                className={FIELD}
              />
            </SettingRow>
            <SettingRow label={t("deferral.netMin")} hint={t("deferral.netMinHint")} htmlFor={`${uid}-netmin`} narrow>
              <NumberField
                id={`${uid}-netmin`}
                min={1}
                max={60}
                value={policy.netMin}
                onChange={(e) => change({ netMin: parseInt(e.target.value, 10) || 1 })}
                wrapperClassName="w-28"
                className={FIELD}
              />
            </SettingRow>
          </>
        )}
        <SettingRow label={t("deferral.retryMin")} hint={t("deferral.retryMinHint")} htmlFor={`${uid}-retry`} newId="deferral-retry" narrow>
          <NumberField
            id={`${uid}-retry`}
            min={5}
            max={720}
            value={policy.retryMin}
            onChange={(e) => change({ retryMin: parseInt(e.target.value, 10) || 5 })}
            wrapperClassName="w-28"
            className={FIELD}
          />
        </SettingRow>
        <div className="py-3">
          <ToggleRow
            label={t("deferral.window")}
            hint={t("deferral.windowHint")}
            checked={windowOn}
            onChange={(on) => change(on ? NEW_WINDOW : { windowFrom: "", windowTo: "" })}
          />
        </div>
        {windowOn && (
          <SettingRow label={t("deferral.windowTimes")} hint={t("deferral.windowTimesHint")}>
            <TimePicker
              label={t("deferral.windowStart")}
              value={policy.windowFrom}
              onChange={(windowFrom) => change({ windowFrom })}
              hueIndex={hueIndex}
            />
            <span className="text-sm text-carbon-textSub">{t("deferral.windowAnd")}</span>
            <TimePicker
              label={t("deferral.windowEnd")}
              value={policy.windowTo}
              onChange={(windowTo) => change({ windowTo })}
              hueIndex={hueIndex}
            />
          </SettingRow>
        )}
        <SettingRow label={t("idle.maxHours")} hint={t("deferral.maxHoursHint")} htmlFor={`${uid}-hours`} narrow>
          <NumberField
            id={`${uid}-hours`}
            min={1}
            max={168}
            value={policy.maxHours}
            onChange={(e) => change({ maxHours: parseInt(e.target.value, 10) || 1 })}
            wrapperClassName="w-28"
            className={FIELD}
          />
        </SettingRow>
        <SettingRow label={t("deferral.atDeadline")} hint={t("deferral.atDeadlineHint")} narrow>
          <Selector
            items={[
              { id: "give-up", label: t("deferral.giveUp") },
              { id: "back-up", label: t("deferral.backUp") },
            ]}
            label={t("deferral.atDeadline")}
            size="sm"
            inline
            select="one"
            activation="manual"
            active={policy.atDeadline}
            onChange={(id) => change({ atDeadline: id as AtDeadline })}
          />
        </SettingRow>
      </Rows>
      {waiting && (
        <p className="mt-3 text-xs text-statusWarn" role="status">
          {deferralSentence(t, waiting)}
        </p>
      )}
    </Card>
  );
}
