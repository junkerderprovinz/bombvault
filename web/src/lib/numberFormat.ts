// Numbers follow the language the page is shown in. The language provider
// keeps it on <html lang>, so a formatter called outside React reads the same
// one as the text around the number.

export function numberLocale(): string {
  return (typeof document !== "undefined" && document.documentElement.lang) || "en";
}

/** formatNumber writes a number the way the page's language does: 1,234.5 in
 *  English, 1.234,5 in German. */
export function formatNumber(n: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(numberLocale(), options).format(n);
}

/** oneDecimal writes a number with exactly one digit after the separator. */
export function oneDecimal(n: number): string {
  return formatNumber(n, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
}
