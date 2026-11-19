// The MCP clients the card offers a button for, and what each one's setup
// dialog says. The facts come from each client's own documentation: where its
// configuration lives, how it can get the key without holding it, and what it
// does with a certificate it does not know. Where the documentation leaves the
// certificate open, the dialog words that step as "if it refuses", never as a
// promise. docs/mcp.md lists the same clients.
import type { CatalogEntry } from "@glimstone/core";
import { MCP_PROVIDERS, type McpGroup, type McpProviderId } from "@glimstone/core/mcpProviders";
import type { TranslationKey } from "./i18n";
import { SNIPPETS, type McpSnippetClient, type McpSnippetInput } from "./mcpSnippets";

/** The title of each of the catalog's groups, which is also a client's kind. */
export const GROUP_LABEL: Record<McpGroup, TranslationKey> = {
  chat: "mcp.kindDesktop",
  models: "mcp.kindLocalModels",
  web: "mcp.kindWebChat",
  editor: "mcp.kindEditor",
  extension: "mcp.kindVsCode",
  terminal: "mcp.kindTerminal",
  automation: "mcp.kindAutomation",
};

/** Where a configuration file lives. `unix` stands for macOS and Linux alike. */
export interface ConfigPaths {
  windows?: string;
  macos?: string;
  linux?: string;
  unix?: string;
}

/** How the configuration gets into the client. */
export type SetupWay =
  | { kind: "command" }
  /** `ui` opens the file in the client; `paste` takes the same text pasted in. */
  | { kind: "file"; paths: ConfigPaths; ui?: string; paste?: string }
  | { kind: "form"; ui: string }
  | { kind: "other" };

/** How the client comes by the key. */
export type KeyWay =
  /** mcp-remote reads it from a file the configuration names. */
  | { kind: "keyFile" }
  /** An environment variable. `unsure` where the documentation shows the
   *  variable in the file's other fields but not in a header. */
  | { kind: "env"; unsure?: boolean }
  /** A masked prompt, kept in the client's secret storage. */
  | { kind: "prompt" }
  /** A line in a dotenv file of the client's own. */
  | { kind: "dotenv"; file: string }
  /** The key has to sit in the configuration file. */
  | { kind: "inFile" }
  /** The key is typed into the client's own settings. */
  | { kind: "inApp" }
  /** An encrypted credential store. */
  | { kind: "credential" }
  /** The vendor's servers keep it. */
  | { kind: "vendor" }
  | { kind: "other" };

/** What the client needs to trust BombVault's own certificate. */
export type CertWay =
  /** The configuration names the file already. */
  | { kind: "placeholder" }
  /** An environment variable names the file. */
  | { kind: "env"; variable: string }
  /** The system's trusted certificates. */
  | { kind: "system" }
  /** A certificate list of the client's own that takes no additions. */
  | { kind: "ownList" }
  /** A cloud service, which needs a publicly trusted certificate anyway. */
  | { kind: "none" }
  | { kind: "other" };

export type ClientGroup = "local" | "cloud";

export interface McpClient {
  /** Stored with a key made for this client, so it must never change. */
  id: string;
  name: string;
  group: ClientGroup;
  setup: SetupWay;
  key: KeyWay;
  cert: CertWay;
  /** A cloud service that takes no key and signs in through OAuth instead. The
   *  key is its setup text; the dialog replaces the key step with sign-in. */
  oauth?: TranslationKey;
  /** A sentence the dialog puts first. */
  note?: TranslationKey;
  /** The client runs mcp-remote through npx. */
  node?: boolean;
}

const NODE_CERT: CertWay = { kind: "env", variable: "NODE_EXTRA_CA_CERTS" };
const PYTHON_CERT: CertWay = { kind: "env", variable: "SSL_CERT_FILE" };
const SYSTEM_CERT: CertWay = { kind: "system" };

/** VS Code's user data folder, where its extensions keep their settings. */
function vsCodeStorage(rest: string): ConfigPaths {
  return {
    windows: `%APPDATA%\\Code\\User\\${rest.replaceAll("/", "\\")}`,
    macos: `~/Library/Application Support/Code/User/${rest}`,
    linux: `~/.config/Code/User/${rest}`,
  };
}

/** A file in the user's home folder. */
function home(rest: string): ConfigPaths {
  return { windows: `%USERPROFILE%\\${rest.replaceAll("/", "\\")}`, unix: `~/${rest}` };
}

