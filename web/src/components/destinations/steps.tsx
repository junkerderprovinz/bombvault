// What each step of setting up a destination shows, without its heading and
// its buttons: the wizard on a card stacks the steps, the window to add a
// storage location shows one at a time.
import type { BackendOption, Provider } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { CopyBlock } from "../CopyBlock";
import { InfoBubble } from "../InfoBubble";
import { IconFolder } from "../navGlyphs";
import { SelectField } from "../SelectField";
import { VerdictLine } from "../TestButton";
import { Toggle } from "../Toggle";
import type { DestinationDraftState } from "./useDestinationDraft";

export const FIELD = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";
const DOCS_RECOVERY = "https://junkerderprovinz.github.io/bombvault/offsite-recovery/";

const FIELD_LABELS: Record<string, TranslationKey> = {
  user: "dest.field.user",
  username: "dest.field.user",
  pass: "dest.field.pass",
  password: "dest.field.pass",
  url: "dest.field.url",
  host: "dest.field.host",
  port: "dest.field.port",
  endpoint: "dest.field.endpoint",
  region: "dest.field.region",
  access_key_id: "dest.field.access_key_id",
  secret_access_key: "dest.field.secret_access_key",
  token: "dest.field.token",
  account: "dest.field.account",
  key: "dest.field.key",
  api_key: "dest.field.api_key",
  email: "dest.field.email",
  "2fa": "dest.field.2fa",
  domain: "dest.field.domain",
  apple_id: "dest.field.apple_id",
  library: "dest.field.library",
  share_name: "dest.field.share_name",
  path: "dest.field.path",
};

/** The sentences that name the thing being set up, which the card calls a
 *  destination and the Storage locations page a storage location. */
export interface DestinationWords {
  folderTip: TranslationKey;
  lockNone: TranslationKey;
  recovery: TranslationKey;
}

export const DESTINATION_WORDS: DestinationWords = {
  folderTip: "dest.folder.tip",
  lockNone: "dest.lock.none",
  recovery: "dest.recovery",
};

// rclone marks user names and account ids sensitive too, which is right for
// its logs but would hide what someone is typing. A field is masked when it
// holds a password or a secret.
function masked(o: BackendOption): boolean {
  return o.password || (/(secret|token|key|pass)/.test(o.name) && !o.name.endsWith("_id"));
}

function fitNote(t: ReturnType<typeof useT>["t"], p: Provider, label: string): { text: string; tone: "ok" | "warn" } {
  if (p.group === "cloud" && p.selfHosted) return { text: t("dest.note.selfHosted"), tone: "ok" };
  if (p.group === "cloud") {
    const cap = p.id === "gdrive" ? ` ${t("dest.note.dailyCap")}` : "";
    return { text: t("dest.note.slower").replace("{name}", () => label) + cap, tone: "warn" };
  }
  if (p.route === "rest") return { text: t("dest.note.rest"), tone: "ok" };
  return { text: t("dest.note.good") + (p.lock ? ` ${t("dest.note.lock")}` : ""), tone: "ok" };
}

function DraftField({ draft, option }: { draft: DestinationDraftState; option: BackendOption }) {
  const { t } = useT();
  const { picked, settings, setSetting } = draft;
  const o = option;
  const label = FIELD_LABELS[o.name] ? t(FIELD_LABELS[o.name]) : o.name;
  const tip = o.name === "pass" && picked?.auth === "apppassword" ? t("dest.appPassword.tip") : o.help;
  // For an S3 service only the examples rclone ties to it: the untied ones
  // describe signature quirks of unnamed servers.
  const s3Provider = picked?.preset?.provider;
  const examples =
    o.secret || !o.examples
      ? []
      : o.examples
          .filter((e) => (s3Provider ? !!e.provider && e.provider.split(",").includes(s3Provider) : !e.provider))
          .slice(0, 12);
  const wide = o.name === "url" || o.name === "token" || o.name === "endpoint" || o.name === "path";
  return (
    <label className={`flex flex-col gap-1 ${wide ? "md:col-span-2" : ""}`}>
      <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
        {label}
        {tip && <InfoBubble tip={tip} />}
      </span>
      <input
        type={masked(o) ? "password" : "text"}
        value={settings[o.name] ?? ""}
        onChange={(e) => setSetting(o.name, e.target.value)}
        placeholder={o.name === "url" ? picked?.urlHint ?? o.default : o.default}
        autoComplete={masked(o) ? "new-password" : "off"}
        spellCheck={false}
        dir="ltr"
        className={`${FIELD} font-mono text-start`}
      />
      {examples.length > 0 && (
        <span className="flex flex-wrap gap-1.5">
          {examples.map((e) => (
            <Badge
              key={e.value}
              as="button"
              tone={settings[o.name] === e.value ? "active" : "neutral"}
              size="small"
              onClick={() => setSetting(o.name, e.value)}
            >
              {e.help || e.value}
            </Badge>
          ))}
        </span>
      )}
    </label>
  );
}

