// The two refusals a call that hands out or creates key material can meet
// before it is looked at: no login password is set, or the session is the one
// the phone app opens this instance with.
import type { SecretsRefusal } from "./api";
import type { TranslationKey } from "./i18n";

const SECRETS_SENTENCE: Record<SecretsRefusal, TranslationKey> = {
  "password-not-set": "secrets.passwordNotSet",
  "own-login-needed": "secrets.ownLoginNeeded",
};

/** secretsSentence is the sentence of such a refusal, undefined for any
 *  other answer. */
export function secretsSentence(t: (key: TranslationKey) => string, code: string | undefined): string | undefined {
  return code !== undefined && code in SECRETS_SENTENCE ? t(SECRETS_SENTENCE[code as SecretsRefusal]) : undefined;
}

/** deleteSentence is secretsSentence for a call that deletes backups for good,
 *  whose missing password is not about keys. */
export function deleteSentence(t: (key: TranslationKey) => string, code: string | undefined): string | undefined {
  return code === "password-not-set" ? t("secrets.deletePasswordNotSet") : secretsSentence(t, code);
}
