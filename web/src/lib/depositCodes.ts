// Why the server refused a key deposit call, as a sentence. The map is
// written out so that tsc and the orphan test see every key.
import type { DepositRefusal, SecretsRefusal } from "./api";
import type { TranslationKey } from "./i18n";
import { secretsSentence } from "./secretsRefusal";

/** The codes of the server's deposit table. "deposit-changed" is only
 *  recorded in the activity log, and the two refusals for the session or the
 *  missing password come from another gate and have sentences of their own. */
type DepositCode = Exclude<DepositRefusal, SecretsRefusal> | "deposit-changed";

export const DEPOSIT_CODE_KEY: Record<DepositCode, TranslationKey> = {
  "keeper-too-old": "deposit.code.keeper-too-old",
  "deposit-none": "deposit.code.deposit-none",
  "release-declined": "deposit.code.release-declined",
  "release-limit": "deposit.code.release-limit",
  "deposit-version": "deposit.code.deposit-version",
  "deposit-code-shape": "deposit.code.deposit-code-shape",
  "deposit-code": "deposit.code.deposit-code",
  "member-gone": "deposit.code.member-gone",
  "member-silent": "deposit.code.member-silent",
  "backup-key-locked": "deposit.code.backup-key-locked",
  "release-decided": "deposit.code.release-decided",
  "keeper-full": "deposit.code.keeper-full",
  "deposit-owner": "deposit.code.deposit-owner",
  "deposit-unreadable": "deposit.code.deposit-unreadable",
  "deposit-busy": "deposit.code.deposit-busy",
  "deposit-changed": "deposit.code.deposit-changed",
};

/** depositSentence says why a deposit call failed. A code this build does not
 *  know falls back to the server's own sentence, so a newer server still says
 *  something true. */
export function depositSentence(t: (key: TranslationKey) => string, answer: { code?: string; error?: string }): string {
  if (isRefusal(answer.code)) return t(DEPOSIT_CODE_KEY[answer.code]);
  const secrets = secretsSentence(t, answer.code);
  if (secrets) return secrets;
  return answer.error ?? t("instances.role.saveError");
}

function isRefusal(code: string | undefined): code is DepositCode {
  return code !== undefined && code in DEPOSIT_CODE_KEY;
}
