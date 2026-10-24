// What a container's backup that waits for its app says about itself: why it
// waits, when it is tried again and what happens at the end of the wait.

import { formatLogDate } from "./activityLog";
import type { DeferredBackup } from "./api";
import type { useT } from "./i18n";
import { formatClockTime } from "./reltime";
import { runReason } from "./runReason";

type T = ReturnType<typeof useT>["t"];

/** deferralTime is the time of day of unix, with the date in front when it is
 *  not today. A deadline often falls on the next morning. */
export function deferralTime(unix: number, now: number = Date.now()): string {
  const at = new Date(unix * 1000);
  const today = new Date(now);
  const clock = formatClockTime(unix, false);
  return at.toDateString() === today.toDateString() ? clock : `${formatLogDate(unix * 1000)} ${clock}`;
}

/** deferralSentence is the whole word on a waiting backup: its reason, its
 *  next try and its deadline. */
export function deferralSentence(t: T, d: DeferredBackup, now: number = Date.now()): string {
  const waiting = t("deferral.waiting").replace("{reason}", runReason(d.reason, t)).replace("{time}", deferralTime(d.nextTry, now));
  const end = t(d.atDeadline === "back-up" ? "deferral.deadlineBackUp" : "deferral.deadlineGiveUp").replace(
    "{time}",
    deferralTime(d.deadline, now)
  );
  return `${waiting} ${end}`;
}
