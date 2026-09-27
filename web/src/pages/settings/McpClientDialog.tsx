import { useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { IconCheckCircle, IconCopy, IconDownload } from "../../components/navGlyphs";
import { RevealInput } from "../../components/RevealInput";
import { HUE_OFFSET, Selector } from "../../components/Selector";
import { Toggle } from "../../components/Toggle";
import type { McpKeyView, McpOAuthView } from "../../lib/api";
import type { TranslationKey } from "../../lib/i18n";
import { KIND_LABEL, OTHER_CLIENT, clientById, snippetFor, type ConfigPaths, type McpClient } from "../../lib/mcpClients";
import { KEY_PLACEHOLDER, KEY_VARIABLE, type McpSnippetInput } from "../../lib/mcpSnippets";
import { tLtr } from "../../lib/ltrFragments";
import { focusableElements } from "../../lib/useConfirm";
import { ClientMark } from "./McpClientMark";
import { connectorUrl, McpOAuthSettings } from "./McpOAuthSettings";

/** A key handed out by create, with the row it belongs to. */
export interface FreshKey {
  key: string;
  id: string;
  hint: string;
}

type T = (key: TranslationKey, count?: number) => string;

export interface McpClientDialogProps {
  client: McpClient;
  /** The keys that still work, which the client can be given instead of a new one. */
  keys: McpKeyView[];
  /** Counts the key lists the card has fetched, so the dialog can tell a
   *  fresh one from the one it opened with. */
  lists: number;
  /** The address, endpoint and certificate state the snippets are written for. */
  snippetBase: Omit<McpSnippetInput, "key">;
  /** Why this address may not create a key; undefined when it may. */
  mintRefused?: string;
  /** Why no further key can be made, while the active keys are at the limit. */
  limitNote?: string;
  /** The (i) of the permission switch, with the server's limits filled in. */
  allowStartHint: string;
  /** Sign-in through OAuth, which a client that takes no key sets up with. */
  oauth: McpOAuthView;
  authEnabled: boolean;
  onOAuthChange: (next: McpOAuthView) => void;
  /** Creates the key; null when the server refused, which the card reports. */
  onCreate: (label: string, canStartBackups: boolean) => Promise<FreshKey | null>;
  onCopy: (text: string) => void;
  onDownloadCertificate: () => void;
  /** Names the key made here once a call has used it, so the card can stop
   *  showing it. */
  onClose: (usedKey: string | null) => void;
  t: T;
}

const OS_PATHS: [keyof ConfigPaths, string][] = [
  ["windows", "Windows"],
  ["macos", "macOS"],
  ["linux", "Linux"],
  ["unix", "macOS, Linux"],
];

/**
 * McpClientDialog sets up one client in three steps: a key, the configuration
 * for that client, and the wait for its first call, which it sees in the key's
 * last use while the card polls the list. A cloud client carries the warning
 * that BombVault must then face the internet. One that only signs in through
 * OAuth gets sign-in as its first step instead of a key, and the dialog waits
 * for its grant to appear.
 */
export function McpClientDialog({
  client,
  keys,
  lists,
  snippetBase,
  mintRefused,
  limitNote,
  allowStartHint,
  oauth,
  authEnabled,
  onOAuthChange,
  onCreate,
  onCopy,
  onDownloadCertificate,
  onClose,
  t,
}: McpClientDialogProps) {
  const other = client.id === OTHER_CLIENT.id;
  const canMint = mintRefused === undefined;
  const name = other ? t("mcp.otherClient") : client.name;
  const cardRef = useRef<HTMLDivElement>(null);
  // A grant belongs to the client that signed in and cannot be handed to
  // another one, so only keys are offered for reuse.
  const staticKeys = keys.filter((k) => !k.viaOAuth);
  const [mode, setMode] = useState<"new" | "pick">(
    (canMint && !limitNote) || staticKeys.every((k) => k.unusable) ? "new" : "pick"
  );
  const [label, setLabel] = useState(other ? "" : client.name);
  const [allowStart, setAllowStart] = useState(false);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [keyVisible, setKeyVisible] = useState(true);
  const [created, setCreated] = useState<FreshKey | null>(null);
  const [picked, setPicked] = useState<{ id: string; since: number } | null>(null);
  // Each key's last use in the first list fetched after the dialog opened. The
  // list it opened with can be minutes old, and a call from before the dialog
  // must not count as the client's first.
  const [openedAt] = useState(lists);
  const [baseline, setBaseline] = useState<Record<string, number> | null>(null);
  if (baseline === null && lists !== openedAt) {
    setBaseline(Object.fromEntries(keys.map((k) => [k.id, k.lastUsedAt])));
  }

  const chosenId = created?.id ?? picked?.id;
  const chosen = keys.find((k) => k.id === chosenId);
  let since = 0;
  if (!created && picked) since = baseline ? Math.max(picked.since, baseline[picked.id] ?? 0) : Infinity;
  // For an OAuth client the sign-in itself is the proof: a grant of this
  // client that the first list after opening did not have.
  const signedIn = client.oauth
    ? keys.find((k) => k.viaOAuth && k.client === client.id && baseline !== null && !(k.id in baseline))
    : undefined;
  const connected = client.oauth ? signedIn !== undefined : chosen !== undefined && chosen.lastUsedAt > since;

  const closeRef = useRef(() => onClose(null));
  closeRef.current = () => onClose(created && connected ? created.id : null);

  useEffect(() => {
    const trigger = document.activeElement;
    cardRef.current?.focus();
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        closeRef.current();
        return;
      }
      const card = cardRef.current;
      if (e.key !== "Tab" || !card) return;
      const focusables = focusableElements(card);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const inside = document.activeElement instanceof Node && card.contains(document.activeElement);
      if (e.shiftKey ? !inside || document.activeElement === first : !inside || document.activeElement === last) {
        e.preventDefault();
        (e.shiftKey ? last : first).focus();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      if (trigger instanceof HTMLElement && document.contains(trigger)) trigger.focus();
    };
  }, []);

  async function create() {
    const trimmed = label.trim();
    if (trimmed === "") {
      setShake((n) => n + 1);
      return;
    }
    setBusy(true);
    const fresh = await onCreate(trimmed, allowStart);
    setBusy(false);
    if (fresh) setCreated(fresh);
    else setShake((n) => n + 1);
  }

  const snippet = snippetFor(client, { ...snippetBase, key: created?.key ?? KEY_PLACEHOLDER });
  // Hiding the key field hides the key in the configuration below it too; Copy
  // still takes the real one.
  const shownSnippet =
    created && !keyVisible ? snippetFor(client, { ...snippetBase, key: "•".repeat(created.key.length) }) : snippet;
  const warning = client.group === "cloud" && (
    <div className="flex flex-col gap-1 rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
      <strong className="font-semibold text-statusWarn">{t("mcp.cloudWarningTitle")}</strong>
      <span>{t("mcp.cloudWarning")}</span>
    </div>
  );

  const who = (
    <div className="flex items-center gap-3">
      <span className="glim-client-mark glim-client-mark-lg" aria-hidden="true">
        <ClientMark client={client} />
      </span>
      <span className="flex flex-col">
        <span className="text-sm font-semibold leading-snug text-carbon-text">{name}</span>
        {client.kind && <span className="text-xs text-carbon-textSub">{t(KIND_LABEL[client.kind])}</span>}
      </span>
    </div>
  );

  const fill = (key: TranslationKey, values: Record<string, string>) =>
    Object.entries(values).reduce((s, [k, v]) => s.replaceAll(`{${k}}`, v), t(key));

  /** fill for a sentence that names a variable or a file, which it sets as
   *  code so it can be told apart from the words around it. */
  const rich = (key: TranslationKey, values: Record<string, string>): ReactNode[] =>
    t(key)
      .split(/({[a-z]+})/)
      .map((part, i) => {
        const name = /^{([a-z]+)}$/.exec(part)?.[1];
        if (name === undefined || !(name in values)) return part;
        if (name !== "var" && name !== "file") return values[name];
        return (
          <code key={i} dir="ltr" className="rounded-control bg-carbon-surface2 px-1 font-mono text-[0.78em]">
            {values[name]}
          </code>
        );
      });

  /** A sentence with its (i) at the end of the text, where the eye stops. */
  const withTip = (line: ReactNode, tip: string | undefined) => (
    <>
      {line}
      {tip && (
        <span className="ms-1 inline-flex align-[-0.125rem]">
          <InfoBubble tip={tip} />
        </span>
      )}
    </>
  );

  let body;
  if (client.oauth) {
    body = (
      <>
        {warning}
        {who}
        {oauthStep()}
        {oauthConfigStep(client.oauth)}
        {oauthWaitStep()}
      </>
    );
  } else {
    body = (
      <>
        {warning}
        {who}
        {client.note && <p className="text-sm text-carbon-textSub">{t(client.note)}</p>}
        {keyStep()}
        {configStep()}
        {waitStep()}
      </>
    );
  }

  function oauthStep() {
    const grants = keys.filter((k) => k.viaOAuth).length;
    return (
      <Step n={1} title={t("mcp.stepOAuth")} done={oauth.active}>
        <McpOAuthSettings
          oauth={oauth}
          authEnabled={authEnabled}
          onChange={onOAuthChange}
          idPrefix="bv-mcp-dialog"
          t={t}
        />
        {oauth.active && grants >= oauth.grantLimit && (
          <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
            {t("oauth.limitReached")}
          </p>
        )}
      </Step>
    );
  }

  function oauthConfigStep(setup: TranslationKey) {
    const connector = connectorUrl(oauth);
    return (
      <Step n={2} title={fill("mcp.stepConfig", { name })}>
        <p className="text-sm text-carbon-textSub">{t(setup)}</p>
        {connector !== "" && (
          <>
            <pre
              dir="ltr"
              className="overflow-x-auto whitespace-pre-wrap rounded-control bg-carbon-surface2 px-3 py-2 text-start font-mono text-xs leading-relaxed text-carbon-text [overflow-wrap:anywhere]"
            >
              <code>{connector}</code>
            </pre>
            <Button
              label={t("mcp.copyConnector")}
              labelKey="mcp.copyConnector"
              glyph={<IconCopy />}
              tone="subtle"
              onClick={() => onCopy(connector)}
              className="self-start"
            />
          </>
        )}
        <p className="text-sm text-carbon-textSub">{tLtr(t, "mcp.oauthProxyNote")}</p>
      </Step>
    );
  }

  function oauthWaitStep() {
    let content;
    if (signedIn) {
      content = (
        <p role="status" className="flex items-start gap-2 text-sm text-statusOk">
          <span className="mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full bg-statusOkSolid" />
          {fill("mcp.oauthSignedIn", {
            name: signedIn.label,
            time: new Date(signedIn.createdAt * 1000).toLocaleTimeString(),
          })}
        </p>
      );
    } else {
      content = (
        <>
          <p role="status" className="flex items-start gap-2 text-sm text-carbon-text">
            <span className="glim-wait-dot mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full bg-accent" />
            {fill("mcp.oauthWaitLine", { app: name })}
          </p>
          <p className="text-xs text-carbon-textMuted">{fill("mcp.oauthWaitHint", { app: name })}</p>
        </>
      );
    }
    return (
      <Step n={3} title={connected ? t("mcp.connectedTitle") : t("mcp.oauthWaitTitle")} done={connected}>
        {content}
      </Step>
    );
  }

  function keyStep() {
    const done = created !== null || picked !== null;
    const usable = staticKeys.filter((k) => !k.unusable);
    let content;
    if (created) {
      content = (
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2.5">
          <p className="text-sm font-medium text-carbon-text">{t("mcp.newKeyTitle")}</p>
          <p className="text-sm text-carbon-textSub">{t("mcp.showOnce")}</p>
          <RevealInput
            visible={keyVisible}
            onToggleVisible={() => setKeyVisible((v) => !v)}
            showLabel={t("common.showValue")}
            hideLabel={t("common.hideValue")}
            value={created.key}
            readOnly
            aria-label={t("mcp.newKeyTitle")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface px-3 py-1.5 font-mono text-sm text-carbon-text glim-field-focus"
          />
          <Button
            label={t("mcp.copyKey")}
            labelKey="mcp.copyKey"
            glyph={<IconCopy />}
            tone="subtle"
            onClick={() => onCopy(created.key)}
            className="self-start"
          />
        </div>
      );
    } else {
      const items = [];
      if (canMint) items.push({ id: "new", label: t("mcp.newKey") });
      if (usable.length > 0) items.push({ id: "pick", label: t("mcp.existingKey") });
      content = (
        <>
          {items.length === 0 && (
            <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              {mintRefused}
            </p>
          )}
          {items.length > 1 && (
            <Selector
              items={items}
              label={t("mcp.stepKey")}
              active={mode}
              onChange={(id) => setMode(id as "new" | "pick")}
              hueOffset={HUE_OFFSET.mcpClient}
              className="self-start"
            />
          )}
          {mode === "new" && canMint && (
            <>
              <div className="flex flex-col gap-1.5">
                <label htmlFor="bv-mcp-dialog-label" className="text-xs text-carbon-textSub">
                  {t("mcp.labelLabel")}
                </label>
                <input
                  key={shake}
                  id="bv-mcp-dialog-label"
                  value={label}
                  maxLength={64}
                  placeholder={t("mcp.labelPlaceholder")}
                  onChange={(e) => setLabel(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void create();
                  }}
                  className={`w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus${
                    shake ? " glim-shake" : ""
                  }`}
                />
              </div>
              <span className="flex items-center gap-1.5">
                <Toggle label={t("mcp.allowStart")} checked={allowStart} onChange={setAllowStart} />
                <InfoBubble tip={allowStartHint} />
              </span>
              <Button
                label={t("mcp.createKey")}
                labelKey="mcp.createKey"
                tone="accent"
                onClick={() => void create()}
                disabled={busy || limitNote !== undefined}
                busy={busy}
                hint={limitNote}
                className="self-start"
              />
            </>
          )}
          {mode === "pick" && (
            <>
              <ul className="flex flex-col gap-1.5">
                {usable.map((k) => {
                  const on = picked?.id === k.id;
                  return (
                    <li key={k.id}>
                      <button
                        type="button"
                        aria-pressed={on}
                        onClick={() => setPicked({ id: k.id, since: k.lastUsedAt })}
                        className={`flex w-full items-center gap-2.5 rounded-control bg-carbon-surface2 px-3 py-2 text-start [--mark-ground:var(--carbon-surface2)] hover:bg-carbon-surface3 hover:[--mark-ground:var(--carbon-surface3)] focus-visible:outline-2 focus-visible:outline-(--focus-ring)${
                          on ? " shadow-[inset_0_0_0_1.5px_var(--accent-text)]" : ""
                        }`}
                      >
                        <span className="glim-client-mark" aria-hidden="true">
                          <ClientMark client={clientById(k.client)} forKey />
                        </span>
                        <span className="flex min-w-0 flex-col">
                          <span className="truncate text-sm text-carbon-text">{k.label}</span>
                          <span className="text-xs text-carbon-textMuted">
                            {t("mcp.keyHint").replace("{hint}", k.hint)}
                          </span>
                        </span>
                      </button>
                    </li>
                  );
                })}
              </ul>
              <p className="text-xs text-carbon-textSub">{t("mcp.pickHint")}</p>
              {picked && <p className="text-xs text-carbon-textSub">{t("mcp.pickShared")}</p>}
            </>
          )}
        </>
      );
    }
    return (
      <Step n={1} title={t("mcp.stepKey")} done={done}>
        {content}
      </Step>
    );
  }

  function configStep() {
    const setup = client.setup;
    const copyKey: TranslationKey =
      setup.kind === "command" ? "mcp.copyCommand" : setup.kind === "file" ? "mcp.copyConfig" : "mcp.copySettings";
    const intro =
      setup.kind === "command"
        ? t("mcp.setupCommand")
        : setup.kind === "file"
          ? t("mcp.setupFile")
          : setup.kind === "form"
            ? fill("mcp.setupForm", { app: name, ui: setup.ui })
            : t("mcp.setupOther");
    return (
      <Step n={2} title={other ? t("mcp.stepConfigOther") : fill("mcp.stepConfig", { name })}>
        <p className="text-sm text-carbon-textSub">
          {intro}
          {client.node && ` ${fill("mcp.setupNeedsNode", { app: name })}`}
        </p>
        {snippet !== undefined && (
          <pre
            dir="ltr"
            className="overflow-x-auto whitespace-pre-wrap rounded-control bg-carbon-surface2 px-3 py-2 text-start font-mono text-xs leading-relaxed text-carbon-text [overflow-wrap:anywhere]"
          >
            <code>{shownSnippet}</code>
          </pre>
        )}
        {snippet !== undefined && (
          <Button
            label={t(copyKey)}
            labelKey={copyKey}
            glyph={<IconCopy />}
            tone="subtle"
            onClick={() => onCopy(snippet)}
            className="self-start"
          />
        )}
        {setup.kind === "file" && where(setup.paths, setup.ui, setup.paste)}
        {keyLine()}
        {snippetBase.selfSigned && client.cert.kind !== "none" && certLine()}
      </Step>
    );
  }

  function where(paths: ConfigPaths, ui?: string, paste?: string) {
    return (
      <div className="flex flex-col gap-1 pt-1">
        <span className="text-xs text-carbon-textSub">{t("mcp.setupWhere")}</span>
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
          {OS_PATHS.filter(([os]) => paths[os]).map(([os, label]) => (
            <div key={os} className="contents">
              <dt className="text-carbon-textMuted">{label}</dt>
              <dd dir="ltr" className="min-w-0 break-all text-start font-mono text-carbon-text">
                {paths[os]}
              </dd>
            </div>
          ))}
        </dl>
        {ui && <p className="text-sm text-carbon-textSub">{fill("mcp.setupFileUi", { app: name, ui })}</p>}
        {paste && <p className="text-sm text-carbon-textSub">{fill("mcp.setupPaste", { app: name, ui: paste })}</p>}
      </div>
    );
  }

  function keyLine() {
    const key = client.key;
    const app = { app: name };
    let line: ReactNode;
    let tip: string | undefined;
    switch (key.kind) {
      case "keyFile":
        line = fill("mcp.keyFileLine", app);
        if (client.setup.kind === "file") tip = fill("mcp.keyFileJsonTip", app);
        break;
      case "env":
        line = [...rich("mcp.keyEnv", { ...app, var: KEY_VARIABLE }), key.unsure ? ` ${fill("mcp.keyEnvUnsure", app)}` : ""];
        tip = fill("mcp.keyEnvTip", { ...app, var: KEY_VARIABLE });
        break;
      case "prompt":
        line = fill("mcp.keyPrompt", app);
        break;
      case "dotenv":
        line = rich("mcp.keyDotenv", { ...app, var: KEY_VARIABLE, file: key.file });
        break;
      case "inFile":
        line = fill("mcp.keyInFile", app);
        break;
      case "inApp":
        line = fill("mcp.keyInApp", app);
        break;
      case "credential":
        line = fill("mcp.keyCredential", app);
        break;
      case "vendor":
        line = fill("mcp.keyVendor", app);
        break;
      default:
        line = t("mcp.keyOther");
    }
    return (
      <p className="pt-1 text-sm text-carbon-textSub">{withTip(line, tip)}</p>
    );
  }

  function certLine() {
    const cert = client.cert;
    const app = { app: name };
    let line: ReactNode;
    let tip: string | undefined;
    switch (cert.kind) {
      case "placeholder":
        line = t("mcp.certPlaceholder");
        break;
      case "env":
        line = rich("mcp.certEnv", { ...app, var: cert.variable });
        tip = fill("mcp.certEnvTip", { var: cert.variable });
        break;
      case "system":
        line = fill("mcp.certSystem", app);
        tip = t("mcp.certSystemTip");
        break;
      case "ownList":
        line = fill("mcp.certOwnList", app);
        break;
      default:
        line = t("mcp.certOther");
        tip = t("mcp.certOtherTip");
    }
    return (
      <div className="flex flex-col gap-2 pt-1">
        <p className="text-sm text-carbon-textSub">{withTip(line, tip)}</p>
        <Button
          label={t("mcp.certDownload")}
          labelKey="mcp.certDownload"
          glyph={<IconDownload />}
          tone="subtle"
          onClick={onDownloadCertificate}
          className="self-start"
        />
      </div>
    );
  }

  function waitStep() {
    const app = { app: name };
    let content;
    if (connected && chosen) {
      const values = {
        name: chosen.label,
        time: new Date(chosen.lastUsedAt * 1000).toLocaleTimeString(),
        addr: chosen.lastUsedFrom,
      };
      content = (
        <p role="status" className="flex items-start gap-2 text-sm text-statusOk">
          <span className="mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full bg-statusOkSolid" />
          {fill(chosen.lastUsedFrom ? "mcp.firstCall" : "mcp.firstCallNoAddr", values)}
        </p>
      );
    } else {
      content = (
        <>
          <p role="status" className="flex items-start gap-2 text-sm text-carbon-text">
            <span className="glim-wait-dot mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full bg-accent" />
            {other ? t("mcp.waitLineOther") : fill("mcp.waitLine", app)}
          </p>
          <p className="text-xs text-carbon-textMuted">
            {other ? t("mcp.waitHintOther") : fill("mcp.waitHint", app)}
          </p>
        </>
      );
    }
    return (
      <Step n={3} title={connected ? t("mcp.connectedTitle") : t("mcp.waitTitle")} done={connected}>
        {content}
      </Step>
    );
  }

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) closeRef.current();
      }}
    >
      <div
        ref={cardRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="bv-mcp-dialog-title"
        className="glim-modal-card relative flex max-h-[88vh] w-full max-w-2xl flex-col rounded-card bg-carbon-surface shadow-2xl [--mark-ground:var(--carbon-surface)]"
      >
        <div className="flex items-start px-5 py-4">
          <h2 id="bv-mcp-dialog-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>
              {other ? t("mcp.snippetsLabel") : fill("mcp.dialogTitle", { name })}
            </Badge>
          </h2>
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-5 pt-1 pb-4">{body}</div>
        <div className="flex items-center justify-end gap-3 px-5 py-4">
          <Button
            label={connected ? t("common.done") : t("common.close")}
            labelKey={connected ? "common.done" : "common.close"}
            glyph={connected ? <IconCheckCircle /> : undefined}
            tone="accent"
            onClick={() => closeRef.current()}
          />
        </div>
      </div>
    </div>,
    document.body
  );
}

/** One numbered step: its number turns into a check once it is done. */
function Step({ n, title, done = false, children }: { n: number; title: string; done?: boolean; children: React.ReactNode }) {
  return (
    <section className="grid grid-cols-[1.5rem_1fr] gap-x-3 gap-y-2.5">
      <span
        className={`inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold ${
          done ? "bg-statusOkBg text-statusOk" : "bg-carbon-surface2 text-carbon-textSub"
        }`}
      >
        {done ? <IconCheckCircle /> : n}
      </span>
      <h3 className="self-center text-sm font-semibold text-carbon-text">{title}</h3>
      <div className="col-start-2 flex min-w-0 flex-col gap-2.5">{children}</div>
    </section>
  );
}
