// The words GlimStone's sign-in parts take, out of BombVault's language files, and the codes the kit answers with,
// each with a sentence of its own.
import {
  componentText,
  listOf,
  SIGN_IN_ERRORS,
  SIGN_IN_REDIRECTS,
  signInTileTextEn,
  type Plural,
  type SignInErrorText,
  type SignInReauthText,
  type SignInRemoved,
  type SignInText,
  type SignInTileText,
} from "@glimstone/core";
import { countForms, type TranslationKey } from "./i18n";

type T = (key: TranslationKey, n?: number) => string;

// Codes a browser answers with beside the kit's.
export const ERROR_CODES = [...SIGN_IN_ERRORS, "offline", "cancelled"] as const;
export const ARRIVED_CODES = SIGN_IN_REDIRECTS;

const key = (k: string) => `signin.${k}` as TranslationKey;
const counted = (k: string) => countForms(key(k)) as Plural;

function eye(lang: string) {
  const field = componentText(lang).passwordField;
  return { show: field.show, hide: field.hide };
}

// Codes that say what another key says already, in every language.
const SAME_AS: Record<string, string> = {
  "errors.notJSON": "errors.badRequest",
  "errors.signInAgain": "errors.confirmOff",
  "errors.serviceUntested": "tile.services.mustTest",
  "arrived.serviceOff": "errors.serviceOff",
  "arrived.serviceUnknown": "errors.serviceUnknown",
};
const said = (t: T, k: string) => t(`signin.${SAME_AS[k] ?? k}` as TranslationKey);

export const errorWords = (t: T): SignInErrorText => Object.fromEntries(ERROR_CODES.map((code) => [code, said(t, "errors." + code)]));
export const arrivedWords = (t: T): Record<string, string> => Object.fromEntries(ARRIVED_CODES.map((code) => [code, said(t, "arrived." + code)]));

export function cardText(t: T, lang: string): SignInText {
  const s = (k: string) => t(`signin.${k}` as TranslationKey);
  const field = componentText(lang).passwordField;
  return {
    name: s("name"),
    password: s("password"),
    signIn: s("signIn"),
    signingIn: s("signingIn"),
    passkey: "",
    forgot: s("forgot"),
    code: s("codeLabel"),
    codeHint: s("codeTip"),
    codeInvalid: s("badCode"),
    back: s("back"),
    resetCode: s("logCode"),
    resetCodeTip: t("auth.resetCodeTip"),
    newPassword: s("newPassword"),
    confirmPassword: field.confirm,
    setPassword: s("setPassword"),
    saving: s("saving"),
    mismatch: field.mismatch,
    tooShort: "",
    shortHint: s("shortHint"),
    resetRejected: s("resetRejected"),
    failed: s("failed"),
    ...eye(lang),
  };
}

/** What the page after a reset says: the sessions that ended and whatever else the reset took away. */
export function resetNote(t: T, lang: string, removed: SignInRemoved): string {
  const gone = [
    ...(removed.twoFactor ? [t("signin.tile.twoFactor.title")] : []),
    ...(removed.passkeys ? [t("signin.tile.password.goes.passkeys", removed.passkeys)] : []),
  ];
  return [
    t("signin.resetDone"),
    ...(gone.length ? [t("signin.resetGone").replace("{gone}", listOf(lang, gone))] : []),
    ...(removed.services ? [t("signin.resetServices")] : []),
    t("auth.resetDonePhones") + ".",
    t("auth.resetDoneHint"),
  ].join(" ");
}

export function reauthText(t: T, lang: string): SignInReauthText {
  const s = (k: string) => t(`signin.${k}` as TranslationKey);
  return {
    title: s("reauth.title"),
    text: s("reauth.text"),
    password: s("password"),
    confirm: s("reauth.confirm"),
    cancel: t("common.cancel"),
    close: t("common.close"),
    ...eye(lang),
    wrong: s("errors.wrongPassword"),
    off: s("errors.confirmOff"),
    failed: s("failed"),
  };
}

