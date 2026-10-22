// What each step of setting up a destination shows, without its heading and
// its buttons: the wizard on a card stacks the steps, the window to add a
// storage location shows one at a time.
import { useId, type ReactNode } from "react";
import type { BackendOption, Provider } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { CopyBlock } from "../CopyBlock";
import { InfoBubble } from "../InfoBubble";
import { IconFolder } from "../navGlyphs";
import { SelectField } from "../SelectField";
import { Selector } from "../Selector";
import { Rows, SettingRow } from "../storage/rows";
import { VerdictLine } from "../TestButton";
import { Toggle } from "../Toggle";
import { defaultSignIn, signInOption, signInsFor, useCloudCredSets } from "../../lib/useCloudCredSets";
import { endpointBucket, type DestinationDraftState } from "./useDestinationDraft";

export const FIELD = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";

// The fields a saved sign-in fills in on the server.
const SAVED_FIELDS = ["access_key_id", "secret_access_key", "user", "pass"];
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

// Where a provider's field means something rclone's help does not say, as
// provider id and field name.
const OWN_TIPS: Record<string, TranslationKey> = {
  "garage:region": "dest.region.garage",
  "idrivee2:endpoint": "dest.endpoint.idrive",
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
export function masked(o: BackendOption): boolean {
  return o.password || (/(secret|token|key|pass)/.test(o.name) && !o.name.endsWith("_id"));
}

// Where a service hands out the personal login token rclone trades for a
// sign-in.
const LOGIN_TOKEN_PAGES: Record<string, string> = {
  jottacloud: "https://www.jottacloud.com/web/secure",
};

const QUESTION_LABELS: Record<string, TranslationKey> = {
  config_driveid: "dest.question.config_driveid",
  config_workspace: "dest.question.config_workspace",
  config_team_drive_id: "dest.question.config_team_drive_id",
};

// rclone's config blob: JSON in unpadded base64.
const configBlob = (settings: Record<string, string>) => btoa(JSON.stringify(settings)).replace(/=+$/, "");

/** The command that signs in on a computer with a browser, or null while the
 *  form lacks what it needs. ShareFile and Zoho are given a config blob: with
 *  one, rclone prints the ShareFile address along with the token, and Zoho
 *  learns its region before the sign-in. */
export function authorizeCommand(p: Provider, settings: Record<string, string>): string | null {
  if (p.backend === "zoho") {
    const region = settings.region?.trim();
    return region ? `rclone authorize "zoho" "${configBlob({ region })}"` : null;
  }
  if (p.backend === "sharefile") return `rclone authorize "sharefile" "${configBlob({})}"`;
  return `rclone authorize "${p.backend}"`;
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

/** One field of a provider's form. `kept` stands in an empty field whose
 *  stored value the server does not show. */
export function DraftField({
  draft,
  option,
  kept,
}: {
  draft: Pick<DestinationDraftState, "picked" | "settings" | "setSetting">;
  option: BackendOption;
  kept?: string;
}) {
  const { t } = useT();
  const id = useId();
  const { picked, settings, setSetting } = draft;
  const o = option;
  const label = FIELD_LABELS[o.name] ? t(FIELD_LABELS[o.name]) : o.name;
  if (picked?.backend === "ftp" && o.name === "tls") return <FtpTlsField draft={draft} />;
  // The TLS choice above sets it along with tls.
  if (picked?.backend === "ftp" && o.name === "explicit_tls") return null;
  const ownTip = picked && OWN_TIPS[`${picked.id}:${o.name}`];
  const tip = ownTip
    ? t(ownTip)
    : o.name === "pass" && picked?.auth === "apppassword"
      ? t("dest.appPassword.tip")
      : o.name === "pass" && picked?.auth === "sshkey"
        ? t("dest.sshPass.tip")
        : o.help;
  if (o.type === "bool") {
    return (
      <SettingRow label={label} hint={tip || undefined} newId="dest-switches">
        <Toggle
          hideLabel
          label={label}
          checked={(settings[o.name] ?? o.default) === "true"}
          onChange={(on) => setSetting(o.name, on ? "true" : "false")}
        />
      </SettingRow>
    );
  }
  // For an S3 service only the examples rclone ties to it: the untied ones
  // describe signature quirks of unnamed servers. A field with a tip of its
  // own has none: for Garage's region they offer an empty one, which Garage
  // refuses.
  const s3Provider = picked?.preset?.provider;
  const examples =
    o.secret || !o.examples || ownTip
      ? []
      : o.examples
          .filter((e) => (s3Provider ? !!e.provider && e.provider.split(",").includes(s3Provider) : !e.provider))
          .slice(0, 12);
  return (
    <SettingRow label={label} hint={tip || undefined} htmlFor={id}>
      <input
        id={id}
        type={masked(o) ? "password" : "text"}
        value={settings[o.name] ?? ""}
        onChange={(e) => setSetting(o.name, e.target.value)}
        placeholder={kept ?? (o.name === "url" ? picked?.urlHint ?? o.default : o.default)}
        autoComplete={masked(o) ? "new-password" : "off"}
        spellCheck={false}
        dir="ltr"
        className={`w-72 max-w-full ${FIELD} font-mono text-start`}
      />
      {examples.length > 0 && (
        <span className="flex w-full flex-wrap justify-end gap-1.5">
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
    </SettingRow>
  );
}

type FtpTls = "none" | "explicit" | "implicit";

/** FtpTlsField is rclone's two FTP switches as the one choice they are:
 *  rclone refuses both on. Implicit TLS listens on port 990 unless the server
 *  says otherwise, so the port follows the choice while it is a default. */
function FtpTlsField({ draft }: { draft: Pick<DestinationDraftState, "settings" | "setSetting"> }) {
  const { t } = useT();
  const { settings, setSetting } = draft;
  const mode: FtpTls = settings.tls === "true" ? "implicit" : settings.explicit_tls === "true" ? "explicit" : "none";
  function choose(next: FtpTls) {
    setSetting("tls", next === "implicit" ? "true" : "false");
    setSetting("explicit_tls", next === "explicit" ? "true" : "false");
    const port = (settings.port ?? "").trim();
    if (next === "implicit" && (port === "" || port === "21")) setSetting("port", "990");
    if (next !== "implicit" && port === "990") setSetting("port", "");
  }
  return (
    <SettingRow label={t("dest.ftpTls")} newId="dest-switches">
      <SelectField
        value={mode}
        onChange={choose}
        label={t("dest.ftpTls")}
        options={[
          { value: "none", label: t("dest.ftpTls.none") },
          { value: "explicit", label: t("dest.ftpTls.explicit") },
          { value: "implicit", label: t("dest.ftpTls.implicit") },
        ]}
        className={FIELD}
      />
    </SettingRow>
  );
}

/** How well the provider suits backups, what it needs to sign in, and the
 *  form with the verdict of the last test. */
export function SignInBody({ draft, providerLabel }: { draft: DestinationDraftState; providerLabel: string }) {
  const { t } = useT();
  const { picked, fields } = draft;
  const nameId = useId();
  const bucketId = useId();
  const shareId = useId();
  const sets = useCloudCredSets();
  if (!picked) return null;
  const kind = picked.route === "s3" || picked.route === "rest" ? picked.route : null;
  const saved = kind ? signInsFor(sets, kind) : [];
  const usingSaved = kind !== null && draft.credsRef !== "";
  const chosen = saved.find((s) => s.id === draft.credsRef);
  const note = fitNote(t, picked, providerLabel);
  const command = authorizeCommand(picked, draft.settings);
  const { question } = draft;
  const questionLabel = question && (QUESTION_LABELS[question.name] ? t(QUESTION_LABELS[question.name]) : question.name);
  return (
    <>
      <div className={`rounded-card px-3 py-2 text-xs leading-relaxed ${note.tone === "warn" ? "bg-statusWarnBg text-statusWarn" : "bg-statusOkBg text-statusOk"}`}>
        {note.text}
      </div>
      {picked.id === "icloud" && (
        <div className="rounded-card bg-statusWarnBg px-3 py-2 text-xs leading-relaxed text-statusWarn">{t("dest.note.icloud")}</div>
      )}
      {picked.auth === "token" && (
        <div className="flex flex-col gap-1">
          <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("dest.token.run")}
            <InfoBubble tip={t("dest.token.tip")} />
          </span>
          {command ? <CopyBlock text={command} /> : <p className="text-xs text-carbon-textMuted">{t("dest.token.regionFirst")}</p>}
        </div>
      )}
      {picked.auth === "logintoken" && LOGIN_TOKEN_PAGES[picked.id] && (
        <div className="flex flex-col gap-1">
          <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("dest.loginToken.run")}
            <InfoBubble tip={t("dest.loginToken.tip")} />
          </span>
          <a href={LOGIN_TOKEN_PAGES[picked.id]} target="_blank" rel="noreferrer noopener" className="text-xs underline" dir="ltr">
            {LOGIN_TOKEN_PAGES[picked.id]}
          </a>
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
      {picked.route === "s3" && !usingSaved && <p className="text-xs text-carbon-textMuted">{t("dest.key.note")}</p>}
      <Rows>
        <SettingRow label={t("dest.name")} hint={t("dest.nameHint")} htmlFor={nameId}>
          <input
            id={nameId}
            value={draft.name}
            onChange={(e) => draft.setName(e.target.value)}
            autoComplete="off"
            className={`w-72 max-w-full ${FIELD}`}
          />
        </SettingRow>
        {kind && saved.length > 0 && (
          <SettingRow label={t("storage.credentials")} hint={usingSaved ? t("storage.add.savedHint") : undefined}>
            <Selector
              items={[
                { id: "saved", label: t("storage.add.savedSignIn") },
                { id: "new", label: t("storage.add.newKeys") },
              ]}
              label={t("storage.credentials")}
              size="sm"
              inline
              select="one"
              active={usingSaved ? "saved" : "new"}
              onChange={(id) => draft.setCredsRef(id === "saved" ? (defaultSignIn(saved) ?? saved[0]).id : "")}
            />
          </SettingRow>
        )}
        {usingSaved && kind && (
          <SettingRow label={t("storage.add.savedSignIn")} note={`${t("storage.add.signsInAs")}: ${(kind === "s3" ? chosen?.s3KeyId : chosen?.restUser) ?? ""}`}>
            <SelectField
              value={draft.credsRef}
              onChange={draft.setCredsRef}
              label={t("storage.add.savedSignIn")}
              options={saved.map((s) => ({ value: s.id, label: signInOption(t, s) }))}
              className={FIELD}
            />
          </SettingRow>
        )}
        {fields.main
          .filter((o) => !usingSaved || !SAVED_FIELDS.includes(o.name))
          .map((o) => (
            <DraftField key={o.name} draft={draft} option={o} />
          ))}
        {picked.route === "s3" && (
          <SettingRow label={t("dest.field.bucket")} hint={t("dest.bucketHint")} htmlFor={bucketId} newId="storage-add-bucket">
            <input
              id={bucketId}
              value={draft.bucket}
              onChange={(e) => draft.setBucket(e.target.value)}
              placeholder={endpointBucket(draft.settings.endpoint ?? "")}
              autoComplete="off"
              spellCheck={false}
              dir="ltr"
              className={`w-72 max-w-full ${FIELD} font-mono text-start`}
            />
          </SettingRow>
        )}
        {picked.shares && (
          <SettingRow label={t("dest.field.share_name")} hint={t("dest.share.tip")} htmlFor={shareId} newId="dest-smb-share">
            <input
              id={shareId}
              value={draft.share}
              onChange={(e) => draft.setShare(e.target.value)}
              autoComplete="off"
              spellCheck={false}
              dir="ltr"
              className={`w-72 max-w-full ${FIELD} font-mono text-start`}
            />
          </SettingRow>
        )}
        {question && questionLabel && (
          <SettingRow label={questionLabel} hint={t("dest.question.hint")} newId="wizard-setup-choice">
            <SelectField
              value={draft.settings[question.name] ?? ""}
              onChange={(value) => draft.setSetting(question.name, value)}
              label={questionLabel}
              options={[
                { value: "", label: t("dest.question.pick"), disabled: true },
                ...question.choices.map((c) => ({ value: c.value, label: c.help || c.value })),
              ]}
              className={`w-72 max-w-full ${FIELD}`}
            />
          </SettingRow>
        )}
      </Rows>
      {fields.more.length > 0 && (
        <MoreSettings>
          {fields.more.map((o) => (
            <DraftField key={o.name} draft={draft} option={o} />
          ))}
        </MoreSettings>
      )}
      <VerdictLine verdict={draft.test.verdict} />
      {draft.reached && draft.hostKey && (
        <div data-new="dest-host-key" className="flex flex-col gap-1">
          <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("dest.hostKey")}
            <InfoBubble tip={t("dest.hostKey.tip")} />
          </span>
          <CopyBlock text={draft.hostKey.fingerprint} />
        </div>
      )}
    </>
  );
}

/** MoreSettings holds the settings of a provider that most people leave as
 *  they are, in a sub-card under the ones everybody fills in. */
export function MoreSettings({ children }: { children: ReactNode }) {
  const { t } = useT();
  return (
    <section aria-label={t("dest.more")} className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3">
      <span className="text-[length:var(--text-caption)] font-medium uppercase tracking-[0.06em] text-carbon-textMuted">
        {t("dest.more")}
      </span>
      <Rows>{children}</Rows>
    </section>
  );
}

/** The folder the repositories go into: a walk through the provider's folders
 *  where it has any, and the address that results. */
export function FolderBody({ draft }: { draft: DestinationDraftState }) {
  const { t } = useT();
  const { picked, dir, folders, free, newFolder } = draft;
  if (!picked) return null;
  // Only the server makes a share, so its list takes no new folder.
  const atShares = !!picked.shares && dir.length === 0;
  return (
    <>
      {draft.browsable && (
        // bv-convention-exception: setting-row-beside-label: this is a folder browser, its new-folder input belongs to the list it extends.
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
          {atShares && <p className="px-2 text-xs text-carbon-textMuted">{t("dest.folder.shares")}</p>}
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
          {atShares ? null : newFolder !== null ? (
            <span className="flex gap-2 p-1">
              <input
                autoFocus
                value={newFolder}
                onChange={(e) => draft.setNewFolder(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void draft.makeFolder()}
                placeholder={t("dest.folder.newName")}
                className={`${FIELD} flex-1`}
              />
              <Button label={t("dest.folder.create")} labelKey="dest.folder.create" tone="accent" onClick={() => void draft.makeFolder()} />
            </span>
          ) : (
            <Button label={t("dest.folder.new")} labelKey="dest.folder.new" tone="neutral" onClick={() => draft.setNewFolder("")} className="self-start" />
          )}
        </div>
      )}
      {draft.needsBucket && <p className="text-xs text-statusWarn">{t(atShares ? "dest.folder.pickShare" : "dest.folder.pickBucket")}</p>}
      <div className="flex flex-col gap-1">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("dest.location")}
          <InfoBubble tip={t(`dest.route.${picked.route}` as TranslationKey)} />
        </span>
        <p dir="ltr" className="rounded-control bg-carbon-surface3 px-3 py-1.5 text-start font-mono text-sm text-carbon-text break-all">
          {draft.location}
        </p>
      </div>
      {picked.route === "s3" && (picked.storageClasses?.length ?? 0) > 0 && (
        <SelectField
          label={t("cloud.storageClass.default")}
          value={draft.storageClass}
          onChange={draft.setStorageClass}
          options={[
            { value: "", label: t("cloud.storageClass.default") },
            ...(picked.storageClasses ?? []).map((c) => ({ value: c, label: c })),
          ]}
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
      {(picked.auth === "token" || picked.auth === "logintoken") && ` ${t("dest.recovery.token").replace("{name}", () => providerLabel)}`}
      {picked.auth === "sshkey" && ` ${t("dest.recovery.ssh")}`}{" "}
      <a href={DOCS_RECOVERY} target="_blank" rel="noreferrer noopener" className="underline">
        {t("dest.recovery.link")}
      </a>
    </p>
  );
}
