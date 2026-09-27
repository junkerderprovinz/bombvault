// The configuration each MCP client needs, as text to copy. Pure functions with
// no i18n: every line is a command, a header, a form field or a key that a
// client parses literally, so translating any of it would break the connection.
//
// A key never goes on a command line, where the process list and the shell
// history would keep it. Where a client can take the key from a file, an
// environment variable or a secret prompt, its snippet names that instead;
// where it cannot, the key sits in the snippet and the dialog says so.

export interface McpSnippetInput {
  /** Scheme, host and port of the address the operator opened BombVault at. */
  origin: string;
  endpointPath: string;
  /** The fresh key, or KEY_PLACEHOLDER outside the dialog that shows it once. */
  key: string;
  /** An HTTPS origin served with BombVault's own certificate, which a client
   *  only trusts once it is pointed at the certificate file. */
  selfSigned: boolean;
}

export const KEY_PLACEHOLDER = "<your key>";
export const CERT_PATH_PLACEHOLDER = "<path of the downloaded bombvault-cert.pem>";
export const KEY_FILE_PLACEHOLDER = "<path of the file with your key>";

/** The environment variable every client that reads one takes the key from. */
export const KEY_VARIABLE = "BOMBVAULT_MCP_KEY";

/** mcpUrl joins the origin and the endpoint path without doubling the slash
 *  between them. */
export function mcpUrl(i: Pick<McpSnippetInput, "origin" | "endpointPath">): string {
  return i.origin.replace(/\/+$/, "") + "/" + i.endpointPath.replace(/^\/+/, "");
}

function plainHttp(i: McpSnippetInput): boolean {
  return i.origin.toLowerCase().startsWith("http:");
}