export function tileText(t: T, lang: string): SignInTileText {
  const s = (k: string) => t(`signin.${k}` as TranslationKey);
  const tile = (k: string) => s("tile." + k);
  const field = componentText(lang).passwordField;
  return {
    cancel: t("common.cancel"),
    close: t("common.close"),
    confirm: tile("confirm"),
    copy: t("common.copy"),
    copied: t("common.copied"),
    back: s("back"),
    save: t("settings.save"),
    remove: s("remove"),
    ...eye(lang),
    confirmPassword: field.confirm,
    mismatch: field.mismatch,
    failed: s("failed"),
    needsPassword: { title: tile("needsPassword.title"), reason: tile("needsPassword.reason") },
    notLocal: { title: tile("notLocal.title"), reason: tile("notLocal.reason") },
    password: {
      title: s("password"),
      hint: tile("password.hint"),
      openHint: tile("password.openHint"),
      viaService: tile("password.viaService"),
      state: s("password"),
      set: tile("password.set"),
      none: tile("password.none"),
      noneHint: tile("password.noneHint"),
      change: tile("password.change"),
      add: tile("password.add"),
      current: tile("password.current"),
      next: s("newPassword"),
      save: tile("password.save"),
      saved: tile("password.saved"),
      first: tile("password.first"),
      wrongCurrent: tile("password.wrongCurrent"),
      shortHint: s("shortHint"),
      shortSaved: s("longer"),
      remove: tile("password.remove"),
      removeTitle: tile("password.removeTitle"),
      removeText: tile("password.removeText"),
      goes: {
        twoFactor: tile("twoFactor.title"),
        passkeys: counted("tile.password.goes.passkeys"),
        services: tile("password.goes.services"),
        tokens: counted("tile.password.goes.tokens"),
      },
      removed: tile("password.removed"),
    },
    askPassword: {
      title: tile("askPassword.title"),
      hint: tile("askPassword.hint"),
      label: tile("askPassword.label"),
      when: tile("askPassword.when"),
      on: tile("askPassword.on"),
      off: tile("askPassword.off"),
    },
    twoFactor: {
      title: tile("twoFactor.title"),
      hint: tile("twoFactor.hint"),
      state: tile("twoFactor.state"),
      on: tile("twoFactor.on"),
      none: tile("twoFactor.none"),
      noneHint: tile("twoFactor.noneHint"),
      setUp: tile("twoFactor.setUp"),
      codesLeft: tile("twoFactor.codes"),
      codesLeftValue: counted("tile.twoFactor.codesLeft"),
      newCodes: tile("twoFactor.newCodes"),
      turnOff: tile("twoFactor.turnOff"),
      setupTitle: tile("twoFactor.setupTitle"),
      scan: tile("twoFactor.scan"),
      qr: tile("twoFactor.qr"),
      key: tile("twoFactor.key"),
      code: tile("twoFactor.code"),
      confirm: tile("twoFactor.turnOn"),
      sixDigits: tile("twoFactor.sixDigits"),
      codesTitle: tile("twoFactor.codes"),
      codesText: tile("twoFactor.codesText"),
      copyCodes: tile("twoFactor.copyCodes"),
      codesSaved: tile("twoFactor.codesSaved"),
      turnedOn: tile("twoFactor.turnedOn"),
      newCodesReady: tile("twoFactor.newCodesReady"),
      offTitle: tile("twoFactor.offTitle"),
      offText: tile("twoFactor.offText"),
      offConfirm: tile("twoFactor.turnOff"),
      turnedOff: tile("twoFactor.turnedOff"),
    },
    passkeys: {
      title: tile("passkeys.title"),
      hint: tile("passkeys.hint"),
      add: tile("passkeys.add"),
      none: tile("passkeys.none"),
      noneHint: tile("passkeys.noneHint"),
      created: tile("passkeys.created"),
      used: tile("passkeys.used"),
      never: tile("passkeys.never"),
      here: tile("passkeys.here"),
      other: tile("passkeys.other"),
      synced: tile("passkeys.synced"),
      rename: tile("passkeys.rename"),
      renameTitle: tile("passkeys.renameTitle"),
      removeText: tile("passkeys.removeText"),
      removed: tile("passkeys.removed"),
      added: tile("passkeys.added"),
      renamed: tile("passkeys.renamed"),
      addTitle: tile("passkeys.add"),
      name: s("name"),
      nameHint: tile("passkeys.nameHint"),
      fallbackName: tile("passkeys.fallbackName"),
      continue: tile("passkeys.continue"),
      unavailableTitle: tile("passkeys.unavailableTitle"),
      unavailable: tile("passkeys.unavailable"),
    },
    sessions: {
      title: tile("sessions.title"),
      hint: tile("sessions.hint"),
      current: tile("sessions.current"),
      end: s("logout"),
      endOthers: tile("sessions.endOthers"),
      ended: tile("sessions.ended"),
      othersEnded: tile("sessions.othersEnded"),
      from: tile("sessions.from"),
      seen: tile("sessions.seen"),
      on: tile("sessions.on"),
      browser: tile("sessions.browser"),
    },
    // API tokens are BombVault's own, under Integrations, so the tile never draws the kit's.
    tokens: signInTileTextEn.tokens,
    services: {
      title: tile("services.title"),
      unnamed: tile("services.unnamed"),
      hint: tile("services.hint"),
      add: tile("services.add"),
      on: tile("twoFactor.on"),
      off: tile("twoFactor.off"),
      untested: tile("services.untested"),
      paused: tile("services.paused"),
      show: tile("services.show"),
      mustTest: tile("services.mustTest"),
      test: tile("services.test"),
      testing: tile("services.testing"),
      testOk: tile("services.testOk"),
      testFailed: tile("services.testFailed"),
      tested: tile("services.tested"),
      fromConfig: tile("services.fromConfig"),
      confirmAgain: tile("services.confirmAgain"),
      edit: tile("services.edit"),
      openBinds: tile("services.openBinds"),
      removeText: tile("services.removeText"),
      removed: tile("services.removed"),
      turnedOn: tile("services.turnedOn"),
      turnedOff: tile("services.turnedOff"),
      updated: tile("services.updated"),
      pick: tile("services.add"),
      pickHint: tile("services.pickHint"),
      group: tile("services.group"),
      own: tile("services.own"),
      ownSub: "OpenID Connect",
      ownHint: tile("services.ownHint"),
      search: tile("services.search"),
      searchNone: tile("services.searchNone"),
      clear: tile("services.clear"),
      saved: tile("services.saved"),
      formHint: tile("services.formHint"),
      whoGetsIn: tile("services.whoGetsIn"),
      role: tile("services.role"),
      roleHint: tile("services.roleHint"),
      issuer: tile("services.issuer"),
      issuerHint: tile("services.issuerHint"),
      clientId: tile("services.clientId"),
      secret: tile("services.secret"),
      secretKept: tile("services.secretKept"),
      redirect: tile("services.redirect"),
      redirectHint: tile("services.redirectHint"),
      label: tile("services.label"),
      scopes: tile("services.scopes"),
      nameClaim: tile("services.nameClaim"),
      groupsClaim: tile("services.groupsClaim"),
      more: tile("services.more"),
      turnOn: tile("twoFactor.turnOn"),
      missing: tile("services.missing"),
      unreachable: tile("services.unreachable"),
    },
    accounts: {
      title: tile("accounts.title"),
      hint: tile("accounts.hint"),
      owner: tile("accounts.owner"),
      blocked: tile("accounts.blocked"),
      block: tile("accounts.block"),
      unblock: tile("accounts.unblock"),
      seen: tile("accounts.seen"),
      created: tile("accounts.created"),
      groups: tile("accounts.groups"),
      removeText: tile("accounts.removeText"),
      removed: tile("accounts.removed"),
      blockedDone: tile("accounts.blockedDone"),
      unblocked: tile("accounts.unblocked"),
    },
    roles: { admin: s("role.admin") },
    via: {
      open: tile("via.open"),
      password: s("password"),
      passkey: tile("via.passkey"),
      service: "{name}",
      token: tile("via.token"),
      door: tile("via.door"),
    },
    errors: errorWords(t),
    arrived: arrivedWords(t),
  };
}