export const LOCAL_CLIENTS: McpClient[] = [
  {
    id: "anythingllm",
    name: "AnythingLLM",
    group: "local",
    setup: {
      kind: "file",
      paths: {
        windows: "%APPDATA%\\anythingllm-desktop\\storage\\plugins\\anythingllm_mcp_servers.json",
        macos: "~/Library/Application Support/anythingllm-desktop/storage/plugins/anythingllm_mcp_servers.json",
        linux: "~/.config/anythingllm-desktop/storage/plugins/anythingllm_mcp_servers.json",
      },
      ui: "Agent Skills, MCP Servers",
    },
    key: { kind: "inFile" },
    cert: NODE_CERT,
  },
  {
    id: "antigravity",
    name: "Antigravity",
    group: "local",
    setup: { kind: "file", paths: home(".gemini/config/mcp_config.json"), ui: "Manage MCP Servers, View raw config" },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "claude-code",
    name: "Claude Code",
    group: "local",
    setup: { kind: "command" },
    key: { kind: "keyFile" },
    cert: { kind: "placeholder" },
    node: true,
  },
  {
    id: "claude-desktop",
    name: "Claude Desktop",
    group: "local",
    setup: {
      kind: "file",
      paths: {
        windows: "%APPDATA%\\Claude\\claude_desktop_config.json",
        macos: "~/Library/Application Support/Claude/claude_desktop_config.json",
      },
      ui: "Settings, Developer, Edit Config",
    },
    key: { kind: "keyFile" },
    cert: { kind: "placeholder" },
    node: true,
  },
  {
    id: "cline",
    name: "Cline",
    group: "local",
    setup: {
      kind: "file",
      paths: vsCodeStorage("globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json"),
      ui: "MCP Servers, Configure MCP Servers",
    },
    key: { kind: "inFile" },
    cert: NODE_CERT,
  },
  {
    id: "codex",
    name: "Codex CLI",
    group: "local",
    setup: { kind: "file", paths: home(".codex/config.toml") },
    key: { kind: "env" },
    cert: { kind: "env", variable: "CODEX_CA_CERTIFICATE" },
  },
  {
    id: "continue",
    name: "Continue",
    group: "local",
    setup: { kind: "file", paths: home(".continue/config.yaml") },
    key: { kind: "dotenv", file: "~/.continue/.env" },
    cert: NODE_CERT,
  },
  {
    id: "copilot-cli",
    name: "Copilot CLI",
    group: "local",
    setup: { kind: "file", paths: home(".copilot/mcp-config.json") },
    key: { kind: "inFile" },
    cert: NODE_CERT,
  },
  {
    id: "cursor",
    name: "Cursor",
    group: "local",
    setup: { kind: "file", paths: home(".cursor/mcp.json"), ui: "Settings, MCP, Add new global MCP server" },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "gemini",
    name: "Gemini CLI",
    group: "local",
    setup: { kind: "file", paths: home(".gemini/settings.json") },
    key: { kind: "env", unsure: true },
    cert: NODE_CERT,
  },
  {
    id: "copilot",
    name: "GitHub Copilot",
    group: "local",
    setup: { kind: "file", paths: vsCodeStorage("mcp.json"), ui: "MCP: Open User Configuration" },
    key: { kind: "prompt" },
    cert: SYSTEM_CERT,
  },
  {
    id: "goose",
    name: "Goose",
    group: "local",
    setup: {
      kind: "file",
      paths: { windows: "%APPDATA%\\Block\\goose\\config\\config.yaml", unix: "~/.config/goose/config.yaml" },
    },
    key: { kind: "env", unsure: true },
    cert: SYSTEM_CERT,
  },
  {
    id: "jan",
    name: "Jan",
    group: "local",
    setup: { kind: "form", ui: "Settings, MCP Servers, Add MCP Server" },
    key: { kind: "inApp" },
    cert: SYSTEM_CERT,
  },
  {
    id: "jetbrains",
    name: "JetBrains",
    group: "local",
    setup: {
      kind: "file",
      paths: home(".junie/mcp/mcp.json"),
      ui: "Settings, Tools, Junie, MCP Settings",
      paste: "Settings, Tools, AI Assistant, Model Context Protocol (MCP), Add",
    },
    key: { kind: "inFile" },
    cert: SYSTEM_CERT,
  },
  {
    id: "kimi",
    name: "Kimi Code",
    group: "local",
    setup: { kind: "file", paths: home(".kimi/mcp.json") },
    key: { kind: "inFile" },
    cert: PYTHON_CERT,
  },
  {
    id: "lmstudio",
    name: "LM Studio",
    group: "local",
    setup: { kind: "file", paths: home(".lmstudio/mcp.json"), ui: "Program, Install, Edit mcp.json" },
    key: { kind: "inFile" },
    cert: { kind: "ownList" },
  },
  {
    id: "vibe",
    name: "Mistral Vibe",
    group: "local",
    setup: { kind: "file", paths: home(".vibe/config.toml") },
    key: { kind: "env" },
    cert: PYTHON_CERT,
  },
  {
    id: "msty",
    name: "Msty",
    group: "local",
    setup: { kind: "form", ui: "Toolbox, Tools, Add New Tool" },
    key: { kind: "inApp" },
    cert: SYSTEM_CERT,
  },
  {
    id: "n8n",
    name: "n8n",
    group: "local",
    setup: { kind: "form", ui: "MCP Client Tool" },
    key: { kind: "credential" },
    cert: NODE_CERT,
  },
  {
    id: "openwebui",
    name: "Open WebUI",
    group: "local",
    setup: { kind: "form", ui: "Admin Panel, Settings, External Tools, Add Connection" },
    key: { kind: "inApp" },
    cert: PYTHON_CERT,
  },
  {
    id: "opencode",
    name: "opencode",
    group: "local",
    setup: { kind: "file", paths: home(".config/opencode/opencode.json") },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "perplexity",
    name: "Perplexity",
    group: "local",
    setup: { kind: "form", ui: "Settings, Connectors, Add Connector, MCP Connector" },
    key: { kind: "keyFile" },
    cert: { kind: "placeholder" },
    note: "mcp.notePerplexity",
    node: true,
  },
  {
    id: "qwen",
    name: "Qwen Code",
    group: "local",
    setup: { kind: "file", paths: home(".qwen/settings.json") },
    key: { kind: "env", unsure: true },
    cert: NODE_CERT,
  },
  {
    id: "roo",
    name: "Roo Code",
    group: "local",
    setup: {
      kind: "file",
      paths: vsCodeStorage("globalStorage/rooveterinaryinc.roo-cline/settings/mcp_settings.json"),
      ui: "MCP, Edit Global MCP",
    },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "visualstudio",
    name: "Visual Studio",
    group: "local",
    setup: { kind: "file", paths: { windows: "%USERPROFILE%\\.mcp.json" } },
    key: { kind: "inFile" },
    cert: SYSTEM_CERT,
  },
  {
    id: "warp",
    name: "Warp",
    group: "local",
    setup: {
      kind: "file",
      paths: { unix: "~/.warp/.mcp.json" },
      paste: "Settings, Agents, Warp Agent, Manage MCP servers, + Add",
    },
    key: { kind: "inFile" },
    cert: SYSTEM_CERT,
  },
  {
    id: "windsurf",
    name: "Windsurf",
    group: "local",
    setup: {
      kind: "file",
      paths: { windows: "%APPDATA%\\devin\\mcp_config.json", unix: "~/.config/devin/mcp_config.json" },
    },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "zed",
    name: "Zed",
    group: "local",
    setup: {
      kind: "file",
      paths: { windows: "%APPDATA%\\Zed\\settings.json", unix: "~/.config/zed/settings.json" },
    },
    key: { kind: "inFile" },
    cert: SYSTEM_CERT,
  },
];

