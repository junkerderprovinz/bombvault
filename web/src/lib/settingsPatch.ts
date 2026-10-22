import { getSettings, putSettings, type PutSettingsResponse, type Settings } from "./api";

export interface SettingsPatchResult {
  res: PutSettingsResponse;
  /** The object that went out with the answer in res. */
  sent: Settings;
  /** The revision the page holds now: the one the save left, or the one of
   *  the settings read again. */
  revision?: string;
  /** The settings as read again when the copy the patch was laid on was
   *  stale, so the caller can take what changed elsewhere. */
  fresh?: Settings;
}

/** putSettingsPatch sends patch on top of base, the settings as read at
 *  revision. The PUT carries the whole object, so when the settings changed
 *  elsewhere since, the patch goes once more on top of a fresh read instead:
 *  sending base would write its old values back over that change. */
export async function putSettingsPatch(
  base: Settings,
  revision: string | undefined,
  patch: Partial<Settings>
): Promise<SettingsPatchResult> {
  const sent: Settings = { ...base, ...patch };
  const res = await putSettings(sent, revision);
  if (res.ok || res.code !== "settings-stale") {
    return { res, sent, revision: res.ok ? res.revision : revision };
  }
  const read = await getSettings();
  if (!read.ok) return { res, sent, revision };
  const again: Settings = { ...read.settings, ...patch };
  const second = await putSettings(again, read.revision);
  return {
    res: second,
    sent: again,
    revision: second.ok ? second.revision : read.revision,
    fresh: read.settings,
  };
}
