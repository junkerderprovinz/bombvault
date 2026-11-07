// The sign-in card's flow, after GlimStone's core ("The sign-in card" in its
// design language). The page answers each step through a callback, the flow
// keeps what was typed and which form shows, and components/SignInCard.tsx
// draws it.

import { shake } from "./shake";

/** The card's words, translated by the page. */
export interface SignInText {
  password: string;
  signIn: string;
  /** The bubble on Sign in while it works. */
  signingIn: string;
  passkey: string;
  /** "Forgot password?", with the question mark. */
  forgot: string;
  /** The label of the second step's code field. */
  code: string;
  /** The line under the code field, such as where the code comes from. */
  codeHint: string;
  /** Said when the page refuses a code without a reason of its own. */
  codeInvalid: string;
  back: string;
  /** The label of the reset form's code field. */
  resetCode: string;
  /** The (i) beside it: where the log is, how long the code lasts, what a reset removes. */
  resetCodeTip: string;
  newPassword: string;
  confirmPassword: string;
  setPassword: string;
  /** The bubble on Set password while it works. */
  saving: string;
  mismatch: string;
  /** Says how many characters a new password needs, with the number filled in. */
  tooShort: string;
  /** Said when the page refuses the code from the log. */
  resetRejected: string;
  /** Said when a callback throws or answers without a reason. */
  failed: string;
}

/**
 * What the page answers to a sign-in or a passkey. `ok` means the card's part
 * is done: the page signs in or shows a step of its own. `needCode` asks for
 * the second step, and `error` is said above Sign in.
 */
export interface SignInAnswer {
  ok?: boolean;
  needCode?: boolean;
  error?: string;
}

/** What the page answers to asking for a reset code or to setting the password. `rejected` means the code from the log was refused. */
export interface ResetAnswer {
  ok?: boolean;
  rejected?: boolean;
  error?: string;
}

export interface SignInHandlers {
  /** `code` is set once the page asked for one, and `passkey` when a passkey started the sign-in. */
  onSignIn: (input: { password: string; code?: string; passkey: boolean }) => Promise<SignInAnswer>;
  /** An empty answer leaves the card as it was. */
  onPasskey?: () => Promise<SignInAnswer>;
  /** Writes a code to the log. The reset form shows only once this answers ok. */
  onAskCode?: () => Promise<ResetAnswer>;
  onSetPassword?: (input: { code: string; password: string }) => Promise<ResetAnswer>;
}

export type SignInField = "password" | "code" | "resetCode" | "newPassword" | "confirm";

export type SignInButton = "submit" | "set-password" | "passkey" | "forgot";

/**
 * The last refused step: the button that shakes and the field that takes the
 * focus with its content selected. `count` grows with every refusal, so the
 * same refusal twice still shakes twice.
 */
export interface SignInRefusal {
  button: SignInButton;
  field: SignInField | null;
  count: number;
}

export interface SignInState {
  view: "signIn" | "reset";
  password: string;
  code: string;
  needCode: boolean;
  /** A passkey started this sign-in, so the code goes without the password. */
  viaPasskey: boolean;
  resetCode: string;
  newPassword: string;
  confirm: string;
  error: string | null;
  refusal: SignInRefusal | null;
  busy: "signIn" | "passkey" | "askCode" | "setPassword" | null;
}

export interface SignInOptions extends SignInHandlers {
  text: SignInText;
  /** The shortest new password the reset form takes. */
  minPassword: number;
}

export interface SignInFlow {
  state(): SignInState;
  subscribe(listener: () => void): () => void;
  set(field: SignInField, value: string): void;
  submit(): Promise<void>;
  passkey(): Promise<void>;
  forgot(): Promise<void>;
  back(): void;
}

const START: SignInState = {
  view: "signIn",
  password: "",
  code: "",
  needCode: false,
  viaPasskey: false,
  resetCode: "",
  newPassword: "",
  confirm: "",
  error: null,
  refusal: null,
  busy: null,
};

const BUTTON_OF: Record<NonNullable<SignInState["busy"]>, SignInButton> = {
  signIn: "submit",
  setPassword: "set-password",
  passkey: "passkey",
  askCode: "forgot",
};

/**
 * The flow behind one card. `options` is read at every step, so the card hands
 * in its current props rather than the ones it was made with.
 */
