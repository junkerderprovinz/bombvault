// An operator pastes these snippets where BombVault cannot see them, so a
// wrong header name or a missing --allow-http surfaces as a client that fails
// to connect for no visible reason.
import { describe, expect, it } from "vitest";
import {
  CERT_PATH_PLACEHOLDER,
  KEY_FILE_PLACEHOLDER,
  KEY_PLACEHOLDER,
  KEY_VARIABLE,
  SNIPPETS,
  claudeCodeSnippet,
  claudeDesktopSnippet,
  genericSnippet,
  mcpUrl,
  type McpSnippetClient,
} from "./mcpSnippets";
import type { McpSnippetInput } from "./mcpSnippets";

const KEY = "bvmcp_7Qa-Zb0_cD";

const trusted: McpSnippetInput = {
  origin: "https://backup.example.com",
  endpointPath: "/mcp",
  key: KEY,
  selfSigned: false,
};

const own: McpSnippetInput = { ...trusted, origin: "https://192.168.1.10:3443", selfSigned: true };

const plain: McpSnippetInput = { ...trusted, origin: "http://tower:3443" };

const CLIENTS = Object.keys(SNIPPETS) as McpSnippetClient[];

interface DesktopConfig {
  mcpServers: { bombvault: { command: string; args: string[]; env?: Record<string, string> } };
}

function desktopEntry(input: McpSnippetInput): DesktopConfig["mcpServers"]["bombvault"] {
  return (JSON.parse(claudeDesktopSnippet(input)) as DesktopConfig).mcpServers.bombvault;
}

