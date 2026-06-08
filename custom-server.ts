import { createServer as createHttpServer } from "node:http";
import { createServer as createHttpsServer } from "node:https";
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, chmodSync } from "node:fs";
import { join } from "node:path";
import { parse } from "node:url";
import next from "next";
import { getConfig } from "./lib/config";
import { initDb } from "./server/db";

// Single Node server: init the SQLite DB, then serve Next. HTTPS by default with
// a self-signed cert generated on first start (the house WebUI-HTTPS rule); set
// HTTP_ONLY=true to fall back to plain HTTP behind a reverse proxy.
const cfg = getConfig();
const dev = process.env.NODE_ENV !== "production";
const hostname = process.env.HOSTNAME ?? "0.0.0.0";
const httpOnly = (process.env.HTTP_ONLY ?? "false").toLowerCase() === "true";

function ensureSelfSigned(): { key: Buffer; cert: Buffer } {
  const certDir = join(cfg.CONFIG_DIR, "certs");
  mkdirSync(certDir, { recursive: true });
  const keyPath = join(certDir, "key.pem");
  const certPath = join(certDir, "cert.pem");
  if (!existsSync(keyPath) || !existsSync(certPath)) {
    execFileSync("openssl", [
      "req", "-x509", "-newkey", "rsa:2048", "-nodes",
      "-keyout", keyPath, "-out", certPath, "-days", "3650",
      "-subj", "/CN=bombvault",
    ]);
    // SEC-003: the private key must not be world/group-readable. chmod is a
    // no-op on Windows; skip there to avoid odd ACL behaviour (dev only).
    if (process.platform !== "win32") chmodSync(keyPath, 0o600);
  }
  return { key: readFileSync(keyPath), cert: readFileSync(certPath) };
}

function banner(scheme: string, port: number): void {
  const proto = scheme.toUpperCase();
  // eslint-disable-next-line no-console
  console.log(
    [
      "",
      "  ############################################################",
      `   BOMBVAULT IS READY  ->  open the WebUI now (${proto} ${port})`,
      "  ############################################################",
      "",
    ].join("\n"),
  );
}

async function main(): Promise<void> {
  cfg.ensureDataDirs();
  initDb();

  if (cfg.DISABLE_AUTH) {
    // eslint-disable-next-line no-console
    console.warn(
      "⚠  AUTH DISABLED — anyone who can reach the WebUI has root-equivalent control of this host. Use only on a trusted, non-exposed network.",
    );
  }

  const port = httpOnly ? cfg.PORT : cfg.HTTPS_PORT;
  const app = next({ dev, hostname, port });
  const handle = app.getRequestHandler();
  await app.prepare();

  const listener = (req: import("node:http").IncomingMessage, res: import("node:http").ServerResponse) =>
    handle(req, res, parse(req.url ?? "/", true));

  if (httpOnly) {
    createHttpServer(listener).listen(port, hostname, () => banner("http", port));
  } else {
    createHttpsServer(ensureSelfSigned(), listener).listen(port, hostname, () =>
      banner("https", port),
    );
  }
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error("Fatal: failed to start BombVault", err);
  process.exit(1);
});