/** How well the provider suits backups, what it needs to sign in, and the
 *  form with the verdict of the last test. */
export function SignInBody({ draft, providerLabel }: { draft: DestinationDraftState; providerLabel: string }) {
  const { t } = useT();
  const { picked, fields } = draft;
  if (!picked) return null;
  const note = fitNote(t, picked, providerLabel);
  return (
    <>
      <div className={`rounded-card px-3 py-2 text-xs leading-relaxed ${note.tone === "warn" ? "bg-statusWarnBg text-statusWarn" : "bg-statusOkBg text-statusOk"}`}>
        {note.text}
      </div>
      {picked.auth === "token" && (
        <div className="flex flex-col gap-1">
          <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("dest.token.run")}
            <InfoBubble tip={t("dest.token.tip")} />
          </span>
          <CopyBlock text={`rclone authorize "${picked.backend}"`} />
        </div>
      )}
      {picked.auth === "sshkey" && draft.publicKey && (
        <div className="flex flex-col gap-1">
          <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("dest.sshKey")}
            <InfoBubble tip={t("dest.sshKey.tip")} />
          </span>
          <CopyBlock text={draft.publicKey} />
        </div>
      )}
      {picked.route === "s3" && <p className="text-xs text-carbon-textMuted">{t("dest.key.note")}</p>}
      <div className="grid gap-3 md:grid-cols-2">
        <label className="flex flex-col gap-1">
          <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("dest.name")}
            <InfoBubble tip={t("dest.nameHint")} />
          </span>
          <input value={draft.name} onChange={(e) => draft.setName(e.target.value)} autoComplete="off" className={FIELD} />
        </label>
        {fields.main.map((o) => (
          <DraftField key={o.name} draft={draft} option={o} />
        ))}
        {draft.showMore && fields.more.map((o) => <DraftField key={o.name} draft={draft} option={o} />)}
      </div>
      {fields.more.length > 0 && <Toggle label={t("dest.more")} checked={draft.showMore} onChange={draft.setShowMore} />}
      <VerdictLine verdict={draft.test.verdict} />
    </>
  );
}

/** The folder the repositories go into: a walk through the provider's folders
 *  where it has any, and the address that results. */
