import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "../Button";
import { CopyBlock } from "../CopyBlock";
import { FolderBrowser } from "../FolderBrowser";
import { InfoBubble } from "../InfoBubble";
import { RevealInput } from "../RevealInput";
import { SelectField } from "../SelectField";
import { Selector } from "../Selector";
import { WindowActions } from "../WindowActions";
import { PlaceMark } from "../placeMarks";
import { getVMSSH, type OkEnvelope } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import {
  domainName,
  folderStateText,
  placeErrorText,
  probeErrorText,
  probeFactText,
  probeFailureText,
  providerName,
} from "../../lib/placeText";
import {
  PLACE_DOMAINS,
  createPlace,
  probePlace,
  type CatalogField,
  type CatalogProvider,
  type Place,
  type PlaceKind,
  type ProbeResult,
} from "../../lib/places";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { MeshOffers } from "./MeshOffers";
import { RcloneConfig } from "./RcloneConfig";
import { RestServerRecipe } from "./RestServerRecipe";

// The form after a tile: the provider's fields, a connection test that adds
// nothing, and only then the name, the location question and Add. Any edit
// after the test makes it stale, so what is added is always what was tested.

const FIELD_KEYS: Record<string, TranslationKey> = {
  keyId: "places.field.keyId",
  secret: "places.field.secret",
  region: "places.field.region",
  account: "places.field.accountId",
  endpoint: "places.field.endpoint",
  bucket: "places.field.bucket",
  path: "places.field.path",
  container: "places.field.container",
  url: "places.field.url",
  user: "places.field.user",
  password: "places.field.password",
  host: "places.field.host",
  port: "places.field.port",
  remote: "places.field.remote",
};

/** fieldLabelKey is a field's label, where a kind of place names it its own way. */
export function fieldLabelKey(kind: PlaceKind, key: string): TranslationKey {
  if (kind === "azure" && key === "account") return "places.field.storageAccount";
  if (kind === "azure" && key === "secret") return "places.field.accessKey";
  if (kind === "webdav" && key === "password") return "places.field.appPassword";
  return FIELD_KEYS[key] ?? "places.field.path";
}

const INTRO_KEYS: Record<string, TranslationKey> = {
  b2: "places.intro.b2",
  s3: "places.intro.s3Cloud",
  r2: "places.intro.s3Cloud",
  wasabi: "places.intro.s3Cloud",
  "hetzner-os": "places.intro.s3Cloud",
  storj: "places.intro.s3Cloud",
  idrive: "places.intro.s3Cloud",
  scaleway: "places.intro.s3Cloud",
  ovh: "places.intro.s3Cloud",
  digitalocean: "places.intro.s3Cloud",
  ionos: "places.intro.s3Cloud",
  contabo: "places.intro.s3Cloud",
  exoscale: "places.intro.s3Cloud",
  vultr: "places.intro.s3Cloud",
  gcs: "places.intro.gcs",
  azure: "places.intro.azure",
  storagebox: "places.intro.storagebox",
  minio: "places.intro.s3Self",
  seaweedfs: "places.intro.s3Self",
  garage: "places.intro.s3Self",
  ceph: "places.intro.s3Self",
  juicefs: "places.intro.s3Self",
  rustfs: "places.intro.s3Self",
  versitygw: "places.intro.s3Self",
  "s3-other": "places.intro.s3Self",
  nextcloud: "places.intro.webdav",
  owncloud: "places.intro.webdav",
  opencloud: "places.intro.webdav",
  "rest-server": "places.intro.rest",
  sftp: "places.intro.sftp",
  bombvault: "places.intro.bombvault",
  rclone: "places.intro.rclone",
  synology: "places.intro.device",
  qnap: "places.intro.device",
  truenas: "places.intro.device",
  "unraid-other": "places.intro.device",
  share: "places.intro.device",
  "unraid-folder": "places.intro.unraidFolder",
};

