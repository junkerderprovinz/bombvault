import { useT } from "../../lib/i18n";

type T = ReturnType<typeof useT>["t"];

/** The message of a request the server turned away, for example while a ZFS
 *  run holds the domain. */
export function failText(t: T, err: unknown): string {
  return err instanceof Error ? err.message : t("settings.error");
}
