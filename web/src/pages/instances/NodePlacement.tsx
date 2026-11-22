// Where a node's backups go: this instance's receiving server, which the node
// can only add to, a storage location of this instance that the node reaches
// with a sign-in of its own, or the node itself, which this instance fetches
// them from. The backups go directly; the relay carries only the requests.

import { useEffect, useState } from "react";
import {
  getCloudCredSets,
  getReceiverServer,
  listDestinations,
  placeNode,
  setReceiverNodeAddress,
  ApiError,
  type CloudCredSetInfo,
  type Destination,
  type NodePlacement,
  type NodePlaceKind,
  type NodePlaceResult,
  type NodeReach,
  type NodeView,
  type OffsiteDomain,
  type ReceiverServer,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { nodeAddressProblem } from "../../lib/nodeAddress";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { Button } from "../../components/Button";
import { Choice, Choices } from "../../components/entry/Choice";
import { SelectField } from "../../components/SelectField";
import { Rows, SettingRow } from "../../components/storage/rows";
import { useToast } from "../../lib/toast";
import { InstanceWindow } from "./InstanceWindow";
import { PlacementPicture } from "./placementArt";

type T = ReturnType<typeof useT>["t"];

const PLACE_KINDS = [
  { kind: "receiver", title: "nodes.toReceiver", text: "nodes.toReceiverText" },
  { kind: "location", title: "nodes.toLocation", text: "nodes.toLocationText" },
  { kind: "node", title: "nodes.toNode", text: "nodes.toNodeText" },
] as const;

const INPUT = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** What a person picked for a node's backups, before it is sent. */
export interface PlacementChoice {
  kind: NodePlaceKind;
  /** The receiving server's address for nodes, as typed. */
  address: string;
  /** A certificate in PEM form, empty to send none. */
  certificate: string;
  /** Where this instance reaches a node that keeps its own backups. */
  nodeAddress: string;
  location: string;
  signIn: string;
}

/** What the choice is made from. */
export interface PlacementSources {
  receiver: ReceiverServer | null;
  /** The storage locations a node can reach: S3 services and REST servers. */
  locations: Destination[];
  signIns: CloudCredSetInfo[];
}

export function usePlacementSources(): PlacementSources | null {
  const [sources, setSources] = useState<PlacementSources | null>(null);
  useEffect(() => {
    let alive = true;
    Promise.all([
      getReceiverServer().catch(() => null),
      listDestinations().catch(() => null),
      getCloudCredSets().catch(() => null),
    ]).then(([receiver, locations, signIns]) => {
      if (!alive) return;
      setSources({
        receiver: receiver?.server ?? null,
        locations: (locations?.destinations ?? []).filter((d) => /^(rest|s3):/.test(d.repo)),
        signIns: signIns?.sets ?? [],
      });
    });
    return () => {
      alive = false;
    };
  }, []);
  return sources;
}

export function firstChoice(sources: PlacementSources, node?: NodeView): PlacementChoice {
  const location = sources.locations.find((l) => l.name === node?.placementTarget) ?? sources.locations[0];
  const kind = node?.placement ?? (sources.receiver ? "receiver" : location ? "location" : "node");
  return {
    kind,
    address: sources.receiver?.nodeUrl ?? "",
    certificate: "",
    nodeAddress: node?.placement === "node" ? (node.placementTarget ?? "") : "",
    location: location?.id ?? "",
    signIn: "",
  };
}

/** The saved sign-ins a node may use at a location: any but the one this
 *  instance writes there with, which for a location naming none is the default. */
export function signInsFor(location: Destination | undefined, signIns: CloudCredSetInfo[]): CloudCredSetInfo[] {
  if (!location) return [];
  return signIns.filter((s) => s.id !== location.credsRef && !(location.credsRef === "" && s.default));
}

export function placementReady(choice: PlacementChoice, sources: PlacementSources): boolean {
  if (choice.kind === "receiver") {
    return sources.receiver !== null && choice.address.trim() !== "" && nodeAddressProblem(choice.address) === null;
  }
  if (choice.kind === "node") return choice.nodeAddress.trim() !== "" && nodeAddressProblem(choice.nodeAddress, true) === null;
  const location = sources.locations.find((l) => l.id === choice.location);
  return signInsFor(location, sources.signIns).some((s) => s.id === choice.signIn);
}

export function placementOf(choice: PlacementChoice): NodePlacement {
  if (choice.kind === "node") return { kind: "node", address: bareAddress(choice.nodeAddress) };
  return choice.kind === "receiver" ? { kind: "receiver" } : { kind: "location", location: choice.location, signIn: choice.signIn };
}

const bareAddress = (typed: string) => typed.trim().replace(/\/+$/, "");

/** The name the card shows for where the backups go. */
export function placementTarget(choice: PlacementChoice, sources: PlacementSources): string {
  if (choice.kind === "receiver") return choice.address.trim();
  if (choice.kind === "node") return bareAddress(choice.nodeAddress);
  return sources.locations.find((l) => l.id === choice.location)?.name ?? "";
}

/**
 * saveAddress stores the receiving server's address for nodes when the choice
 * changed it, and returns what is wrong with it, or null. A certificate set
 * earlier is only known to exist, so it goes when the address is saved
 * without a new one.
 */
export async function saveAddress(choice: PlacementChoice, sources: PlacementSources, t: T): Promise<string | null> {
  if (choice.kind !== "receiver") return null;
  const address = choice.address.trim();
  if (address === (sources.receiver?.nodeUrl ?? "") && choice.certificate.trim() === "") return null;
  try {
    const res = await setReceiverNodeAddress(address, choice.certificate);
    return res.ok ? null : (res.error ?? t("settings.error"));
  } catch (err) {
    if (err instanceof ApiError) return err.detail ?? err.message;
    return t("settings.error");
  }
}

/** placedSentence says where the backups go, or will go once the node is
 *  online. */
export function placedSentence(t: T, choice: PlacementChoice, target: string, later = false): string {
  const onNode = choice.kind === "node";
  const key = later ? (onNode ? "nodes.placementLaterOnNode" : "nodes.placementLater") : onNode ? "nodes.placedOnNode" : "nodes.placed";
  return t(key).replace("{target}", () => target);
}

/** placeError says why a placement failed, in the reader's language where the
 *  server named the reason. */
export function placeError(t: T, res: NodePlaceResult): string {
  const key = res.code === "node-rest-unreachable" ? "nodes.restUnreachable" : res.code === "node-rest-foreign" ? "nodes.restForeign" : null;
  if (!key) return res.error ?? t("settings.error");
  return t(key)
    .replace("{address}", () => res.address ?? "")
    .replace("{port}", () => res.port ?? "")
    .replace("{seconds}", () => String(res.seconds ?? ""));
}

/** The sentences for the repositories a node could not reach. */
export function reachFailures(t: T, reach: NodeReach[] = []): string[] {
  return reach
    .filter((r) => !r.reached)
    .map((r) =>
      t("nodes.reachFailed")
        .replace("{domain}", () => (r.domain in DOMAIN_LABEL ? t(DOMAIN_LABEL[r.domain as OffsiteDomain]) : r.domain))
        .replace("{detail}", () => r.detail ?? ""),
    );
}

export function PlacementFields({
  choice,
  sources,
  onChange,
}: {
  choice: PlacementChoice;
  sources: PlacementSources;
  onChange: (next: PlacementChoice) => void;
}) {
  const { t } = useT();
  const set = (patch: Partial<PlacementChoice>) => onChange({ ...choice, ...patch });
  const problem = nodeAddressProblem(choice.address);
  const nodeProblem = nodeAddressProblem(choice.nodeAddress, true);
  const location = sources.locations.find((l) => l.id === choice.location);
  const signIns = signInsFor(location, sources.signIns);

  return (
    <>
      <p className="text-sm text-carbon-textSub">{t("nodes.placementIntro")}</p>
      <div data-new="node-placement">
        <Choices label={t("nodes.step.placement")}>
          {PLACE_KINDS.map(({ kind, title, text }) => (
            <Choice
              key={kind}
              on={choice.kind === kind}
              title={t(title)}
              text={t(text)}
              picture={<PlacementPicture kind={kind} on={choice.kind === kind} />}
              onPick={() => set({ kind })}
            />
          ))}
        </Choices>
      </div>

      {choice.kind === "receiver" &&
        (sources.receiver ? (
          <div className="flex flex-col gap-2">
            <Rows plain>
              <SettingRow label={t("nodes.address")} hint={t("nodes.addressTip")} htmlFor="bv-node-address">
                <input
                  id="bv-node-address"
                  type="url"
                  value={choice.address}
                  onChange={(e) => set({ address: e.target.value })}
                  placeholder="https://backup.example.org"
                  spellCheck={false}
                  autoComplete="off"
                  dir="ltr"
                  aria-invalid={problem !== null}
                  className={`${INPUT} w-72 max-w-full font-mono text-start`}
                />
              </SettingRow>
              <SettingRow
                label={t("nodes.certificate")}
                hint={t("nodes.certificateTip")}
                htmlFor="bv-node-certificate"
                note={sources.receiver.nodeCa ? t("nodes.certificateSet") : undefined}
              >
                <textarea
                  id="bv-node-certificate"
                  value={choice.certificate}
                  onChange={(e) => set({ certificate: e.target.value })}
                  placeholder="-----BEGIN CERTIFICATE-----"
                  rows={3}
                  spellCheck={false}
                  dir="ltr"
                  className={`${INPUT} w-72 max-w-full resize-y font-mono text-xs text-start`}
                />
              </SettingRow>
            </Rows>
            {problem && (
              <p role="alert" className="text-xs text-statusWarn">
                {t(problem === "plain" ? "nodes.addressPlain" : "nodes.addressForm")}
              </p>
            )}
          </div>
        ) : (
          <p className="text-sm text-statusWarn">{t("nodes.noReceiver")}</p>
        ))}

      {choice.kind === "node" && (
        <div className="flex flex-col gap-2">
          <Rows plain>
            <SettingRow label={t("nodes.nodeAddress")} hint={t("nodes.nodeAddressTip")} htmlFor="bv-node-own-address">
              <input
                id="bv-node-own-address"
                type="url"
                value={choice.nodeAddress}
                onChange={(e) => set({ nodeAddress: e.target.value })}
                placeholder="https://203.0.113.7:8000"
                spellCheck={false}
                autoComplete="off"
                dir="ltr"
                aria-invalid={nodeProblem !== null}
                className={`${INPUT} w-72 max-w-full font-mono text-start`}
              />
            </SettingRow>
          </Rows>
          <p className="text-xs text-carbon-textSub">{t("nodes.portNote")}</p>
          {nodeProblem && (
            <p role="alert" className="text-xs text-statusWarn">
              {t(nodeProblem === "plain" ? "nodes.fetchPlain" : "nodes.addressForm")}
            </p>
          )}
        </div>
      )}

      {choice.kind === "location" &&
        (sources.locations.length === 0 ? (
          <p className="text-sm text-statusWarn">{t("nodes.noLocation")}</p>
        ) : (
          <div className="flex flex-col gap-2">
            <Rows plain>
              <SettingRow label={t("nodes.fig.location")} htmlFor="bv-node-location">
                <SelectField
                  id="bv-node-location"
                  value={choice.location}
                  onChange={(id) => set({ location: id, signIn: "" })}
                  label={t("nodes.fig.location")}
                  options={sources.locations.map((l) => ({ value: l.id, label: l.name }))}
                  className={`${INPUT} w-72 max-w-full`}
                  wrap
                />
              </SettingRow>
              {signIns.length === 0 ? (
                <SettingRow label={t("nodes.signIn")} hint={t("nodes.signInTip")} note={t("nodes.noSignIn")} />
              ) : (
                <SettingRow label={t("nodes.signIn")} hint={t("nodes.signInTip")} htmlFor="bv-node-sign-in">
                  <SelectField
                    id="bv-node-sign-in"
                    value={choice.signIn}
                    onChange={(id) => set({ signIn: id })}
                    label={t("nodes.signIn")}
                    options={[{ value: "", label: t("nodes.choose") }, ...signIns.map((s) => ({ value: s.id, label: s.name }))]}
                    className={`${INPUT} w-72 max-w-full`}
                  />
                </SettingRow>
              )}
            </Rows>
            <p className="text-xs text-statusWarn">{t("nodes.locationWarning")}</p>
          </div>
        ))}
    </>
  );
}

/** PlacementWindow changes where an adopted node's backups go. */
export function PlacementWindow({ node, onClose, onSaved }: { node: NodeView; onClose: () => void; onSaved: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const sources = usePlacementSources();
  const [choice, setChoice] = useState<PlacementChoice | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notes, setNotes] = useState<string[]>([]);

  useEffect(() => {
    if (sources && !choice) setChoice(firstChoice(sources, node));
  }, [sources, choice, node]);

  async function save() {
    if (!choice || !sources) return;
    setSaving(true);
    setError(null);
    try {
      const refused = await saveAddress(choice, sources, t);
      if (refused) {
        setError(refused);
        return;
      }
      const res = await placeNode(node.id, placementOf(choice));
      if (!res.ok) {
        setError(placeError(t, res));
        return;
      }
      onSaved();
      const said = reachFailures(t, res.reach);
      if (said.length === 0) {
        push(placedSentence(t, choice, placementTarget(choice, sources)), "success");
        onClose();
        return;
      }
      setNotes(said);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setSaving(false);
    }
  }

  const done = notes.length > 0;
  return (
    <InstanceWindow
      title={t("nodes.editPlacement")}
      facts={node.name}
      onClose={onClose}
      footer={
        done ? (
          <Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onClose} />
        ) : (
          <>
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={saving} />
            <Button
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void save()}
              busy={saving}
              disabled={saving || !choice || !sources || !placementReady(choice, sources)}
            />
          </>
        )
      }
    >
      {sources && choice && !done && <PlacementFields choice={choice} sources={sources} onChange={setChoice} />}
      {!sources && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && (
        <p role="alert" className="text-sm text-statusFail wrap-break-word">
          {error}
        </p>
      )}
      {notes.map((note) => (
        <p key={note} className="text-sm text-statusWarn wrap-break-word">
          {note}
        </p>
      ))}
    </InstanceWindow>
  );
}