export const CLOUD_CLIENTS: McpClient[] = [
  {
    id: "chatgpt",
    name: "ChatGPT",
    group: "cloud",
    setup: { kind: "other" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
    oauth: "mcp.setupChatGPT",
  },
  {
    id: "claudeai",
    name: "Claude",
    group: "cloud",
    setup: { kind: "other" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
    oauth: "mcp.setupClaudeAi",
  },
  {
    id: "grok",
    name: "Grok",
    group: "cloud",
    setup: { kind: "form", ui: "grok.com, Connectors, New Connector, Custom" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
    note: "mcp.noteGrok",
  },
  {
    id: "lechat",
    name: "Le Chat",
    group: "cloud",
    setup: { kind: "form", ui: "Connectors, Add a Custom Connector" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
  },
];

export const OTHER_CLIENT: McpClient = {
  id: "other",
  name: "",
  group: "local",
  setup: { kind: "other" },
  key: { kind: "other" },
  cert: { kind: "other" },
};

const BY_ID = new Map<string, McpClient>(
  [...LOCAL_CLIENTS, ...CLOUD_CLIENTS, OTHER_CLIENT].map((c) => [c.id, c])
);

// The catalog calls the web chat at claude.ai claude. Keys and sign-ins made
// for it carry claudeai, so that stays the id BombVault stores.
const CATALOG_ID: Record<string, McpProviderId> = { claudeai: "claude" };

/** The catalog's id for a client of BombVault's. */
export function catalogId(id: string): McpProviderId {
  return CATALOG_ID[id] ?? (id as McpProviderId);
}

/** The id BombVault stores for a client picked from the catalog. */
export function storedId(id: string): string {
  return Object.keys(CATALOG_ID).find((own) => CATALOG_ID[own] === id) ?? id;
}

const ENTRIES = new Map(MCP_PROVIDERS.entries.map((e) => [e.id as string, e]));

/** A client's entry in the catalog: its mark or glyph, and the group that says
 *  what kind of app it is. */
export function clientEntry(client: McpClient): CatalogEntry<McpGroup, McpProviderId> | undefined {
  return ENTRIES.get(catalogId(client.id));
}

/** The client a key was created for, or undefined for a key made before the
 *  list existed, through Other client, or for a client since dropped. */
export function clientById(id: string | undefined): McpClient | undefined {
  if (!id || id === OTHER_CLIENT.id) return undefined;
  return BY_ID.get(id);
}

/** The configuration to copy for a client, or undefined for one that only
 *  gets a note. */
export function snippetFor(client: McpClient, input: McpSnippetInput): string | undefined {
  return client.id in SNIPPETS ? SNIPPETS[client.id as McpSnippetClient](input) : undefined;
}
