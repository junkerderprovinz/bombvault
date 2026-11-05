// The two refusals a call that hands out or creates key material can meet
// before it is looked at: no login password is set, or the session is the one
// the phone app opens this instance with.
import { openedAsPairedPhone, openedThroughRelay, openedWithoutPassword, type SecretsRefusal } from "./api";
import type { TranslationKey } from "./i18n";

const SECRETS_SENTENCE: Record<SecretsRefusal, TranslationKey> = {
  "password-not-set": "secrets.passwordNotSet",
  "own-login-needed": "secrets.ownLoginNeeded",
};

/** secretsSentence is the sentence of such a refusal, undefined for any
 *  other answer. A page opened through the relay cannot sign in, so there it
 *  says where to go instead. */
export function secretsSentence(t: (key: TranslationKey) => string, code: string | undefined): string | undefined {
  if (code === "own-login-needed" && openedThroughRelay()) return t("auth.relayLoginNeeded");
  return code !== undefined && code in SECRETS_SENTENCE ? t(SECRETS_SENTENCE[code as SecretsRefusal]) : undefined;
}

/** deleteSentence is secretsSentence for a call that deletes backups for good,
 *  whose missing password is not about keys. */
export function deleteSentence(t: (key: TranslationKey) => string, code: string | undefined): string | undefined {
  return code === "password-not-set" ? t("secrets.deletePasswordNotSet") : secretsSentence(t, code);
}

/** flashZipRefusal says why the server would refuse the ZIP of a flash backup.
 *  The download is a plain link, and a refusal in its place would leave the page. */
export function flashZipRefusal(t: (key: TranslationKey) => string): string | undefined {
  if (openedWithoutPassword()) return secretsSentence(t, "password-not-set");
  return openedAsPairedPhone() ? t("restoreMany.flashOwnLogin") : undefined;
}
