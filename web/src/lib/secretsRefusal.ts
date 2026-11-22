// The refusal a call that hands out or creates key material can meet before
// it is looked at: where a password is set, the session is the one the phone
// app opens this instance with.
import { openedAsPairedPhone, openedThroughRelay, type SecretsRefusal } from "./api";
import type { TranslationKey } from "./i18n";

const SECRETS_SENTENCE: Record<SecretsRefusal, TranslationKey> = {
  "own-login-needed": "secrets.ownLoginNeeded",
};

/** secretsSentence is the sentence of such a refusal, undefined for any
 *  other answer. A page opened through the relay cannot sign in, so there it
 *  says where to go instead. */
export function secretsSentence(t: (key: TranslationKey) => string, code: string | undefined): string | undefined {
  if (code === "own-login-needed" && openedThroughRelay()) return t("auth.relayLoginNeeded");
  return code !== undefined && code in SECRETS_SENTENCE ? t(SECRETS_SENTENCE[code as SecretsRefusal]) : undefined;
}

/** flashZipRefusal says why the server would refuse the ZIP of a flash backup.
 *  The download is a plain link, and a refusal in its place would leave the page. */
export function flashZipRefusal(t: (key: TranslationKey) => string): string | undefined {
  return openedAsPairedPhone() ? t("restoreMany.flashOwnLogin") : undefined;
}
