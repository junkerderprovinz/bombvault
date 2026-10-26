import { humanBytes } from "./forecast";
import { oneDecimal } from "./numberFormat";

/** rateHint writes a rate given in KiB/s, the unit restic's limits take, in
 *  bytes and in bits: 2048 is "≈ 2.0 MB/s (16.8 Mbit/s)". Zero means no limit
 *  and gives an empty string. */
export function rateHint(kib: number): string {
  if (kib <= 0) return "";
  const bits = kib * 1024 * 8;
  const bitRate = bits < 1e6 ? `${oneDecimal(bits / 1e3)} kbit/s` : `${oneDecimal(bits / 1e6)} Mbit/s`;
  return `≈ ${humanBytes(kib * 1024)}/s (${bitRate})`;
}
