import { useId, type ReactNode } from "react";
import { IconKey, IconLink } from "../../components/glyphs";
import { IconFileProgram, IconTabApps } from "../../components/navGlyphs";
import { OTHER_CLIENT, clientEntry, type McpClient } from "../../lib/mcpClients";

/** withOwnIds gives a mark's gradients and masks ids of their own, because the
 *  same mark sits on the button, on key tiles and in the dialog at once, and a
 *  second copy of an id would point every copy at the first one's gradient. */
function withOwnIds(svg: string, suffix: string): string {
  return svg
    .replace(/id="([^"]+)"/g, `id="$1-${suffix}"`)
    .replace(/url\(#([^)]+)\)/g, `url(#$1-${suffix})`)
    .replace(/href="#([^"]+)"/g, `href="#$1-${suffix}"`);
}

/** Our glyph for one the MCP catalog names, for a client without a mark. */
export function catalogGlyph(glyph: string | undefined): ReactNode {
  if (glyph === "IconCode") return <IconFileProgram />;
  if (glyph === "IconLink") return <IconLink />;
  return <IconTabApps />;
}

/**
 * ClientMark draws a client's mark from the catalog: its resting version and,
 * where the brand has one, the single-colour version a lit brand tile swaps
 * in. Other client draws the link glyph, and a key made for no client the key
 * glyph.
 */
export function ClientMark({ client, forKey = false }: { client: McpClient | undefined; forKey?: boolean }) {
  const suffix = useId().replace(/[^a-zA-Z0-9]/g, "");
  if (!client) return <IconKey />;
  if (client === OTHER_CLIENT) return forKey ? <IconKey /> : <IconLink />;
  const entry = clientEntry(client);
  if (!entry?.mark) return catalogGlyph(entry?.glyph);
  const rest = withOwnIds(entry.mark.svg, suffix);
  const hover = entry.mark.hover && withOwnIds(entry.mark.hover, suffix);
  const html = hover ? rest.replace("<svg ", '<svg class="glim-mark-rest" ') + hover.replace("<svg ", '<svg class="glim-mark-hover" ') : rest;
  return <span className="contents" dangerouslySetInnerHTML={{ __html: html }} />;
}