export function FolderBody({ draft }: { draft: DestinationDraftState }) {
  const { t } = useT();
  const { picked, dir, folders, free, newFolder } = draft;
  if (!picked) return null;
  return (
    <>
      {draft.browsable && (
        <div className="flex flex-col gap-1 rounded-card bg-carbon-surface p-2">
          <div className="flex flex-wrap items-center gap-1 px-2 py-1 text-xs text-carbon-textMuted" dir="ltr">
            <button type="button" className="text-carbon-textSub hover:text-carbon-text" onClick={() => void draft.loadFolders([])}>
              {picked.id}:
            </button>
            {dir.map((d, i) => (
              <span key={i} className="flex items-center gap-1">
                <span>/</span>
                <button type="button" className="text-carbon-textSub hover:text-carbon-text" onClick={() => void draft.loadFolders(dir.slice(0, i + 1))}>
                  {d}
                </button>
              </span>
            ))}
          </div>
          {free !== null && (
            <p className="px-2 text-xs text-carbon-textMuted">{t("dest.free").replace("{size}", () => humanBytes(free))}</p>
          )}
          {picked.route === "s3" && dir.length === 0 && <p className="px-2 text-xs text-carbon-textMuted">{t("dest.folder.buckets")}</p>}
          {folders && folders.length === 0 && <p className="px-2 py-1 text-xs text-carbon-textMuted">{t("dest.folder.empty")}</p>}
          {folders?.map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => void draft.loadFolders([...dir, f])}
              className="flex items-center gap-2 rounded-control px-2 py-1.5 text-start text-sm text-carbon-text hover:bg-carbon-surface2"
            >
              <span className="text-carbon-textMuted"><IconFolder /></span>
              {f}
            </button>
          ))}
          {draft.folderError && <p className="px-2 text-xs text-statusFail">{draft.folderError}</p>}
          {newFolder !== null ? (
            <span className="flex gap-2 p-1">
              <input
                autoFocus
                value={newFolder}
                onChange={(e) => draft.setNewFolder(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void draft.makeFolder()}
                placeholder={t("dest.folder.newName")}
                className={`${FIELD} flex-1`}
              />
              <Button label={t("dest.folder.create")} labelKey={null} tone="accent" onClick={() => void draft.makeFolder()} />
            </span>
          ) : (
            <Button label={t("dest.folder.new")} labelKey="dest.folder.new" tone="neutral" onClick={() => draft.setNewFolder("")} className="self-start" />
          )}
        </div>
      )}
      {draft.needsBucket && <p className="text-xs text-statusWarn">{t("dest.folder.pickBucket")}</p>}
      <div className="flex flex-col gap-1">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("dest.location")}
          <InfoBubble tip={t(`dest.route.${picked.route}` as TranslationKey)} />
        </span>
        <p dir="ltr" className="rounded-control bg-carbon-surface3 px-3 py-1.5 text-start font-mono text-sm text-carbon-text break-all">
          {draft.location}
        </p>
      </div>
      {picked.route === "s3" && (
        <SelectField
          label={t("cloud.storageClass.default")}
          value={draft.storageClass}
          onChange={draft.setStorageClass}
          options={[{ value: "", label: t("cloud.storageClass.default") }, ...STORAGE_CLASSES.map((c) => ({ value: c, label: c }))]}
          className={FIELD}
        />
      )}
    </>
  );
}

/** The append-only switch with what it can and cannot promise at this provider. */
export function ProtectBody({
  draft,
  providerLabel,
  words = DESTINATION_WORDS,
}: {
  draft: DestinationDraftState;
  providerLabel: string;
  words?: DestinationWords;
}) {
  const { t } = useT();
  const { picked } = draft;
  if (!picked) return null;
  const named = (key: TranslationKey) => t(key).replace("{name}", () => providerLabel);
  return (
    <>
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-0.5">
          <span className="text-sm text-carbon-text">{t("offsite.immutable")}</span>
          <span className="text-xs text-carbon-textMuted">{t("dest.immutableHint")}</span>
        </div>
        <Toggle hideLabel label={t("offsite.immutable")} checked={draft.immutable} onChange={draft.setImmutable} disabled={!picked.lock} className="mt-0.5" />
      </div>
      <div className={`rounded-card px-3 py-2 text-xs leading-relaxed ${picked.lock ? "bg-carbon-surface text-carbon-textSub" : "bg-statusWarnBg text-statusWarn"}`}>
        {picked.route === "rest" ? t("dest.lock.rest") : picked.lock ? named("dest.lock.s3") : named(words.lockNone)}
      </div>
    </>
  );
}

/** How the destination comes back after a loss of the server. */
export function RecoveryBody({
  draft,
  providerLabel,
  words = DESTINATION_WORDS,
}: {
  draft: DestinationDraftState;
  providerLabel: string;
  words?: DestinationWords;
}) {
  const { t } = useT();
  const { picked } = draft;
  if (!picked) return null;
  return (
    <p className="rounded-card bg-carbon-surface px-3 py-2 text-xs leading-relaxed text-carbon-textSub">
      {t(words.recovery)}
      {picked.auth === "token" && ` ${t("dest.recovery.token").replace("{name}", () => providerLabel)}`}
      {picked.auth === "sshkey" && ` ${t("dest.recovery.ssh")}`}{" "}
      <a href={DOCS_RECOVERY} target="_blank" rel="noreferrer noopener" className="underline">
        {t("dest.recovery.link")}
      </a>
    </p>
  );
}