const ROOT_KEYS: Record<string, TranslationKey> = {
  remotes: "places.root.remotes",
  user: "places.root.shares",
  "": "places.root.disks",
};

/** folderRoots are the folder browser's roots for a local provider, as its catalog entry names them. */
export function folderRoots(provider: CatalogProvider): { path: string; labelKey: TranslationKey }[] {
  return (provider.pickRoots ?? []).map((path) => ({ path, labelKey: ROOT_KEYS[path] ?? "places.root.disks" }));
}

const FIELD_CLASS = "w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** The empty first choice of a field that lists what the probe found. */
const CHOOSE_KEYS: Record<string, TranslationKey> = {
  bucket: "places.form.chooseBucket",
  container: "places.form.chooseContainer",
  remote: "places.form.chooseRemote",
};

function SecretField({ id, value, onChange }: { id: string; value: string; onChange: (v: string) => void }) {
  const reveal = useReveal();
  return (
    <RevealInput
      {...reveal}
      id={id}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      autoComplete="off"
      spellCheck={false}
      wrapperClassName="w-full"
      className={`${FIELD_CLASS} font-mono`}
    />
  );
}

/** The key an SFTP server needs, since BombVault signs in with it and no password. */
function PublicKey() {
  const { t } = useT();
  const [key, setKey] = useState("");
  useEffect(() => {
    getVMSSH()
      .then((r) => setKey(r.ok ? (r.publicKey ?? "") : ""))
      .catch(() => setKey(""));
  }, []);
  if (!key) return null;
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("places.form.publicKey")}
        <InfoBubble tip={t("places.form.publicKeyHint")} />
      </span>
      <CopyBlock text={key} t={t} />
    </div>
  );
}

