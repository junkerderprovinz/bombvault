// The kit's sign-in as BombVault talks to it: one client for the sign-in page, the sign-in tile and the password
// question.
import { createSignInApi, type SignInReauth, type SignInReauthState } from "@glimstone/core";
import { basePath } from "./base";

export const signIn = createSignInApi({ root: basePath() });

// The kit's client asks for the password only before its own changes. BombVault's API wants a fresh sign-in too,
// for a new MCP key or API token, a phrase and pairing phones, so one question serves both and SignInReauthWindow
// draws whichever is waiting.
function shareQuestion(kit: SignInReauth) {
  let own: SignInReauthState = { asking: false, password: "", busy: false, error: null, refused: 0 };
  const listeners = new Set<() => void>();
  let waiting: ((confirmed: boolean) => void) | null = null;
  const update = (change: Partial<SignInReauthState>) => {
    own = { ...own, ...change };
    listeners.forEach((fn) => fn());
  };
  const settle = (confirmed: boolean) => {
    const resume = waiting;
    waiting = null;
    update({ asking: false, password: "", busy: false, error: null });
    resume?.(confirmed);
  };
  const kitAsks = () => kit.state().asking;

  const reauth: SignInReauth = {
    state: () => (kitAsks() ? kit.state() : own),
    subscribe(listener) {
      listeners.add(listener);
      const stop = kit.subscribe(listener);
      return () => {
        listeners.delete(listener);
        stop();
      };
    },
    set: (password) => (kitAsks() ? kit.set(password) : update({ password, error: null })),
    async submit() {
      if (kitAsks()) return kit.submit();
      if (!own.asking || own.busy || own.password === "") return;
      update({ busy: true, error: null });
      const r = await signIn.confirm(own.password);
      if (r.ok) settle(true);
      else update({ busy: false, error: r.error, refused: own.refused + 1 });
    },
    cancel: () => (kitAsks() ? kit.cancel() : settle(false)),
  };

  let pending: Promise<boolean> | null = null;
  const ask = (): Promise<boolean> => {
    if (listeners.size === 0) return Promise.resolve(false);
    pending ??= new Promise<boolean>((resolve) => {
      waiting = resolve;
      update({ asking: true, password: "", error: null });
    }).finally(() => (pending = null));
    return pending;
  };
  return { reauth, ask };
}

const question = shareQuestion(signIn.reauth);

/** The client the reauth window draws, with BombVault's own question beside the kit's. */
export const reauthClient = { ...signIn, reauth: question.reauth };

/** askPassword asks for the password once more, and says whether it was confirmed. */
export const askPassword = question.ask;

/** wantsPassword reports whether an answer of BombVault's API asks for a fresh sign-in the password can give. */
export function wantsPassword(status: number, body: unknown): boolean {
  const b = body as { error?: unknown; askPassword?: unknown } | null;
  return status === 403 && b?.error === "signInAgain" && b.askPassword === true;
}