function json(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

/**
 * mcpRemoteCommand starts mcp-remote with the key read from a file, so neither
 * the command nor the process list shows it. A `${VAR}` in the arguments would
 * not keep it out: Claude Code fills it from its own environment before it
 * starts the server. `@latest` keeps an older global mcp-remote without
 * `--header-file` from being picked, and the path is quoted because the
 * operator fills it in by hand and a space would split it.
 */
function mcpRemoteCommand(i: McpSnippetInput): string {
  const http = plainHttp(i) ? " --allow-http" : "";
  return `npx -y mcp-remote@latest ${mcpUrl(i)} --header-file "${KEY_FILE_PLACEHOLDER}"${http}`;
}

/**
 * claudeCodeSnippet returns the `claude mcp add` command for one terminal run.
 * Claude Code's own HTTP transport is no way round mcp-remote, since it
 * refuses BombVault's self-issued certificate even with NODE_EXTRA_CA_CERTS
 * set.
 */
export function claudeCodeSnippet(i: McpSnippetInput): string {
  const cert = i.selfSigned ? `-e "NODE_EXTRA_CA_CERTS=${CERT_PATH_PLACEHOLDER}" ` : "";
  return `claude mcp add bombvault --scope user ${cert}-- ${mcpRemoteCommand(i)}`;
}

/**
 * claudeDesktopSnippet returns claude_desktop_config.json's server entry. It
 * names a key file as the Claude Code command does, so the JSON holds no key
 * and nothing Claude Desktop might expand can put one on mcp-remote's command
 * line.
 */
export function claudeDesktopSnippet(i: McpSnippetInput): string {
  const args = ["-y", "mcp-remote@latest", mcpUrl(i), "--header-file", KEY_FILE_PLACEHOLDER];
  if (plainHttp(i)) args.push("--allow-http");

  const env = i.selfSigned ? { env: { NODE_EXTRA_CA_CERTS: CERT_PATH_PLACEHOLDER } } : {};
  return json({ mcpServers: { bombvault: { command: "npx", args, ...env } } });
}

/** genericSnippet lists the fields any Streamable HTTP client asks for, with
 *  both header forms; the dialog says in the reader's language that either one
 *  works and what to do about the certificate. */
export function genericSnippet(i: McpSnippetInput): string {
  return [
    `URL: ${mcpUrl(i)}`,
    "Transport: Streamable HTTP",
    `Authorization: Bearer ${i.key}`,
    `X-API-Key: ${i.key}`,
  ].join("\n");
}

/** The header of a client that holds the key itself. */
function bearer(i: McpSnippetInput) {
  return { Authorization: `Bearer ${i.key}` };
}

/** The header of a client that fills in the variable, in its own syntax. */
function bearerFrom(variable: string) {
  return { Authorization: `Bearer ${variable}` };
}

/** A form typed into the client's own window, one field per line, named as the
 *  client names them. */
function form(fields: [string, string][]): string {
  return fields.map(([name, value]) => `${name}: ${value}`).join("\n");
}

/**
 * SNIPPETS holds each client's snippet under its id in the card's client list.
 * The shapes follow each client's own documentation; see docs/mcp.md for the
 * list.
 */
export const SNIPPETS = {
  anythingllm: (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { type: "streamable", url: mcpUrl(i), headers: bearer(i) } } }),
  antigravity: (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { serverUrl: mcpUrl(i), headers: bearerFrom(`\${${KEY_VARIABLE}}`) } } }),
  "claude-code": claudeCodeSnippet,
  "claude-desktop": claudeDesktopSnippet,
  cline: (i: McpSnippetInput) =>
    json({
      mcpServers: { bombvault: { type: "streamableHttp", url: mcpUrl(i), headers: bearer(i), disabled: false } },
    }),
  codex: (i: McpSnippetInput) =>
    ["[mcp_servers.bombvault]", `url = "${mcpUrl(i)}"`, `bearer_token_env_var = "${KEY_VARIABLE}"`].join("\n"),
  continue: (i: McpSnippetInput) =>
    [
      "mcpServers:",
      "  - name: bombvault",
      "    type: streamable-http",
      `    url: ${mcpUrl(i)}`,
      "    requestOptions:",
      "      headers:",
      `        Authorization: "Bearer \${{ secrets.${KEY_VARIABLE} }}"`,
    ].join("\n"),
  "copilot-cli": (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { type: "http", url: mcpUrl(i), headers: bearer(i), tools: ["*"] } } }),
  cursor: (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { url: mcpUrl(i), headers: bearerFrom(`\${env:${KEY_VARIABLE}}`) } } }),
  gemini: (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { httpUrl: mcpUrl(i), headers: bearerFrom(`\${${KEY_VARIABLE}}`) } } }),
  copilot: (i: McpSnippetInput) =>
    json({
      inputs: [{ type: "promptString", id: "bombvault-key", description: "BombVault MCP key", password: true }],
      servers: { bombvault: { type: "http", url: mcpUrl(i), headers: bearerFrom("${input:bombvault-key}") } },
    }),
  goose: (i: McpSnippetInput) =>
    [
      "extensions:",
      "  bombvault:",
      "    type: streamable_http",
      "    name: bombvault",
      "    enabled: true",
      `    uri: "${mcpUrl(i)}"`,
      "    headers:",
      `      Authorization: "Bearer \${${KEY_VARIABLE}}"`,
      "    env_keys:",
      `      - ${KEY_VARIABLE}`,
      "    timeout: 300",
    ].join("\n"),
  jan: (i: McpSnippetInput) =>
    form([
      ["Transport", "Streamable HTTP"],
      ["URL", mcpUrl(i)],
      ["Header", "Authorization"],
      ["Value", `Bearer ${i.key}`],
    ]),
  jetbrains: (i: McpSnippetInput) => json({ mcpServers: { bombvault: { url: mcpUrl(i), headers: bearer(i) } } }),
  kimi: (i: McpSnippetInput) => json({ mcpServers: { bombvault: { url: mcpUrl(i), headers: bearer(i) } } }),
  lmstudio: (i: McpSnippetInput) => json({ mcpServers: { bombvault: { url: mcpUrl(i), headers: bearer(i) } } }),
  vibe: (i: McpSnippetInput) =>
    [
      "[[mcp_servers]]",
      'name = "bombvault"',
      'transport = "streamable-http"',
      `url = "${mcpUrl(i)}"`,
      `api_key_env = "${KEY_VARIABLE}"`,
      'api_key_header = "Authorization"',
      'api_key_format = "Bearer {token}"',
    ].join("\n"),
  msty: (i: McpSnippetInput) =>
    form([
      ["Name", "bombvault"],
      ["Connection Type", "HTTP"],
      ["Server URL", mcpUrl(i)],
      ["Authentication", "Bearer"],
      ["Token", i.key],
    ]),
  n8n: (i: McpSnippetInput) =>
    form([
      ["Endpoint URL", mcpUrl(i)],
      ["Server Transport", "HTTP Streamable"],
      ["Authentication", "Bearer Auth"],
      ["Credential, Token", i.key],
    ]),
  openwebui: (i: McpSnippetInput) =>
    form([
      ["Type", "MCP (Streamable HTTP)"],
      ["URL", mcpUrl(i)],
      ["Auth", "Bearer"],
      ["Key", i.key],
    ]),
  opencode: (i: McpSnippetInput) =>
    json({
      $schema: "https://opencode.ai/config.json",
      mcp: {
        bombvault: {
          type: "remote",
          url: mcpUrl(i),
          enabled: true,
          headers: bearerFrom(`{env:${KEY_VARIABLE}}`),
        },
      },
    }),
  // The Mac app reads no shell profile, so the certificate goes into the
  // command itself.
  perplexity: (i: McpSnippetInput) => {
    const cert = i.selfSigned ? `env "NODE_EXTRA_CA_CERTS=${CERT_PATH_PLACEHOLDER}" ` : "";
    return form([["Server Name", "bombvault"], ["Command", cert + mcpRemoteCommand(i)]]);
  },
  qwen: (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { httpUrl: mcpUrl(i), headers: bearerFrom(`\${${KEY_VARIABLE}}`) } } }),
  roo: (i: McpSnippetInput) =>
    json({
      mcpServers: {
        bombvault: { type: "streamable-http", url: mcpUrl(i), headers: bearerFrom(`\${env:${KEY_VARIABLE}}`) },
      },
    }),
  visualstudio: (i: McpSnippetInput) =>
    json({ servers: { bombvault: { url: mcpUrl(i), requestInit: { headers: bearer(i) } } } }),
  warp: (i: McpSnippetInput) => json({ mcpServers: { bombvault: { url: mcpUrl(i), headers: bearer(i) } } }),
  windsurf: (i: McpSnippetInput) =>
    json({ mcpServers: { bombvault: { serverUrl: mcpUrl(i), headers: bearerFrom(`\${env:${KEY_VARIABLE}}`) } } }),
  zed: (i: McpSnippetInput) => json({ context_servers: { bombvault: { url: mcpUrl(i), headers: bearer(i) } } }),
  grok: (i: McpSnippetInput) =>
    form([
      ["MCP server URL", mcpUrl(i)],
      ["Header", "Authorization"],
      ["Value", `Bearer ${i.key}`],
    ]),
  lechat: (i: McpSnippetInput) =>
    form([
      ["Server URL", mcpUrl(i)],
      ["Authentication", "HTTP Bearer Token"],
      ["Token", i.key],
    ]),
  other: genericSnippet,
} satisfies Record<string, (i: McpSnippetInput) => string>;

export type McpSnippetClient = keyof typeof SNIPPETS;