export function PlaceForm({
  provider,
  hostMountRoot,
  onBack,
  onCancel,
  onAdded,
  onAccepted,
}: {
  provider: CatalogProvider;
  hostMountRoot: string;
  onBack: () => void;
  onCancel: () => void;
  onAdded: (place: Place) => void;
  /** Gets the place an accepted offer made, which already takes its domain's copies. */
  onAccepted: (place: Place) => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [fields, setFields] = useState<Record<string, string>>(() =>
    Object.fromEntries(provider.fields.map((f) => [f.key, ""]))
  );
  const [probe, setProbe] = useState<(OkEnvelope & ProbeResult) | null>(null);
  const [probedWith, setProbedWith] = useState("");
  const [probing, setProbing] = useState(false);
  const [name, setName] = useState(() => providerName(t, provider.id));
  const [where, setWhere] = useState<"here" | "away" | null>(null);
  const [adding, setAdding] = useState(false);
  const [shake, setShake] = useState({ test: 0, add: 0 });
  const [remotes, setRemotes] = useState<string[]>([]);
  const bodyRef = useRef<HTMLDivElement>(null);

  const typed = JSON.stringify(fields);
  const fresh = probe !== null && probedWith === typed ? probe : null;
  // An answer that lists buckets or containers alone waits for one to be
  // chosen. A new WebDAV place has folders or a repository and no address,
  // since its remote is named when the place is added.
  const ready = fresh?.ok === true && (!!fresh.base || !!fresh.folders || !!fresh.repoIds);
  const asks = provider.offPremises === undefined;
  const waitsFor: TranslationKey | null = !ready
    ? "places.form.testFirst"
    : name.trim() === ""
      ? "places.form.nameFirst"
      : asks && where === null
        ? "places.form.whereFirst"
        : null;
  const canAdd = waitsFor === null && !adding;

  // The tile that opened the form is gone, and focus with it.
  useEffect(() => {
    bodyRef.current?.querySelector("input")?.focus();
  }, []);

  // The stored remotes turn the remote field into a picker, and the text
  // field it replaces takes the focus along.
  useEffect(() => {
    if (remotes.length > 0 && document.activeElement === document.body) {
      bodyRef.current?.querySelector<HTMLElement>("#place-field-remote")?.focus();
    }
  }, [remotes]);

  // A saved config can change what the tested remote points at, so it wants a
  // new test the way an edited field does.
  const rcloneRemotes = useCallback((list: string[]) => {
    setRemotes(list);
    setProbe(null);
  }, []);

  function set(key: string, value: string) {
    setFields((f) => ({ ...f, [key]: value }));
  }

  async function test() {
    setProbing(true);
    try {
      const res = await probePlace({ provider: provider.id, fields });
      setProbe(res);
      setProbedWith(typed);
      if (!res.ok) {
        push(probeFailureText(t, lang, res), "fail");
        setShake((s) => ({ ...s, test: s.test + 1 }));
      }
    } catch (err) {
      setProbe(null);
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      setShake((s) => ({ ...s, test: s.test + 1 }));
    } finally {
      setProbing(false);
    }
  }

  async function add() {
    if (!fresh || !canAdd) return;
    setAdding(true);
    try {
      // The probe hands back what it completed, such as B2's endpoint and
      // bucket; the secrets stay the typed ones. An address that already
      // holds a repository becomes a place that is itself that repository.
      const res = await createPlace({
        provider: provider.id,
        fields: { ...fields, ...(fresh.fields ?? {}) },
        name: name.trim(),
        offPremises: asks ? where === "away" : undefined,
        folders: fresh.repoIds?.[""] ? Object.fromEntries(PLACE_DOMAINS.map((d) => [d, ""])) : undefined,
      });
      if (res.ok && res.place) {
        onAdded(res.place);
        return;
      }
      if (res.code === "place-probe-failed") setProbe(null);
      push(placeErrorText(t, lang, res, "common.actionFailed"), "fail");
      setShake((s) => ({ ...s, add: s.add + 1 }));
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      setShake((s) => ({ ...s, add: s.add + 1 }));
    } finally {
      setAdding(false);
    }
  }

  function renderField(f: CatalogField) {
    const id = `place-field-${f.key}`;
    const label = t(fieldLabelKey(provider.kind, f.key));
    if (f.key === "path" && provider.kind === "local") {
      return (
        <FolderBrowser
          key={f.key}
          label={label}
          value={fields.path ?? ""}
          onChange={(v) => set("path", v)}
          hostMountRoot={hostMountRoot}
          placeholder={f.placeholder}
          inDialog
          roots={folderRoots(provider)}
        />
      );
    }
    // An Azure account's containers come back as buckets, as a key's buckets do.
    const listed =
      f.key === "bucket" || f.key === "container" ? (probe?.buckets ?? []) : f.key === "remote" ? remotes : [];
    // A name typed before the list came, such as a bucket the server will
    // create, stays a choice rather than a blank.
    const value = fields[f.key] ?? "";
    const choices = value !== "" && !listed.includes(value) ? [value, ...listed] : listed;
    return (
      <div key={f.key} className="flex flex-col gap-1.5">
        <label htmlFor={id} className="flex items-center gap-1 text-xs text-carbon-textSub">
          {label}
          {f.optional && <span className="text-carbon-textMuted">{t("places.form.optional")}</span>}
        </label>
        {listed.length > 0 ? (
          <SelectField
            id={id}
            label={label}
            value={value}
            onChange={(v) => set(f.key, v)}
            options={[
              { value: "", label: t(CHOOSE_KEYS[f.key] ?? "places.form.chooseBucket") },
              ...choices.map((b) => ({ value: b, label: b })),
            ]}
            className={FIELD_CLASS}
          />
        ) : f.secret ? (
          <SecretField id={id} value={value} onChange={(v) => set(f.key, v)} />
        ) : (
          <input
            id={id}
            type="text"
            dir="ltr"
            value={value}
            onChange={(e) => set(f.key, e.target.value)}
            placeholder={f.key === "port" && provider.defaultPort ? String(provider.defaultPort) : f.placeholder}
            inputMode={f.key === "port" ? "numeric" : undefined}
            autoComplete="off"
            spellCheck={false}
            className={`${FIELD_CLASS} text-start`}
          />
        )}
      </div>
    );
  }

  const facts = (fresh?.facts ?? []).map((f) => probeFactText(t, f)).filter((s): s is string => s !== null);
  const probed = PLACE_DOMAINS.filter((d) => fresh?.folders?.[d]);
  // A key that may list its buckets answers with the list alone until one is
  // chosen, and an empty panel would read as a result.
  const found = fresh?.ok && (fresh.base || facts.length > 0 || probed.length > 0) ? fresh : null;

  return (
    <>
      <div ref={bodyRef} className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 pb-4">
        <div className="flex items-center gap-3">
          <PlaceMark provider={provider.id} size={32} />
          <span className="text-sm font-semibold text-carbon-text">{providerName(t, provider.id)}</span>
          <InfoBubble tip={t(INTRO_KEYS[provider.id] ?? "places.intro.s3Self")} />
        </div>

        {provider.id === "rest-server" && (
          <RestServerRecipe onLogin={(user, password) => setFields((f) => ({ ...f, user, password }))} />
        )}
        {provider.id === "bombvault" && <MeshOffers onAccepted={onAccepted} />}

        {provider.fields.map(renderField)}
        {provider.kind === "sftp" && <PublicKey />}
        {provider.kind === "rclone" && <RcloneConfig onRemotes={rcloneRemotes} />}

        {found && (
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3 text-sm text-carbon-text" aria-live="polite">
            {found.base && (
              <span className="flex flex-wrap gap-x-2">
                <span className="text-carbon-textSub">{t("places.form.address")}</span>
                <span dir="ltr" className="break-all font-mono text-xs">
                  {found.base}
                </span>
              </span>
            )}
            {facts.map((fact) => (
              <span key={fact}>{fact}</span>
            ))}
            {probed.map((d) => (
              <span key={d} className="flex flex-wrap gap-x-2">
                <span className="text-carbon-textSub">{domainName(t, d)}</span>
                <span>{folderStateText(t, found.folders![d]!)}</span>
                {found.errors?.[d] && <span className="text-statusFail">{probeErrorText(t, lang, found.errors[d]!)}</span>}
              </span>
            ))}
          </div>
        )}

        {ready && (
          <div className="flex flex-col gap-1.5">
            <label htmlFor="place-name" className="text-xs text-carbon-textSub">
              {t("places.form.name")}
            </label>
            <input
              id="place-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="off"
              className={FIELD_CLASS}
            />
          </div>
        )}

        {ready && asks && (
          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("places.form.where")}
              <InfoBubble tip={t("places.form.whereHint")} />
            </span>
            <Selector
              label={t("places.form.where")}
              variant="well"
              items={[
                { id: "here", label: t("places.form.here") },
                { id: "away", label: t("places.form.away") },
              ]}
              active={where}
              onChange={(id) => setWhere(id === "away" ? "away" : "here")}
            />
          </div>
        )}
      </div>

      <WindowActions>
        <Button label={t("places.form.back")} labelKey="places.form.back" tone="neutral" onClick={onBack} />
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onCancel} />
        <Button
          key={`test-${shake.test}`}
          label={t("places.form.test")}
          labelKey="places.form.test"
          tone={ready ? "neutral" : "accent"}
          busy={probing}
          disabled={probing || adding}
          onClick={() => void test()}
          className={shake.test ? "glim-shake" : ""}
        />
        <Button
          key={`add-${shake.add}`}
          label={t("places.form.add")}
          labelKey="places.form.add"
          tone={ready ? "accent" : "neutral"}
          busy={adding}
          disabled={!canAdd}
          title={waitsFor ? t(waitsFor) : undefined}
          onClick={() => void add()}
          className={shake.add ? "glim-shake" : ""}
        />
      </WindowActions>
    </>
  );
}
