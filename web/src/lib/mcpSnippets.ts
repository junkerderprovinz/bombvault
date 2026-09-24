// The configuration an MCP client needs, as text to copy. Pure functions with
// no i18n: every line is a command, a header or a JSON key that a client parses
// literally, so translating any of it would break the connection.

export interface McpSnippetInput {
  /** Scheme, host and port of the address the operator opened BombVault at. */
  origin: string;
  endpointPath: string;
  /** The fresh key, or KEY_PLACEHOLDER outside the panel that shows it once. */
  key: string;
  /** An HTTPS origin served with BombVault's own certificate, which a client
   *  only trusts once it is pointed at the certificate file. */
  selfSigned: boolean;
}

export const KEY_PLACEHOLDER = "<your key>";
export const CERT_PATH_PLACEHOLDER = "<path of the downloaded bombvault-cert.pem>";

/** mcpUrl joins the origin and the endpoint path without doubling the slash
 *  between them. */
export function mcpUrl(i: McpSnippetInput): string {
  return i.origin.replace(/\/+$/, "") + "/" + i.endpointPath.replace(/^\/+/, "");
}

/**
 * claudeCodeSnippet returns the `claude mcp add` command for one terminal run.
 * Behind BombVault's own certificate it reaches the server through mcp-remote,
 * because the client's HTTP transport would want NODE_EXTRA_CA_CERTS in the
 * environment the client itself started in, while `-e` sets it for this server
 * alone. The header reference keeps its single quotes so no shell expands it,
 * and both `-e` pairs are quoted because the operator fills the certificate
 * path in by hand and a path with a space would otherwise split in two.
 */
export function claudeCodeSnippet(i: McpSnippetInput): string {
  if (!i.selfSigned) {
    return (
      "claude mcp add --transport http bombvault --scope user " +
      `${mcpUrl(i)} --header "Authorization: Bearer ${i.key}"`
    );
  }
  return (
    `claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=${CERT_PATH_PLACEHOLDER}" ` +
    `-e "BOMBVAULT_MCP_KEY=${i.key}" -- npx -y mcp-remote ${mcpUrl(i)} ` +
    "--header 'X-API-Key:${BOMBVAULT_MCP_KEY}'"
  );
}

/**
 * claudeDesktopSnippet returns the one entry that goes inside "mcpServers" in
 * claude_desktop_config.json. The key travels in `env` and the header argument
 * only references it, because mcp-remote splits a `--header` value on the first
 * space and would drop a key written after one.
 */
export function claudeDesktopSnippet(i: McpSnippetInput): string {
  const args = ["-y", "mcp-remote", mcpUrl(i), "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"];
  if (i.origin.toLowerCase().startsWith("http:")) args.push("--allow-http");

  const env: Record<string, string> = { BOMBVAULT_MCP_KEY: i.key };
  if (i.selfSigned) env.NODE_EXTRA_CA_CERTS = CERT_PATH_PLACEHOLDER;

  // Serialized inside a wrapper object and then stripped of it, so what comes
  // out is what JSON.parse takes back once it sits between braces again.
  return JSON.stringify({ bombvault: { command: "npx", args, env } }, null, 2)
    .split("\n")
    .slice(1, -1)
    .map((line) => line.slice(2))
    .join("\n");
}

/** genericSnippet describes the connection for any client that speaks
 *  Streamable HTTP and takes a header. */
export function genericSnippet(i: McpSnippetInput): string {
  const lines = [
    `URL: ${mcpUrl(i)}`,
    "Transport: Streamable HTTP",
    `Header: Authorization: Bearer ${i.key}`,
    `(or) X-API-Key: ${i.key}`,
  ];
  if (i.selfSigned) lines.push("Certificate: trust bombvault-cert.pem (download it in this card)");
  return lines.join("\n");
}
