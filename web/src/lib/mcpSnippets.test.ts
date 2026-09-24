// An operator pastes these snippets where BombVault cannot see them, so a
// wrong header name or a missing --allow-http surfaces as a client that fails
// to connect for no visible reason.
import { describe, expect, it } from "vitest";
import {
  CERT_PATH_PLACEHOLDER,
  KEY_PLACEHOLDER,
  claudeCodeSnippet,
  claudeDesktopSnippet,
  genericSnippet,
  mcpUrl,
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

interface DesktopEntry {
  bombvault: { command: string; args: string[]; env: Record<string, string> };
}

function desktopEntry(input: McpSnippetInput): DesktopEntry["bombvault"] {
  return (JSON.parse(`{${claudeDesktopSnippet(input)}}`) as DesktopEntry).bombvault;
}

describe("the client snippets", () => {
  it("builds the Claude Code command", () => {
    expect(claudeCodeSnippet(trusted)).toBe(
      "claude mcp add --transport http bombvault --scope user " +
        `https://backup.example.com/mcp --header "Authorization: Bearer ${KEY}"`
    );
    expect(mcpUrl({ ...trusted, origin: "https://backup.example.com/" })).toBe(
      "https://backup.example.com/mcp"
    );

    expect(claudeCodeSnippet(own)).toBe(
      `claude mcp add bombvault --scope user -e NODE_EXTRA_CA_CERTS=${CERT_PATH_PLACEHOLDER} ` +
        `-e BOMBVAULT_MCP_KEY=${KEY} -- npx -y mcp-remote https://192.168.1.10:3443/mcp ` +
        "--header 'X-API-Key:${BOMBVAULT_MCP_KEY}'"
    );
  });

  it("builds a valid Claude Desktop entry", () => {
    const entry = desktopEntry(trusted);
    expect(entry.command).toBe("npx");
    expect(entry.args).toEqual([
      "-y",
      "mcp-remote",
      "https://backup.example.com/mcp",
      "--header",
      "X-API-Key:${BOMBVAULT_MCP_KEY}",
    ]);
    expect(entry.env).toEqual({ BOMBVAULT_MCP_KEY: KEY });

    expect(desktopEntry(own).env).toEqual({
      BOMBVAULT_MCP_KEY: KEY,
      NODE_EXTRA_CA_CERTS: CERT_PATH_PLACEHOLDER,
    });
    expect(desktopEntry(plain).args).toContain("--allow-http");
    expect(desktopEntry(trusted).args).not.toContain("--allow-http");
  });

  it("builds the generic block", () => {
    const block = genericSnippet(trusted);
    expect(block.split("\n")).toEqual([
      "URL: https://backup.example.com/mcp",
      "Transport: Streamable HTTP",
      `Header: Authorization: Bearer ${KEY}`,
      `(or) X-API-Key: ${KEY}`,
    ]);
    expect(genericSnippet(own)).toContain("Certificate: trust bombvault-cert.pem");
  });

  it("uses the placeholder verbatim", () => {
    const anonymous = { ...own, key: KEY_PLACEHOLDER };
    for (const snippet of [
      claudeCodeSnippet(anonymous),
      claudeDesktopSnippet(anonymous),
      genericSnippet(anonymous),
    ]) {
      expect(snippet).toContain(KEY_PLACEHOLDER);
      expect(snippet).not.toContain("bvmcp_");
    }
    expect(desktopEntry(anonymous).env.BOMBVAULT_MCP_KEY).toBe(KEY_PLACEHOLDER);
  });
});