export function createSignInFlow(options: () => SignInOptions): SignInFlow {
  let state = START;
  const listeners = new Set<() => void>();
  const update = (change: Partial<SignInState>) => {
    state = { ...state, ...change };
    listeners.forEach((fn) => fn());
  };
  // A refusal during a step waits for the step to end: the busy button wears a
  // bubble that wraps it, and the shake has to land on the button that stays.
  let pending: Partial<SignInState> = {};
  const refuse = (error: string, button: SignInButton, field: SignInField | null = null) => {
    const change = { error, refusal: { button, field, count: (state.refusal?.count ?? 0) + 1 } };
    if (state.busy) pending = change;
    else update(change);
  };

  async function run(busy: NonNullable<SignInState["busy"]>, work: () => Promise<void>) {
    if (state.busy) return;
    update({ busy, error: null });
    pending = {};
    try {
      await work();
    } catch {
      refuse(options().text.failed, BUTTON_OF[busy]);
    } finally {
      update({ busy: null, ...pending });
    }
  }

  async function signIn() {
    if (signInBlocked(state)) return;
    const { text, onSignIn } = options();
    return run("signIn", async () => {
      const tried = state.needCode;
      const res = await onSignIn({
        password: state.viaPasskey ? "" : state.password,
        code: tried ? state.code.trim() : undefined,
        passkey: state.viaPasskey,
      });
      if (res.ok) return;
      if (res.needCode) {
        // The first request for a code only opens the field.
        update({ needCode: true, code: "" });
        if (tried) refuse(res.error ?? text.codeInvalid, "submit", "code");
        return;
      }
      // A passkey's sign-in that ended starts over with the password.
      if (state.viaPasskey) update({ viaPasskey: false, needCode: false, code: "" });
      refuse(res.error ?? text.failed, "submit", state.needCode ? "code" : "password");
    });
  }

  async function setPassword() {
    const { text, minPassword, onSetPassword, onAskCode } = options();
    if (resetBlocked(state) || !onSetPassword) return;
    if (state.newPassword !== state.confirm) return refuse(text.mismatch, "set-password", "confirm");
    if ([...state.newPassword].length < minPassword) return refuse(text.tooShort, "set-password", "newPassword");
    return run("setPassword", async () => {
      const res = await onSetPassword({ code: state.resetCode.trim(), password: state.newPassword });
      if (res.ok) return;
      if (res.rejected) {
        // The answer does not say whether the code expired, and the sentence
        // promises a new one in that case. Asking writes one only when none is live.
        await onAskCode?.().catch(() => undefined);
        refuse(text.resetRejected, "set-password", "resetCode");
        return;
      }
      refuse(res.error ?? text.failed, "set-password", "resetCode");
    });
  }

  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    set(field, value) {
      update({ [field]: value });
    },
    submit: () => (state.view === "reset" ? setPassword() : signIn()),
    async passkey() {
      const { onPasskey } = options();
      if (!onPasskey) return;
      return run("passkey", async () => {
        const res = await onPasskey();
        if (res.ok) return;
        if (res.needCode) update({ viaPasskey: true, needCode: true, code: "" });
        else if (res.error !== undefined) refuse(res.error, "passkey");
      });
    },
    async forgot() {
      const { text, onAskCode } = options();
      if (!onAskCode) return;
      return run("askCode", async () => {
        const res = await onAskCode();
        if (res.ok) update({ view: "reset", resetCode: "", newPassword: "", confirm: "" });
        else refuse(res.error ?? text.failed, "forgot");
      });
    },
    back() {
      if (state.busy) return;
      update({ view: "signIn", needCode: false, viaPasskey: false, code: "", error: null });
    },
  };
}

/** Sign in waits for the password, and in the second step for the code. */
export function signInBlocked(s: SignInState): boolean {
  if (s.busy) return true;
  if (s.needCode) return s.code.trim() === "";
  return s.password === "";
}

/** Set password waits for the code from the log and a new password. */
export function resetBlocked(s: SignInState): boolean {
  return !!s.busy || s.resetCode.trim() === "" || s.newPassword === "";
}

/** Which form shows, as a key that changes whenever the focus should move to the first empty field of the new form. */
export function signInStage(s: SignInState): string {
  return `${s.view}:${s.needCode}:${s.viaPasskey}`;
}

// Walks up rather than reading the layout, which a page without one, such as a test's, does not have.
function shown(el: Element): boolean {
  for (let e: Element | null = el; e; e = e.parentElement) {
    if ((e as HTMLElement).hidden || getComputedStyle(e).display === "none") return false;
  }
  return true;
}

const FIELDS = 'input:not([type="hidden"], [type="checkbox"], [type="radio"], :disabled), textarea:not(:disabled)';

/**
 * Puts the focus in the first empty field inside `root`, or in its first field
 * when all hold something.
 */
export function focusFirstField(root: HTMLElement | null): void {
  if (!root) return;
  const fields = [...root.querySelectorAll<HTMLInputElement>(FIELDS)].filter(shown);
  (fields.find((f) => f.value === "") ?? fields[0])?.focus({ preventScroll: true });
}

/**
 * Answers a refusal on screen: `button` shakes once (`glim-shake`, which the
 * motion engine stills) and the refused field takes the focus with its content
 * selected, so the next try starts by typing. Returns the shake's cleanup.
 */
export function showRefusal(root: HTMLElement | null, refusal: SignInRefusal, button: HTMLElement | null): () => void {
  const field = refusal.field && root?.querySelector<HTMLInputElement>(`[data-signin-field="${refusal.field}"]`);
  if (field) {
    field.focus({ preventScroll: true });
    field.select();
  }
  return button ? shake(button) : () => {};
}
