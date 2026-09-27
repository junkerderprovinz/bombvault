// The MCP clients the card offers a button for, and what each one's setup
// dialog says. The facts come from each client's own documentation: where its
// configuration lives, how it can get the key without holding it, and what it
// does with a certificate it does not know. Where the documentation leaves the
// certificate open, the dialog words that step as "if it refuses", never as a
// promise. docs/mcp.md lists the same clients.
import type { TranslationKey } from "./i18n";
import { SNIPPETS, type McpSnippetClient, type McpSnippetInput } from "./mcpSnippets";

/** The second line of a client's button. */
export type ClientKind = "terminal" | "editor" | "desktop" | "vscode" | "localModels" | "automation" | "webChat";

export const KIND_LABEL: Record<ClientKind, TranslationKey> = {
  terminal: "mcp.kindTerminal",
  editor: "mcp.kindEditor",
  desktop: "mcp.kindDesktop",
  vscode: "mcp.kindVsCode",
  localModels: "mcp.kindLocalModels",
  automation: "mcp.kindAutomation",
  webChat: "mcp.kindWebChat",
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
  kind?: ClientKind;
  group: ClientGroup;
  /** The key into CLIENT_MARKS; the Other client button draws a glyph. */
  mark?: string;
  /** The `.glim-tile-*` class its button lights up in. */
  tile: string;
  /** Brightens the resting mark on the dark theme, where its colour would read
   *  as a hole in the button. */
  lift?: boolean;
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
    kind: "localModels",
    group: "local",
    mark: "anythingllm",
    tile: "glim-tile-anythingllm",
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
    kind: "editor",
    group: "local",
    mark: "antigravity",
    tile: "glim-tile-antigravity",
    setup: { kind: "file", paths: home(".gemini/config/mcp_config.json"), ui: "Manage MCP Servers, View raw config" },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "claude-code",
    name: "Claude Code",
    kind: "terminal",
    group: "local",
    mark: "claudecode",
    tile: "glim-tile-claude",
    setup: { kind: "command" },
    key: { kind: "keyFile" },
    cert: { kind: "placeholder" },
    node: true,
  },
  {
    id: "claude-desktop",
    name: "Claude Desktop",
    kind: "desktop",
    group: "local",
    mark: "claude",
    tile: "glim-tile-claude",
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
    kind: "vscode",
    group: "local",
    mark: "cline",
    tile: "glim-tile-cline",
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
    kind: "terminal",
    group: "local",
    mark: "openai",
    tile: "glim-tile-openai",
    setup: { kind: "file", paths: home(".codex/config.toml") },
    key: { kind: "env" },
    cert: { kind: "env", variable: "CODEX_CA_CERTIFICATE" },
  },
  {
    id: "continue",
    name: "Continue",
    kind: "vscode",
    group: "local",
    mark: "continue",
    tile: "glim-tile-continue",
    setup: { kind: "file", paths: home(".continue/config.yaml") },
    key: { kind: "dotenv", file: "~/.continue/.env" },
    cert: NODE_CERT,
  },
  {
    id: "copilot-cli",
    name: "Copilot CLI",
    kind: "terminal",
    group: "local",
    mark: "copilot",
    tile: "glim-tile-copilot",
    setup: { kind: "file", paths: home(".copilot/mcp-config.json") },
    key: { kind: "inFile" },
    cert: NODE_CERT,
  },
  {
    id: "cursor",
    name: "Cursor",
    kind: "editor",
    group: "local",
    mark: "cursor",
    tile: "glim-tile-cursor",
    setup: { kind: "file", paths: home(".cursor/mcp.json"), ui: "Settings, MCP, Add new global MCP server" },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "gemini",
    name: "Gemini CLI",
    kind: "terminal",
    group: "local",
    mark: "gemini",
    tile: "glim-tile-gemini",
    setup: { kind: "file", paths: home(".gemini/settings.json") },
    key: { kind: "env", unsure: true },
    cert: NODE_CERT,
  },
  {
    id: "copilot",
    name: "GitHub Copilot",
    kind: "vscode",
    group: "local",
    mark: "vscode",
    tile: "glim-tile-vscode",
    setup: { kind: "file", paths: vsCodeStorage("mcp.json"), ui: "MCP: Open User Configuration" },
    key: { kind: "prompt" },
    cert: SYSTEM_CERT,
  },
  {
    id: "goose",
    name: "Goose",
    kind: "desktop",
    group: "local",
    mark: "goose",
    tile: "glim-tile-goose",
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
    kind: "localModels",
    group: "local",
    mark: "jan",
    tile: "glim-tile-jan",
    setup: { kind: "form", ui: "Settings, MCP Servers, Add MCP Server" },
    key: { kind: "inApp" },
    cert: SYSTEM_CERT,
  },
  {
    id: "jetbrains",
    name: "JetBrains",
    kind: "editor",
    group: "local",
    mark: "jetbrains",
    tile: "glim-tile-jetbrains",
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
    kind: "terminal",
    group: "local",
    mark: "kimi",
    tile: "glim-tile-kimi",
    setup: { kind: "file", paths: home(".kimi/mcp.json") },
    key: { kind: "inFile" },
    cert: PYTHON_CERT,
  },
  {
    id: "lmstudio",
    name: "LM Studio",
    kind: "localModels",
    group: "local",
    mark: "lmstudio",
    tile: "glim-tile-lmstudio",
    setup: { kind: "file", paths: home(".lmstudio/mcp.json"), ui: "Program, Install, Edit mcp.json" },
    key: { kind: "inFile" },
    cert: { kind: "ownList" },
  },
  {
    id: "vibe",
    name: "Mistral Vibe",
    kind: "terminal",
    group: "local",
    mark: "mistral",
    tile: "glim-tile-mistral",
    setup: { kind: "file", paths: home(".vibe/config.toml") },
    key: { kind: "env" },
    cert: PYTHON_CERT,
  },
  {
    id: "msty",
    name: "Msty",
    kind: "localModels",
    group: "local",
    mark: "msty",
    tile: "glim-tile-msty",
    lift: true,
    setup: { kind: "form", ui: "Toolbox, Tools, Add New Tool" },
    key: { kind: "inApp" },
    cert: SYSTEM_CERT,
  },
  {
    id: "n8n",
    name: "n8n",
    kind: "automation",
    group: "local",
    mark: "n8n",
    tile: "glim-tile-n8n",
    setup: { kind: "form", ui: "MCP Client Tool" },
    key: { kind: "credential" },
    cert: NODE_CERT,
  },
  {
    id: "openwebui",
    name: "Open WebUI",
    kind: "localModels",
    group: "local",
    mark: "openwebui",
    tile: "glim-tile-openwebui",
    setup: { kind: "form", ui: "Admin Panel, Settings, External Tools, Add Connection" },
    key: { kind: "inApp" },
    cert: PYTHON_CERT,
  },
  {
    id: "opencode",
    name: "opencode",
    kind: "terminal",
    group: "local",
    mark: "opencode",
    tile: "glim-tile-opencode",
    setup: { kind: "file", paths: home(".config/opencode/opencode.json") },
    key: { kind: "env" },
    cert: NODE_CERT,
  },
  {
    id: "perplexity",
    name: "Perplexity",
    kind: "desktop",
    group: "local",
    mark: "perplexity",
    tile: "glim-tile-perplexity",
    setup: { kind: "form", ui: "Settings, Connectors, Add Connector, MCP Connector" },
    key: { kind: "keyFile" },
    cert: { kind: "placeholder" },
    note: "mcp.notePerplexity",
    node: true,
  },
  {
    id: "qwen",
    name: "Qwen Code",
    kind: "terminal",
    group: "local",
    mark: "qwen",
    tile: "glim-tile-qwen",
    setup: { kind: "file", paths: home(".qwen/settings.json") },
    key: { kind: "env", unsure: true },
    cert: NODE_CERT,
  },
  {
    id: "roo",
    name: "Roo Code",
    kind: "vscode",
    group: "local",
    mark: "roocode",
    tile: "glim-tile-roocode",
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
    kind: "editor",
    group: "local",
    mark: "visualstudio",
    tile: "glim-tile-visualstudio",
    lift: true,
    setup: { kind: "file", paths: { windows: "%USERPROFILE%\\.mcp.json" } },
    key: { kind: "inFile" },
    cert: SYSTEM_CERT,
  },
  {
    id: "warp",
    name: "Warp",
    kind: "terminal",
    group: "local",
    mark: "warp",
    tile: "glim-tile-warp",
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
    kind: "editor",
    group: "local",
    mark: "windsurf",
    tile: "glim-tile-windsurf",
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
    kind: "editor",
    group: "local",
    mark: "zed",
    tile: "glim-tile-zed",
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
    kind: "webChat",
    group: "cloud",
    mark: "chatgpt",
    tile: "glim-tile-chatgpt",
    setup: { kind: "other" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
    oauth: "mcp.setupChatGPT",
  },
  {
    id: "claudeai",
    name: "Claude",
    kind: "webChat",
    group: "cloud",
    mark: "claude",
    tile: "glim-tile-claude",
    setup: { kind: "other" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
    oauth: "mcp.setupClaudeAi",
  },
  {
    id: "grok",
    name: "Grok",
    kind: "webChat",
    group: "cloud",
    mark: "grok",
    tile: "glim-tile-grok",
    setup: { kind: "form", ui: "grok.com, Connectors, New Connector, Custom" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
    note: "mcp.noteGrok",
  },
  {
    id: "lechat",
    name: "Le Chat",
    kind: "webChat",
    group: "cloud",
    mark: "mistral",
    tile: "glim-tile-mistral",
    setup: { kind: "form", ui: "Connectors, Add a Custom Connector" },
    key: { kind: "vendor" },
    cert: { kind: "none" },
  },
];

export const OTHER_CLIENT: McpClient = {
  id: "other",
  name: "",
  group: "local",
  tile: "glim-tile-house",
  setup: { kind: "other" },
  key: { kind: "other" },
  cert: { kind: "other" },
};

const BY_ID = new Map<string, McpClient>(
  [...LOCAL_CLIENTS, ...CLOUD_CLIENTS, OTHER_CLIENT].map((c) => [c.id, c])
);

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
