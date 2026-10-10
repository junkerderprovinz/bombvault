import type { GroupMember, RelayMode, RelayState } from "../../../lib/api";

/** The routes the relay card offers. The server stores three modes, and
 *  "instance" is its own relay served by a paired BombVault. */
export type RelayForm = RelayMode | "instance";

/** Where a BombVault answers the instances that use it as their relay. */
const SERVE_PATH = "/relay/connect";

function servedByInstance(url: string, members: Pick<GroupMember, "address">[]): boolean {
  let relay: URL;
  try {
    relay = new URL(url);
  } catch {
    return false;
  }
  if (relay.pathname.replace(/\/+$/, "") === SERVE_PATH) return true;
  return members.some((m) => {
    try {
      return new URL(m.address).hostname === relay.hostname;
    } catch {
      return false;
    }
  });
}

/** relayForm tells the two kinds of own relay apart without a stored choice.
 *  An address on a paired instance, or at the path an instance serves its
 *  relay under, means a BombVault is the relay. Without an address this
 *  instance serving one says the same. */
export function relayForm(relay: Pick<RelayState, "mode" | "url" | "serve">, members: Pick<GroupMember, "address">[]): RelayForm {
  if (relay.mode !== "own") return relay.mode;
  const url = relay.url.trim();
  if (url === "") return relay.serve ? "instance" : "own";
  return servedByInstance(url, members) ? "instance" : "own";
}
