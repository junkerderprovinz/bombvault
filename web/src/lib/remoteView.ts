import { useInstanceScope } from "./instanceScope";

// The single list a control checks before it renders in remote view. It
// mirrors internal/api/group_remote.go's remoteViewRoutes() by hand: every
// key here is a trigger the server actually forwards for a paired member.
// A control for anything else, present or added later, stays hidden while
// remote by construction, since canAct() returns false for a key it does
// not recognise.
//
// Keys are named after the page and the action, not after the wire route,
// so a component reads naturally: canAct("containers.backup"). Reads (the
// lists and metadata remote view shows) need no key: a page reachable in
// remote view at all already had its GET routes allowlisted server-side: a
// page that could not read at all would not be linked into remote view's
// navigation in the first place. This list is only for the triggers, and for
// every other interactive control (restore, delete, export, edit, add,
// import), which is what the switch actually gates.
const REMOTE_VIEW_ACTIONS = new Set([
  "containers.backup",
  "containers.backupAll",
  "vms.backup",
  "files.backup",
  "files.backupAll",
  "zfs.backup",
  "zfs.backupAll",
  "flash.backup",
  "config.backup",
  "backupEverything",
  "backup.cancel",
  "check",
  "verify",
  "offsite.replicateNow",
  "pull.run",
  "receiver.check",
  "fleet.poll",
]);

/** Whether action is one of remote view's own triggers, forwarded to a
 *  paired member's real route. */
export function remoteViewCanAct(action: string): boolean {
  return REMOTE_VIEW_ACTIONS.has(action);
}

/**
 * The gate every page reachable in remote view renders its controls behind.
 * `remote` is whether a peer's instance is open at all; `canAct(action)` is
 * whether one specific trigger may show while it is. A control with no
 * `action` (an edit form, an add button, a delete, a download) simply checks
 * `remote` itself and hides outright: nothing in remote view creates,
 * changes or removes anything beyond the fixed trigger list, so an
 * unrecognised control has no action to name and stays local by default.
 */
export function useRemoteView(): { remote: boolean; canAct: (action: string) => boolean } {
  const { remote } = useInstanceScope();
  return { remote, canAct: (action: string) => !remote || remoteViewCanAct(action) };
}
