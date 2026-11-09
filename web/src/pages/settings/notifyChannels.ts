// What the notification card needs to know about channels, apart from drawing
// them: the kinds and their words, a new channel's settings, which secret a
// moved address waits for, and the sentence for each refusal.
import { REDACTED, type NotifyChannel, type NotifyConfig, type NotifyKind, type NotifyProblem } from "../../lib/api";
import type { TranslationKey } from "../../lib/i18n";

/** The kinds in the order the add list offers them. */
export const KINDS: NotifyKind[] = ["unraid", "ntfy", "gotify", "discord", "slack", "matrix", "smtp", "apprise", "webhook", "healthchecks"];

export const KIND_LABEL: Record<NotifyKind, TranslationKey> = {
  unraid: "notify.unraid",
  ntfy: "notify.kindNtfy",
  gotify: "notify.gotify",
  discord: "notify.kindDiscord",
  slack: "notify.kindSlack",
  matrix: "notify.matrix",
  smtp: "notify.smtp",
  apprise: "notify.apprise",
  webhook: "notify.webhookChannel",
  healthchecks: "notify.healthchecksTitle",
};

export const KIND_HINT: Partial<Record<NotifyKind, TranslationKey>> = {
  unraid: "notify.unraidHint",
  ntfy: "notify.ntfyHint",
  gotify: "notify.gotifyHint",
  apprise: "notify.appriseHint",
  healthchecks: "notify.healthchecksHint",
};

/** The JSON a new webhook posts, the document BombVault has always sent. */
export const WEBHOOK_BODY = '{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}';

export const PLACEHOLDERS = ["event", "severity", "ok", "title", "body", "link", "key", "time", "source", "importance"];

/** newChannel is a channel of kind that starts switched off, so it saves
 *  before its fields are filled in. */
export function newChannel(kind: NotifyKind, taken: NotifyChannel[]): NotifyChannel {
  const c: NotifyChannel = { id: freeID(kind, taken), kind, enabled: false, on: kind === "healthchecks" ? "always" : "failure" };
  switch (kind) {
    case "webhook":
      c.webhook = { url: "", method: "POST", headers: { "Content-Type": "application/json" }, body: WEBHOOK_BODY };
      break;
    case "discord":
      c.discord = { url: "" };
      break;
    case "slack":
      c.slack = { url: "" };
      break;
    case "gotify":
      c.gotify = { url: "", token: "", priorityFailure: 8, prioritySuccess: 5 };
      break;
    case "ntfy":
      c.ntfy = { url: "", token: "" };
      break;
    case "matrix":
      c.matrix = { homeserver: "", room: "", token: "" };
      break;
    case "smtp":
      c.smtp = { host: "", port: 587, username: "", password: "", from: "", to: "", tls: "starttls" };
      break;
    case "apprise":
      c.apprise = { url: "", tags: "" };
      break;
    case "healthchecks":
      c.healthchecks = { url: "", byKey: {} };
      break;
  }
  return c;
}

function freeID(kind: NotifyKind, taken: NotifyChannel[]): string {
  const ids = new Set(taken.map((c) => c.id));
  if (!ids.has(kind)) return kind;
  let n = 2;
  while (ids.has(`${kind}-${n}`)) n++;
  return `${kind}-${n}`;
}

/** A secret and the fields that name where it goes. The server gives a stored
 *  secret back only to the place it was saved for. */
interface SecretPlace {
  secret: (c: NotifyChannel) => string | undefined;
  places: (c: NotifyChannel) => unknown[];
  /** c with the places of prev, so a moved address waits for its secret. */
  hold: (c: NotifyChannel, prev: NotifyChannel) => NotifyChannel;
}

const SECRET_PLACES: Partial<Record<NotifyKind, SecretPlace>> = {
  gotify: {
    secret: (c) => c.gotify?.token,
    places: (c) => [c.gotify?.url],
    hold: (c, prev) => ({ ...c, gotify: { ...c.gotify!, url: prev.gotify!.url } }),
  },
  ntfy: {
    secret: (c) => c.ntfy?.token,
    places: (c) => [c.ntfy?.url],
    hold: (c, prev) => ({ ...c, ntfy: { ...c.ntfy!, url: prev.ntfy!.url } }),
  },
  matrix: {
    secret: (c) => c.matrix?.token,
    places: (c) => [c.matrix?.homeserver],
    hold: (c, prev) => ({ ...c, matrix: { ...c.matrix!, homeserver: prev.matrix!.homeserver } }),
  },
  smtp: {
    secret: (c) => c.smtp?.password,
    places: (c) => [c.smtp?.host, c.smtp?.port, c.smtp?.username],
    hold: (c, prev) => ({
      ...c,
      smtp: { ...c.smtp!, host: prev.smtp!.host, port: prev.smtp!.port, username: prev.smtp!.username },
    }),
  },
  webhook: {
    // Any header kept from before ties the address down.
    secret: (c) => (Object.values(c.webhook?.headers ?? {}).includes(REDACTED) ? REDACTED : ""),
    places: (c) => [c.webhook?.url.trim()],
    hold: (c, prev) => ({ ...c, webhook: { ...c.webhook!, url: prev.webhook!.url } }),
  },
};

