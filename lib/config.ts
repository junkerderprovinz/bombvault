import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { z } from "zod";

// Single source of truth for runtime configuration, validated with zod.
// APP_KEY is the AES-256-GCM master key for secrets at rest: 32 bytes, supplied
// as 64 lowercase hex chars. DATA_DIR holds bulk data (restic local repos, certs);
// CONFIG_DIR holds the small SQLite DB and defaults to DATA_DIR (single-volume
// installs keep working). APPDATA_DIR is the host appdata mount probed by the spike.
const schema = z.object({
  APP_KEY: z
    .string()
    .regex(/^[0-9a-f]{64}$/, "APP_KEY must be 64 lowercase hex characters (32 bytes)"),
  DATA_DIR: z.string().min(1).default("./data"),
  CONFIG_DIR: z.string().min(1).optional(),
  PORT: z.coerce.number().int().positive().default(3000),
  HTTPS_PORT: z.coerce.number().int().positive().default(3443),
  APPDATA_DIR: z.string().min(1).default("/mnt/user/appdata"),
});

export interface AppConfig {
  APP_KEY: string;
  DATA_DIR: string;
  CONFIG_DIR: string;
  PORT: number;
  HTTPS_PORT: number;
  APPDATA_DIR: string;
  DB_PATH: string;
  ensureDataDirs(): void;
}

/** Parse + validate an environment object into a typed, frozen config. Throws on invalid input. */
export function loadConfig(env: Record<string, string | undefined> = process.env): Readonly<AppConfig> {
  const parsed = schema.parse(env);
  const CONFIG_DIR = parsed.CONFIG_DIR ?? parsed.DATA_DIR;
  const DB_PATH = join(CONFIG_DIR, "bombvault.sqlite");

  let dirsReady = false;
  return Object.freeze({
    APP_KEY: parsed.APP_KEY,
    DATA_DIR: parsed.DATA_DIR,
    CONFIG_DIR,
    PORT: parsed.PORT,
    HTTPS_PORT: parsed.HTTPS_PORT,
    APPDATA_DIR: parsed.APPDATA_DIR,
    DB_PATH,
    ensureDataDirs() {
      if (dirsReady) return;
      for (const dir of [parsed.DATA_DIR, CONFIG_DIR]) {
        mkdirSync(dir, { recursive: true });
      }
      dirsReady = true;
    },
  });
}

// Lazily-built singleton for app runtime. Tests call loadConfig() directly with
// an explicit env, so they never touch this.
let cached: Readonly<AppConfig> | null = null;
export function getConfig(): Readonly<AppConfig> {
  return (cached ??= loadConfig());
}
