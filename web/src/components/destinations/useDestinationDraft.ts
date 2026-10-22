import { useEffect, useMemo, useState } from "react";
import {
  checkDraft,
  createDestination,
  draftFolders,
  draftMakeFolder,
  getProviders,
  getVMSSH,
  groupReceiverLogin,
  type Backend,
  type BackendOption,
  type Destination,
  type DestinationDraft,
  type DraftQuestion,
  type GroupReceiver,
  type Provider,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useTestVerdict, type TestVerdictState } from "../../lib/useTestVerdict";
import { providerName } from "./ProviderPicker";

// Settings the provider decides, or BombVault fills in.
const HIDDEN = new Set(["provider", "vendor", "key_file", "env_auth", "type"]);

export interface FormFields {
  main: BackendOption[];
  more: BackendOption[];
}

/** The fields of a provider's form as rclone describes them. */
export function formFields(p: Provider, backend: Backend | undefined): FormFields {
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
export function previewLocation(p: Provider, settings: Record<string, string>, dir: string): string {
  const d = dir.replace(/^\/+|\/+$/g, "");
  if (p.route === "rest") return `rest:${(settings.url ?? "").replace(/\/+$/, "")}`;
  if (p.route === "path") return (settings.path ?? "").replace(/\/+$/, "");
  if (p.route === "s3") {
    let ep = (settings.endpoint ?? "").trim();
    // A restic address pasted into the field: the server keeps its scheme and host.
    const restic = /^s3:([a-z]+:\/\/)?([^/]*)/i.exec(ep);
    if (restic) ep = (restic[1] ?? "") + restic[2];
    ep = ep.replace(/\/+$/, "");
    if (!ep && p.id === "aws") ep = settings.region ? `s3.${settings.region}.amazonaws.com` : "s3.amazonaws.com";
    if (ep && !ep.includes("://")) ep = `https://${ep}`;
    return `s3:${ep}/${d}`;
  }
  return `rclone:${p.id}:${d}`;
}

export interface DestinationDraftState {
  providers: Provider[];
  loadError: string | null;
  picked: Provider | null;
  /** The member whose receiver filled the rest-server form, if one did. */
  fromMember: string | undefined;
  settings: Record<string, string>;
  setSetting: (name: string, value: string) => void;
  name: string;
  setName: (name: string) => void;
  /** The saved sign-in the draft signs in with instead of typed keys, ""
   *  for the keys in the form. */
  credsRef: string;
  setCredsRef: (id: string) => void;
  publicKey: string;
  fields: FormFields;
  test: TestVerdictState;
  /** What the last test left to choose before the connection can stand. */
  question: DraftQuestion | null;
  /** The connection was tested and stands. */
  reached: boolean;
  /** The provider has folders to walk through. */
  browsable: boolean;
  dir: string[];
  folders: string[] | null;
  free: number | null;
  folderError: string | null;
  newFolder: string | null;
  setNewFolder: (name: string | null) => void;
  immutable: boolean;
  setImmutable: (on: boolean) => void;
  storageClass: string;
  setStorageClass: (storageClass: string) => void;
  saving: boolean;
  /** An S3 service is still at its list of buckets. */
  needsBucket: boolean;
  /** The folder is not settled yet, so the destination cannot be saved. */
  folderOpen: boolean;
  location: string;
  pick: (p: Provider) => void;
  /** Fills the rest-server form with a login on the member's receiver.
   *  Resolves to whether it did. */
  pickFromGroup: (r: GroupReceiver) => Promise<boolean>;
  loadFolders: (path: string[]) => Promise<void>;
  runTest: () => Promise<boolean>;
  makeFolder: () => Promise<void>;
  /** Creates the destination, or reports why not and resolves to null. */
  save: () => Promise<Destination | null>;
}

/**
 * useDestinationDraft holds a destination while it is being set up: the
 * provider, its sign-in, the folder and the delete protection, with the calls
 * that test and save it.
 */
export function useDestinationDraft(): DestinationDraftState {
  const { t } = useT();
  const { push } = useToast();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [backends, setBackends] = useState<Record<string, Backend>>({});
  const [loadError, setLoadError] = useState<string | null>(null);
  const [picked, setPicked] = useState<Provider | null>(null);
  const [fromMember, setFromMember] = useState<string | undefined>(undefined);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [name, setName] = useState("");
  const [credsRef, setCredsRef] = useState("");
  const [publicKey, setPublicKey] = useState("");
  const [dir, setDir] = useState<string[]>([]);
  const [folders, setFolders] = useState<string[] | null>(null);
  const [free, setFree] = useState<number | null>(null);
  const [folderError, setFolderError] = useState<string | null>(null);
  const [newFolder, setNewFolder] = useState<string | null>(null);
  const [immutable, setImmutable] = useState(false);
  const [storageClass, setStorageClass] = useState("");
  const [saving, setSaving] = useState(false);
  const [question, setQuestion] = useState<DraftQuestion | null>(null);

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
  const test = useTestVerdict([picked?.id, settings, credsRef], t("offsite.testFailed"));
  const reached = test.verdict?.ok === true;
  const browsable = picked !== null && picked.route !== "rest" && picked.route !== "path";
  const dirPath = dir.join("/");

  function draft(extra: Partial<DestinationDraft> = {}): DestinationDraft {
    return { provider: picked?.id ?? "", settings, ...(credsRef ? { credsRef } : {}), ...extra };
  }

  function failure(e: unknown, fallback: TranslationKey): string {
    return e instanceof Error && e.message ? e.message : t(fallback);
  }

  function pick(p: Provider) {
    setFromMember(undefined);
    setPicked(p);
    setSettings({});
    setQuestion(null);
    setCredsRef("");
    setName(providerName(p, t));
    setDir([]);
    setFolders(null);
    setFree(null);
    setImmutable(false);
    setStorageClass("");
  }

  async function pickFromGroup(r: GroupReceiver) {
    const rest = providers.find((p) => p.route === "rest");
    if (!rest) return false;
    try {
      const res = await groupReceiverLogin(r.memberId);
      if (!res.ok || !res.login) {
        push(res.error ?? t("dest.loadError"), "fail");
        return false;
      }
      const login = res.login;
      pick(rest);
      setFromMember(r.memberId);
      setName(login.name);
      setSettings({ url: login.url, user: login.user, pass: login.password });
      return true;
    } catch (e) {
      push(failure(e, "dest.loadError"), "fail");
      return false;
    }
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
      if ("question" in r && r.question) {
        setQuestion(r.question);
        return { ok: false, reason: t("dest.question.hint") };
      }
      if (picked?.route === "rest") return { ok: true, note: r.initialized ? t("dest.restExisting") : t("dest.restEmpty") };
      return { ok: true, note: t("dest.testOk") };
    });
    if (v.ok && browsable) void loadFolders([]);
    return v.ok;
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
    if (!picked) return null;
    setSaving(true);
    try {
      const r = await createDestination(
        draft({ dir: dirPath, name: name.trim(), immutable: immutable && !!picked.lock, storageClass })
      );
      if (!r.ok || !r.destination) {
        push(r.error ?? t("settings.error"), "fail");
        return null;
      }
      push(t("dest.saved").replace("{name}", () => r.destination?.name ?? ""), "success");
      return r.destination;
    } catch (e) {
      push(failure(e, "settings.error"), "fail");
      return null;
    } finally {
      setSaving(false);
    }
  }

  const needsBucket = picked?.route === "s3" && dir.length === 0;

  return {
    providers,
    loadError,
    picked,
    fromMember,
    settings,
    setSetting: (field, value) => {
      // A changed sign-in may open other drives, so the choice is asked again.
      const stale = question && field !== question.name ? question.name : null;
      if (stale) setQuestion(null);
      setSettings((s) => {
        const next = { ...s, [field]: value };
        if (stale) delete next[stale];
        return next;
      });
    },
    name,
    setName,
    credsRef,
    setCredsRef,
    publicKey,
    fields,
    test,
    question,
    reached,
    browsable,
    dir,
    folders,
    free,
    folderError,
    newFolder,
    setNewFolder,
    immutable,
    setImmutable,
    storageClass,
    setStorageClass,
    saving,
    needsBucket,
    folderOpen: needsBucket || (browsable && folders === null),
    location: picked ? previewLocation(picked, settings, dirPath) : "",
    pick,
    pickFromGroup,
    loadFolders,
    runTest,
    makeFolder,
    save,
  };
}
