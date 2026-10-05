import { useEffect, useMemo, useState } from "react";
import {
  checkDraft,
  createDestination,
  draftFolders,
  draftMakeFolder,
  getProviders,
  getVMSSH,
  type Backend,
  type BackendOption,
  type Destination,
  type DestinationDraft,
  type Provider,
} from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { useToast } from "../../lib/toast";
import { useTestVerdict } from "../../lib/useTestVerdict";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { CopyBlock } from "../CopyBlock";
import { InfoBubble } from "../InfoBubble";
import { IconFolder } from "../navGlyphs";
import { SelectField } from "../SelectField";
import { TestButton, VerdictLine } from "../TestButton";
import { Toggle } from "../Toggle";
import { ProviderPicker, providerName } from "./ProviderPicker";

const FIELD = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";
const STEP = "text-xs font-semibold text-carbon-textSub uppercase tracking-widest flex items-center gap-1.5";
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

// rclone marks user names and account ids sensitive too, which is right for
// its logs but would hide what someone is typing. A field is masked when it
// holds a password or a secret.
function masked(o: BackendOption): boolean {
  return o.password || (/(secret|token|key|pass)/.test(o.name) && !o.name.endsWith("_id"));
}

// Settings the provider decides, or BombVault fills in.
const HIDDEN = new Set(["provider", "vendor", "key_file", "env_auth", "type"]);

/** The fields of a provider's form as rclone describes them. */
function formFields(p: Provider, backend: Backend | undefined): { main: BackendOption[]; more: BackendOption[] } {
  const plain = (name: string, secret = false): BackendOption => ({
    name, help: "", required: true, secret, password: secret, advanced: false, essential: true, default: "",
  });
  if (p.route === "rest") return { main: [plain("url"), plain("user"), plain("pass", true)], more: [] };
  if (p.route === "path") return { main: [plain("path")], more: [] };
  if (!backend) return { main: [], more: [] };
  const s3Provider = p.preset?.provider;
  const applies = (o: BackendOption) =>
    !HIDDEN.has(o.name) && !(p.preset && o.name in p.preset) &&
    (!o.providers || !s3Provider || o.providers.includes(s3Provider));
  const seen = new Set<string>();
  const unique = (o: BackendOption) => !seen.has(o.name) && (seen.add(o.name), true);
  const main = backend.options.filter((o) => applies(o) && (o.essential || o.required) && unique(o));
  const more = backend.options.filter((o) => applies(o) && !o.advanced && !o.essential && !o.required && unique(o));
  return { main, more };
}

/** Where a destination will sit, as the server will write it. */
function previewLocation(p: Provider, settings: Record<string, string>, dir: string): string {
  const d = dir.replace(/^\/+|\/+$/g, "");
  if (p.route === "rest") return `rest:${(settings.url ?? "").replace(/\/+$/, "")}`;
  if (p.route === "path") return (settings.path ?? "").replace(/\/+$/, "");
  if (p.route === "s3") {
    let ep = (settings.endpoint ?? "").trim().replace(/\/+$/, "");
    if (!ep && p.id === "aws") ep = settings.region ? `s3.${settings.region}.amazonaws.com` : "s3.amazonaws.com";
    if (ep && !ep.includes("://")) ep = `https://${ep}`;
    return `s3:${ep}/${d}`;
  }
  return `rclone:${p.id}:${d}`;
}