/** saved is the channel as the server last took it, matched by id and kind. */
export function saved(c: NotifyChannel, good: NotifyConfig): NotifyChannel | undefined {
  return good.channels.find((p) => p.id === c.id && p.kind === c.kind);
}

/** hasSecret reports whether the server holds the channel's secret. */
export function hasSecret(c: NotifyChannel, good: NotifyConfig): boolean {
  const prev = saved(c, good);
  const s = SECRET_PLACES[c.kind];
  return !!prev && !!s && !!s.secret(prev);
}

/** waitsForSecret reports whether c points somewhere new while its secret is
 *  still the stored one, which the server would drop. */
export function waitsForSecret(c: NotifyChannel, good: NotifyConfig): boolean {
  const prev = saved(c, good);
  const s = SECRET_PLACES[c.kind];
  if (!prev || !s || s.secret(c) !== REDACTED || !s.secret(prev)) return false;
  return JSON.stringify(s.places(c)) !== JSON.stringify(s.places(prev));
}

/** outgoing is what the card sends: a moved address that waits for its
 *  secret goes out as it was, and rows left blank stay out. */
export function outgoing(cfg: NotifyConfig, good: NotifyConfig): NotifyConfig {
  return {
    ...cfg,
    channels: cfg.channels.map((c) => {
      let out = c;
      if (waitsForSecret(c, good)) out = SECRET_PLACES[c.kind]!.hold(c, saved(c, good)!);
      if (out.healthchecks?.byKey) {
        const byKey = Object.fromEntries(Object.entries(out.healthchecks.byKey).filter(([, u]) => u.trim() !== ""));
        out = { ...out, healthchecks: { ...out.healthchecks, byKey } };
      }
      if (out.apprise?.urls) {
        const urls = out.apprise.urls.map((u) => u.trim()).filter((u) => u !== "");
        out = { ...out, apprise: { ...out.apprise, urls: urls.length > 0 ? urls : undefined } };
      }
      return out;
    }),
  };
}

/** A refusal tied to one channel. Field is the path inside it, empty for the
 *  channel as a whole. */
export interface ChannelProblem {
  channel: string;
  field: string;
  code: string;
}

/** placeProblems puts each problem on the channel it was about. */
export function placeProblems(sent: NotifyConfig, problems: NotifyProblem[]): ChannelProblem[] {
  return problems.map((p) => ({ channel: sent.channels[p.index]?.id ?? p.channel ?? "", field: p.field ?? "", code: p.code }));
}

const PROBLEM_KEY: Record<string, TranslationKey> = {
  missing_field: "notify.problem.missingField",
  bad_url: "notify.problem.badUrl",
  bad_template: "notify.problem.badTemplate",
  bad_header: "notify.problem.badHeader",
  bad_priority: "notify.problem.badPriority",
  bad_port: "notify.problem.badPort",
};

export function problemKey(code: string): TranslationKey {
  return PROBLEM_KEY[code] ?? "notify.problem.other";
}

const RESULT_KEY: Record<string, TranslationKey> = {
  dns: "notify.result.reach",
  refused: "notify.result.reach",
  network: "notify.result.reach",
  tls: "notify.result.tls",
  timeout: "notify.result.timeout",
  auth: "notify.result.auth",
  not_found: "notify.result.notFound",
  redirect: "notify.result.redirect",
  rejected: "notify.result.rejected",
  rate_limited: "notify.result.rateLimited",
  server: "notify.result.server",
  script: "notify.result.script",
  no_starttls: "notify.result.noStarttls",
  invalid: "notify.result.invalid",
  canceled: "notify.result.canceled",
  "unraid-platform": "notify.result.unraidPlatform",
  "unraid-ssh": "notify.result.unraidSsh",
};

/** resultKey is the sentence for a failed test, undefined for a code the card
 *  does not know. */
export function resultKey(code: string | undefined): TranslationKey | undefined {
  return code ? RESULT_KEY[code] : undefined;
}