// Splits a command the way a shell does over the pieces these snippets use:
// whitespace separates arguments unless it sits inside quotes.
function shellWords(command: string): string[] {
  const words = command.match(/(?:[^\s"']+|"[^"]*"|'[^']*')+/g) ?? [];
  return words.map((w) => w.replace(/"([^"]*)"/g, "$1"));
}

/** Every command and argument in a JSON configuration, at any depth. */
function jsonCommandLine(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap(jsonCommandLine);
  if (value === null || typeof value !== "object") return [];
  const out: string[] = [];
  for (const [name, inner] of Object.entries(value)) {
    if (name === "command" && typeof inner === "string") out.push(...shellWords(inner));
    else if (name === "args" && Array.isArray(inner)) out.push(...inner.map(String));
    else out.push(...jsonCommandLine(inner));
  }
  return out;
}

/** What a client puts on a process's command line from its snippet: the whole
 *  of a terminal command, a form's Command field, or a JSON entry's command
 *  and args. A TOML, YAML or form snippet without a command starts nothing. */
function commandLine(client: McpSnippetClient, snippet: string): string[] {
  if (client === "claude-code") return shellWords(snippet);
  const field = /^Command: (.*)$/m.exec(snippet);
  if (field) return shellWords(field[1]);
  try {
    return jsonCommandLine(JSON.parse(snippet));
  } catch {
    return [];
  }
}

/** The clients whose snippet names the key's variable or prompt instead of
 *  holding the key. */
const READS_THE_KEY_ELSEWHERE: McpSnippetClient[] = [
  "antigravity",
  "claude-code",
  "claude-desktop",
  "codex",
  "continue",
  "copilot",
  "cursor",
  "gemini",
  "goose",
  "opencode",
  "perplexity",
  "qwen",
  "roo",
  "vibe",
  "windsurf",
];

describe("every client's snippet", () => {
  it.each(CLIENTS)("%s keeps the key off the command line", (client) => {
    for (const input of [trusted, own, plain]) {
      const words = commandLine(client, SNIPPETS[client](input));
      expect(words.join(" ")).not.toContain(KEY);
    }
  });

  it("finds the command line of the clients that start a process", () => {
    for (const client of ["claude-code", "claude-desktop", "perplexity"] as const) {
      expect(commandLine(client, SNIPPETS[client](trusted)), client).toContain(mcpUrl(trusted));
    }
  });

  it.each(READS_THE_KEY_ELSEWHERE)("%s leaves the key out of the file", (client) => {
    for (const input of [trusted, own, plain]) {
      expect(SNIPPETS[client](input)).not.toContain(KEY);
    }
  });

  it.each(CLIENTS)("%s points at this address", (client) => {
    expect(SNIPPETS[client](own)).toContain("https://192.168.1.10:3443/mcp");
  });

  it.each(CLIENTS.filter((c) => !READS_THE_KEY_ELSEWHERE.includes(c)))(
    "%s carries the placeholder until a key is made",
    (client) => {
      const snippet = SNIPPETS[client]({ ...own, key: KEY_PLACEHOLDER });
      expect(snippet).toContain(KEY_PLACEHOLDER);
      expect(snippet).not.toContain("bvmcp_");
    }
  );

  it("names one variable wherever a client reads the key from the environment", () => {
    for (const client of ["antigravity", "codex", "continue", "cursor", "gemini", "goose", "opencode", "qwen", "roo", "vibe", "windsurf"] as const) {
      expect(SNIPPETS[client](trusted), client).toContain(KEY_VARIABLE);
    }
  });

  it("parses as JSON wherever the snippet is a JSON file", () => {
    for (const client of CLIENTS) {
      const snippet = SNIPPETS[client](trusted);
      if (!snippet.startsWith("{")) continue;
      expect(() => JSON.parse(snippet), client).not.toThrow();
    }
  });
});

describe("the Claude snippets", () => {
  it("builds the Claude Code command", () => {
    expect(claudeCodeSnippet(trusted)).toBe(
      "claude mcp add bombvault --scope user -- npx -y mcp-remote@latest " +
        `https://backup.example.com/mcp --header-file "${KEY_FILE_PLACEHOLDER}"`
    );
    expect(mcpUrl({ ...trusted, origin: "https://backup.example.com/" })).toBe(
      "https://backup.example.com/mcp"
    );

    expect(claudeCodeSnippet(own)).toBe(
      `claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=${CERT_PATH_PLACEHOLDER}" ` +
        "-- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp " +
        `--header-file "${KEY_FILE_PLACEHOLDER}"`
    );
    expect(shellWords(claudeCodeSnippet(plain))).toContain("--allow-http");
    expect(shellWords(claudeCodeSnippet(trusted))).not.toContain("--allow-http");
  });

  // Claude Code fills a ${VAR} in a server's arguments from its own
  // environment before it starts the server, which would put the key on the
  // process's command line.
  it("leaves no variable in the Claude Code command for Claude Code to fill in", () => {
    for (const input of [trusted, own, plain]) {
      expect(claudeCodeSnippet(input)).not.toContain("${");
    }
  });

  it("keeps a certificate or key file path with a space in one argument", () => {
    const cert = "/Users/sam/My Downloads/bombvault-cert.pem";
    const keyFile = "C:\\Users\\Sam Doe\\bombvault-key.txt";
    const words = shellWords(
      claudeCodeSnippet(own).replace(CERT_PATH_PLACEHOLDER, cert).replace(KEY_FILE_PLACEHOLDER, keyFile)
    );
    expect(words).toContain(`NODE_EXTRA_CA_CERTS=${cert}`);
    expect(words[words.indexOf("--header-file") + 1]).toBe(keyFile);
  });

  it("builds a valid Claude Desktop configuration", () => {
    const entry = desktopEntry(trusted);
    expect(entry.command).toBe("npx");
    expect(entry.args).toEqual([
      "-y",
      "mcp-remote@latest",
      "https://backup.example.com/mcp",
      "--header-file",
      KEY_FILE_PLACEHOLDER,
    ]);
    expect(entry.env).toBeUndefined();

    expect(desktopEntry(own).env).toEqual({ NODE_EXTRA_CA_CERTS: CERT_PATH_PLACEHOLDER });
    expect(desktopEntry(plain).args).toContain("--allow-http");
    expect(desktopEntry(trusted).args).not.toContain("--allow-http");
  });

  // Whether Claude Desktop fills in a ${VAR} in the arguments is not
  // documented, and if it does the key lands on mcp-remote's command line.
  it("leaves no variable in the Claude Desktop configuration for Claude Desktop to fill in", () => {
    for (const input of [trusted, own, plain]) {
      expect(claudeDesktopSnippet(input)).not.toContain("${");
    }
  });

  it("gives Perplexity the same key file as Claude Code", () => {
    const command = /^Command: (.*)$/m.exec(SNIPPETS.perplexity(plain))?.[1] ?? "";
    const words = shellWords(command);
    expect(words[words.indexOf("--header-file") + 1]).toBe(KEY_FILE_PLACEHOLDER);
    expect(words).toContain("--allow-http");
  });

  // The Mac app does not read the shell profile, so a variable set there would
  // never reach mcp-remote.
  it("sets the certificate in Perplexity's own command", () => {
    const command = (input: McpSnippetInput) => /^Command: (.*)$/m.exec(SNIPPETS.perplexity(input))?.[1] ?? "";
    const words = shellWords(command(own).replace(CERT_PATH_PLACEHOLDER, "/Users/sam/My Downloads/bombvault-cert.pem"));
    expect(words.slice(0, 3)).toEqual(["env", "NODE_EXTRA_CA_CERTS=/Users/sam/My Downloads/bombvault-cert.pem", "npx"]);
    expect(command(trusted)).toMatch(/^npx /);
  });
});

describe("Warp", () => {
  it("puts the server under mcpServers, which its + Add field and its file both expect", () => {
    const config = JSON.parse(SNIPPETS.warp(trusted)) as { mcpServers: { bombvault: { url: string } } };
    expect(config.mcpServers.bombvault.url).toBe("https://backup.example.com/mcp");
  });
});

describe("the generic block", () => {
  it("lists literal fields only", () => {
    const block = genericSnippet(trusted);
    expect(block.split("\n")).toEqual([
      "URL: https://backup.example.com/mcp",
      "Transport: Streamable HTTP",
      `Authorization: Bearer ${KEY}`,
      `X-API-Key: ${KEY}`,
    ]);
    expect(genericSnippet(own).split("\n").slice(1)).toEqual(block.split("\n").slice(1));
  });
});

describe("VS Code", () => {
  it("asks for the key through a masked prompt", () => {
    const config = JSON.parse(SNIPPETS.copilot(trusted)) as {
      inputs: { id: string; password: boolean }[];
      servers: { bombvault: { headers: { Authorization: string } } };
    };
    expect(config.inputs).toEqual([expect.objectContaining({ id: "bombvault-key", password: true })]);
    expect(config.servers.bombvault.headers.Authorization).toBe("Bearer ${input:bombvault-key}");
  });
});