export function DestinationWizard({ onDone, onCancel }: { onDone: (d: Destination) => void; onCancel: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [backends, setBackends] = useState<Record<string, Backend>>({});
  const [loadError, setLoadError] = useState<string | null>(null);
  const [picked, setPicked] = useState<Provider | null>(null);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [name, setName] = useState("");
  const [showMore, setShowMore] = useState(false);
  const [publicKey, setPublicKey] = useState("");
  const [dir, setDir] = useState<string[]>([]);
  const [folders, setFolders] = useState<string[] | null>(null);
  const [free, setFree] = useState<number | null>(null);
  const [folderError, setFolderError] = useState<string | null>(null);
  const [newFolder, setNewFolder] = useState<string | null>(null);
  const [immutable, setImmutable] = useState(false);
  const [storageClass, setStorageClass] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let alive = true;
    void getProviders()
      .then((r) => {
        if (!alive) return;
        if (!r.ok) {
          setLoadError(r.error ?? t("dest.loadError"));
          return;
        }
        setProviders(r.providers ?? []);
        setBackends(r.backends ?? {});
        if (r.backendsError) setLoadError(t("dest.providersError").replace("{error}", () => r.backendsError ?? ""));
      })
      .catch(() => alive && setLoadError(t("dest.loadError")));
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (picked?.auth !== "sshkey" || publicKey) return;
    void getVMSSH().then((r) => r.ok && r.publicKey && setPublicKey(r.publicKey)).catch(() => undefined);
  }, [picked, publicKey]);

  const fields = useMemo(
    () => (picked ? formFields(picked, backends[picked.backend]) : { main: [], more: [] }),
    [picked, backends]
  );
  const test = useTestVerdict([picked?.id, settings], t("offsite.testFailed"));
  const reached = test.verdict?.ok === true;
  const browsable = picked !== null && picked.route !== "rest" && picked.route !== "path";
  const dirPath = dir.join("/");

  function draft(extra: Partial<DestinationDraft> = {}): DestinationDraft {
    return { provider: picked?.id ?? "", settings, ...extra };
  }

  function failure(e: unknown, fallback: TranslationKey): string {
    return e instanceof Error && e.message ? e.message : t(fallback);
  }

  function pick(p: Provider) {
    setPicked(p);
    setSettings({});
    setName(providerName(p, t));
    setShowMore(false);
    setDir([]);
    setFolders(null);
    setFree(null);
    setImmutable(false);
    setStorageClass("");
  }

  async function loadFolders(path: string[]) {
    setFolderError(null);
    try {
      const r = await draftFolders(draft({ dir: path.join("/") }));
      if (!r.ok) {
        setFolderError(r.error ?? t("offsite.testFailed"));
        return;
      }
      setDir(path);
      setFolders(r.folders ?? []);
      if (path.length === 0) setFree(r.free ?? null);
    } catch (e) {
      setFolderError(failure(e, "offsite.testFailed"));
    }
  }

  async function runTest() {
    const v = await test.run(async () => {
      const r = await checkDraft(draft()).catch((e: unknown) => ({ ok: false, error: failure(e, "offsite.testFailed"), initialized: false }));
      if (!r.ok) return { ok: false, reason: r.error ?? t("offsite.testFailed") };
      if (picked?.route === "rest") return { ok: true, note: r.initialized ? t("dest.restExisting") : t("dest.restEmpty") };
      return { ok: true, note: t("dest.testOk") };
    });
    if (v.ok && browsable) void loadFolders([]);
  }

  async function makeFolder() {
    const child = (newFolder ?? "").trim();
    if (!child) return;
    try {
      const r = await draftMakeFolder(draft({ dir: dirPath, folder: child }));
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      setNewFolder(null);
      void loadFolders([...dir, child]);
    } catch (e) {
      push(failure(e, "settings.error"), "fail");
    }
  }

  async function save() {
    if (!picked) return;
    setSaving(true);
    try {
      const r = await createDestination(
        draft({ dir: dirPath, name: name.trim(), immutable: immutable && !!picked.lock, storageClass })
      );
      if (!r.ok || !r.destination) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      push(t("dest.saved").replace("{name}", () => r.destination?.name ?? ""), "success");
      onDone(r.destination);
    } catch (e) {
      push(failure(e, "settings.error"), "fail");
    } finally {
      setSaving(false);
    }
  }

  const shownName = picked ? providerName(picked, t) : "";
  const withName = (key: TranslationKey) => t(key).replace("{name}", () => shownName);
  const needsBucket = picked?.route === "s3" && dir.length === 0;

  function fitNote(p: Provider): { text: string; tone: "ok" | "warn" } {
    if (p.group === "cloud" && p.selfHosted) return { text: t("dest.note.selfHosted"), tone: "ok" };
    if (p.group === "cloud") {
      const cap = p.id === "gdrive" ? ` ${t("dest.note.dailyCap")}` : "";
      return { text: withName("dest.note.slower") + cap, tone: "warn" };
    }
    if (p.route === "rest") return { text: t("dest.note.rest"), tone: "ok" };
    return { text: t("dest.note.good") + (p.lock ? ` ${t("dest.note.lock")}` : ""), tone: "ok" };
  }

  function renderField(o: BackendOption) {
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
      <label key={o.name} className={`flex flex-col gap-1 ${wide ? "md:col-span-2" : ""}`}>
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {label}
          {tip && <InfoBubble tip={tip} />}
        </span>
        <input
          type={masked(o) ? "password" : "text"}
          value={settings[o.name] ?? ""}
          onChange={(e) => setSettings((s) => ({ ...s, [o.name]: e.target.value }))}
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
                onClick={() => setSettings((s) => ({ ...s, [o.name]: e.value }))}
              >
                {e.help || e.value}
              </Badge>
            ))}
          </span>
        )}
      </label>
    );
  }

  return (
    <div className="mt-2 flex flex-col gap-4 rounded-card bg-carbon-surface2 p-4">
      <section className="flex flex-col gap-3">
        <span className={STEP}>{t("dest.step.where")}</span>
        {loadError && <p className="text-xs text-statusFail">{loadError}</p>}
        <ProviderPicker providers={providers} picked={picked?.id} onPick={pick} />
        <p className="text-xs text-carbon-textMuted">{t("dest.ownConf")}</p>
      </section>

      {picked && (
        <section className="flex flex-col gap-3 border-t border-carbon-border pt-3">
          <span className={STEP}>{withName("dest.step.signIn")}</span>
          {(() => {
            const note = fitNote(picked);
            return (
              <div className={`rounded-card px-3 py-2 text-xs leading-relaxed ${note.tone === "warn" ? "bg-statusWarnBg text-statusWarn" : "bg-statusOkBg text-statusOk"}`}>
                {note.text}
              </div>
            );
          })()}
          {picked.auth === "token" && (
            <div className="flex flex-col gap-1">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("dest.token.run")}
                <InfoBubble tip={t("dest.token.tip")} />
              </span>
              <CopyBlock text={`rclone authorize "${picked.backend}"`} />
            </div>
          )}
          {picked.auth === "sshkey" && publicKey && (
            <div className="flex flex-col gap-1">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("dest.sshKey")}
                <InfoBubble tip={t("dest.sshKey.tip")} />
              </span>
              <CopyBlock text={publicKey} />
            </div>
          )}
          {picked.route === "s3" && <p className="text-xs text-carbon-textMuted">{t("dest.key.note")}</p>}
          <div className="grid gap-3 md:grid-cols-2">
            <label className="flex flex-col gap-1">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("dest.name")}
                <InfoBubble tip={t("dest.nameHint")} />
              </span>
              <input value={name} onChange={(e) => setName(e.target.value)} autoComplete="off" className={FIELD} />
            </label>
            {fields.main.map(renderField)}
            {showMore && fields.more.map(renderField)}
          </div>
          {fields.more.length > 0 && (
            <Toggle label={t("dest.more")} checked={showMore} onChange={setShowMore} />
          )}
          <VerdictLine verdict={test.verdict} />
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button label={t("dest.otherProvider")} labelKey={null} tone="neutral" onClick={() => pick({ ...picked })} />
            <TestButton label={t("offsite.test")} labelKey="offsite.test" tone="accent" test={test} onClick={() => void runTest()} />
          </div>
        </section>
      )}

      {picked && reached && (
        <section className="flex flex-col gap-3 border-t border-carbon-border pt-3">
          <span className={STEP}>
            {t("dest.step.folder")}
            <InfoBubble tip={t("dest.folder.tip")} />
          </span>
          {browsable && (
            <div className="flex flex-col gap-1 rounded-card bg-carbon-surface p-2">
              <div className="flex flex-wrap items-center gap-1 px-2 py-1 text-xs text-carbon-textMuted" dir="ltr">
                <button type="button" className="text-carbon-textSub hover:text-carbon-text" onClick={() => void loadFolders([])}>
                  {picked.id}:
                </button>
                {dir.map((d, i) => (
                  <span key={i} className="flex items-center gap-1">
                    <span>/</span>
                    <button type="button" className="text-carbon-textSub hover:text-carbon-text" onClick={() => void loadFolders(dir.slice(0, i + 1))}>
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
                  onClick={() => void loadFolders([...dir, f])}
                  className="flex items-center gap-2 rounded-control px-2 py-1.5 text-start text-sm text-carbon-text hover:bg-carbon-surface2"
                >
                  <span className="text-carbon-textMuted"><IconFolder /></span>
                  {f}
                </button>
              ))}
              {folderError && <p className="px-2 text-xs text-statusFail">{folderError}</p>}
              {newFolder !== null ? (
                <span className="flex gap-2 p-1">
                  <input
                    autoFocus
                    value={newFolder}
                    onChange={(e) => setNewFolder(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && void makeFolder()}
                    placeholder={t("dest.folder.newName")}
                    className={`${FIELD} flex-1`}
                  />
                  <Button label={t("dest.folder.create")} labelKey={null} tone="accent" onClick={() => void makeFolder()} />
                </span>
              ) : (
                <Button label={t("dest.folder.new")} labelKey="dest.folder.new" tone="neutral" onClick={() => setNewFolder("")} className="self-start" />
              )}
            </div>
          )}
          {needsBucket && <p className="text-xs text-statusWarn">{t("dest.folder.pickBucket")}</p>}
          <div className="flex flex-col gap-1">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("dest.location")}
              <InfoBubble tip={t(`dest.route.${picked.route}` as TranslationKey)} />
            </span>
            <p dir="ltr" className="rounded-control bg-carbon-surface3 px-3 py-1.5 text-start font-mono text-sm text-carbon-text break-all">
              {previewLocation(picked, settings, dirPath)}
            </p>
          </div>
          {picked.route === "s3" && (
            <SelectField
              label={t("cloud.storageClass.default")}
              value={storageClass}
              onChange={setStorageClass}
              options={[{ value: "", label: t("cloud.storageClass.default") }, ...STORAGE_CLASSES.map((c) => ({ value: c, label: c }))]}
              className={FIELD}
            />
          )}
        </section>
      )}

      {picked && reached && (
        <section className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={STEP}>{t("dest.step.protect")}</span>
          <div className="flex items-start justify-between gap-4">
            <div className="flex flex-col gap-0.5">
              <span className="text-sm text-carbon-text">{t("offsite.immutable")}</span>
              <span className="text-xs text-carbon-textMuted">{t("dest.immutableHint")}</span>
            </div>
            <Toggle hideLabel label={t("offsite.immutable")} checked={immutable} onChange={setImmutable} disabled={!picked.lock} className="mt-0.5" />
          </div>
          <div className={`rounded-card px-3 py-2 text-xs leading-relaxed ${picked.lock ? "bg-carbon-surface text-carbon-textSub" : "bg-statusWarnBg text-statusWarn"}`}>
            {picked.route === "rest" ? t("dest.lock.rest") : picked.lock ? withName("dest.lock.s3") : withName("dest.lock.none")}
          </div>
        </section>
      )}

      {picked && reached && (
        <section className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={STEP}>{t("dest.step.recovery")}</span>
          <p className="rounded-card bg-carbon-surface px-3 py-2 text-xs leading-relaxed text-carbon-textSub">
            {t("dest.recovery")}
            {picked.auth === "token" && ` ${withName("dest.recovery.token")}`}
            {picked.auth === "sshkey" && ` ${t("dest.recovery.ssh")}`}{" "}
            <a href={DOCS_RECOVERY} target="_blank" rel="noreferrer noopener" className="underline">
              {t("dest.recovery.link")}
            </a>
          </p>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onCancel} />
            <Button
              label={t("dest.save")}
              labelKey="dest.save"
              tone="accent"
              onClick={() => void save()}
              busy={saving}
              disabled={saving || needsBucket || (browsable && folders === null)}
            />
          </div>
        </section>
      )}
    </div>
  );
}
