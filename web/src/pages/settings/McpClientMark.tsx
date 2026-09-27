import { useId } from "react";
import { IconKey, IconLink } from "../../components/glyphs";
import { CLIENT_MARKS } from "../../lib/mcpClientMarks";
import type { McpClient } from "../../lib/mcpClients";

/** withOwnIds gives a mark's gradients and masks ids of their own, because the
 *  same mark sits on the button, on key tiles and in the dialog at once, and a
 *  second copy of an id would point every copy at the first one's gradient. */
function withOwnIds(svg: string, suffix: string): string {
  return svg
    .replace(/id="([^"]+)"/g, `id="$1-${suffix}"`)
    .replace(/url\(#([^)]+)\)/g, `url(#$1-${suffix})`)
    .replace(/href="#([^"]+)"/g, `href="#$1-${suffix}"`);
}

/**
 * ClientMark draws a client's mark: its resting version and, where the brand
 * has one, the single-colour version a lit brand tile swaps in. Other client
 * draws the link glyph, and a key made for no client the key glyph.
 */
export function ClientMark({ client, forKey = false }: { client: McpClient | undefined; forKey?: boolean }) {
  const suffix = useId().replace(/[^a-zA-Z0-9]/g, "");
  const mark = client?.mark ? CLIENT_MARKS[client.mark] : undefined;
  if (!mark) return forKey || !client ? <IconKey /> : <IconLink />;
  const rest = withOwnIds(mark.rest, suffix);
  const html = mark.hover
    ? rest.replace("<svg ", '<svg class="glim-mark-rest" ') + mark.hover.replace("<svg ", '<svg class="glim-mark-hover" ')
    : rest;
  return <span className="contents" dangerouslySetInnerHTML={{ __html: html }} />;
}
