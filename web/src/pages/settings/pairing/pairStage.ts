// Where an instance stands in its group, read from GET /api/group. Kept apart
// from the card so the rules can be tested without drawing it.
import type { BadgeTone } from "../../../components/Badge";
import type { GroupState } from "../../../lib/api";
import type { TranslationKey } from "../../../lib/i18n";

/** How long a new member may take to show up before the card suggests why
 *  nobody has. Discovery and the relay both find a member within seconds. */
export const ALONE_AFTER_S = 60;

/** "gone" is a group whose members were there once and are unreachable now. */
export type PairStage = "unpaired" | "new" | "searching" | "alone" | "gone" | "paired";

/** pairStage reads the stage from the group, the seconds since this instance
 *  entered it and whether this page created it. Only a member found now
 *  counts as paired. */
export function pairStage(
  g: Pick<GroupState, "active" | "members" | "memberSeen">,
  joinedAgo: number,
  createdHere: boolean,
): PairStage {
  if (!g.active) return "unpaired";
  if (g.members.length > 0) return "paired";
  if (g.memberSeen) return "gone";
  if (joinedAgo >= ALONE_AFTER_S) return "alone";
  return createdHere ? "new" : "searching";
}

/** The badge each stage shows once there is a group. */
export const STAGE_BADGE: Record<Exclude<PairStage, "unpaired">, { tone: BadgeTone; key: TranslationKey }> = {
  new: { tone: "active", key: "pairing.stateNew" },
  searching: { tone: "neutral", key: "pairing.stateSearching" },
  gone: { tone: "neutral", key: "pairing.stateSearching" },
  alone: { tone: "warn", key: "pairing.stateAlone" },
  paired: { tone: "ok", key: "pairing.paired" },
};

/** clock writes seconds as minutes and seconds, 75 as "1:15". */
export function clock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
