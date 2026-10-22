import { useEffect, useId, useRef, useState } from "react";
import {
  destinationFolder,
  moveOwnFolders,
  previewOwnFolders,
  type CloudCredSetInfo,
  type OffsiteDomain,
  type OkEnvelope,
  type OwnFolderRequest,
  type OwnFolderStep,
  type StorageLocation,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { defaultSignIn, signInOption, useCloudCredSets } from "../../lib/useCloudCredSets";
import { storageLocationsChanged, useStorageLocations } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconForward } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconClose } from "../navGlyphs";
import { isRemotePath } from "../PathModeSwitch";
import { RadioMark } from "../RadioMark";
import { SelectField } from "../SelectField";
import { Toggle } from "../Toggle";
import { LocationMark } from "./LocationMark";
import { Rows, SettingRow } from "./rows";
import { SectionPathWindow } from "./SectionPathWindow";

const OTHER = "other";
/** The refusals that only copying the backups already made runs into. */
const COPY_REFUSALS = new Set(["move-sign-in", "own-folder-unread"]);
const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

type T = ReturnType<typeof useT>["t"];

/** ownFolderOf is the own-folder section a location has for a domain: a home
 *  that is neither a named repository nor a target's direct one. */
function ownFolderOf(location: StorageLocation, domain: OffsiteDomain) {
  return location.sections.find((s) => s.domain === domain && s.use === "home" && !s.repoId && !s.targetId);
}

/** The places an own folder can go: a folder on this host the storage pages
 *  list, or a storage location set up once. */
function choicesOf(locations: StorageLocation[]): StorageLocation[] {
  return locations.filter((l) => (l.object === "path" && l.kind === "local") || l.object === "destination");
}

function signInLine(t: T, location: StorageLocation, sets: CloudCredSetInfo[]): string | null {
  if (location.backend === "local") return t("own.onHost");
  if (location.backend !== "s3" && location.backend !== "rest") return null;
  const set = sets.find((s) => s.id === location.credsRef) ?? (location.credsRef ? undefined : defaultSignIn(sets));
  return set ? t("own.signsInWith").replace("{name}", () => signInOption(t, set)) : null;
}

/** What the switch does to each section, as the window says it. */
function whatHappens(t: T, lang: string, steps: OwnFolderStep[], copyOver: boolean, placeName: string, homeName: (d: OffsiteDomain) => string): string[] {
  const lines: string[] = [];
  for (const step of steps) {
    const section = t(DOMAIN_LABEL[step.domain]);
    const from = homeName(step.domain);
    if (step.same) {
      lines.push(t("own.lineSame").replace("{section}", () => section).replace("{name}", () => placeName));
      continue;
    }
    let line = t("own.lineTo").replace("{section}", () => section).replace("{path}", () => step.to);
    if (step.copyAt) {
      line += ` ${t("own.lineCopyThere").replace(/\{name\}/g, () => placeName).replace("{section}", () => section)}`;
      if (step.pending > 0) {
        line += ` ${(copyOver ? t("own.lineLacking", step.pending) : t("own.lineNewestStay", step.pending)).replace("{from}", () => from)}`;
      }
    } else if (copyOver && step.pending > 0) {
      line += ` ${t("own.lineCopyFirst", step.pending).replace("{from}", () => from)}`;
    } else if (!copyOver && step.backups > 0) {
      line += ` ${t("own.lineStay", step.backups).replace("{from}", () => from)}`;
    }
    const copies = step.copies ?? [];
    if (copies.length > 0) line += ` ${t("own.lineCopies").replace("{list}", () => formatList(lang, copies))}`;
    lines.push(line);
  }
  return lines;
}

/**
 * OwnFolderWindow moves the own folders of one or more sections, the place
 * their backups are written first. It offers the storage locations there
 * are, each with the sign-in it brings, and a typed address with a saved
 * sign-in. Before the switch it says what happens to each section.
 */
export function OwnFolderWindow({
  domains,
  target,
  onClose,
}: {
  /** The sections picked to move when the window opens. */
  domains: OffsiteDomain[];
  /** The location picked when the window opens. */
  target?: string;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const uid = useId();
  const { locations, loaded } = useStorageLocations();
  const sets = useCloudCredSets();
  const [picked, setPicked] = useState<OffsiteDomain[]>(domains);
  const [to, setTo] = useState<string | null>(target ?? null);
  const [address, setAddress] = useState("");
  const [credsRef, setCredsRef] = useState<string | null>(null);
  const [copyOver, setCopyOver] = useState(true);
  const [steps, setSteps] = useState<OwnFolderStep[] | null>(null);
  const [problem, setProblem] = useState<{ text: string; code?: string } | null>(null);
  const stepsFor = useRef<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [safety, setSafety] = useState(false);

  const homes = new Map<OffsiteDomain, StorageLocation>();
  for (const location of locations) {
    for (const section of location.sections) {
      if (ownFolderOf(location, section.domain)) homes.set(section.domain, location);
    }
  }
  const sections = [...homes.keys()];
  const choices = choicesOf(locations);
  const place = to && to !== OTHER ? choices.find((c) => c.id === to) : undefined;
  const remoteAddress = to === OTHER && isRemotePath(address);
  const signIn = credsRef ?? defaultSignIn(sets)?.id ?? "";
  const request: OwnFolderRequest | null =
    picked.length === 0 || !to || (to === OTHER && address.trim() === "")
      ? null
      : {
          domains: picked,
          copyOver,
          ...(place ? { location: place.id } : { address: address.trim(), credsRef: remoteAddress ? signIn : "" }),
        };
  const key = JSON.stringify(request);
  // The steps are the same with the copy on or off, so they stay up while the
  // toggle is checked again. Only the copy itself can be refused.
  const where = JSON.stringify(request && { ...request, copyOver: undefined });

  useEffect(() => {
    if (stepsFor.current !== where) setSteps(null);
    setProblem(null);
    if (!request) return;
    let alive = true;
    const timer = setTimeout(() => {
      void previewOwnFolders(request)
        .catch((e: unknown): OkEnvelope => ({ ok: false, error: e instanceof Error ? e.message : undefined }))
        .then((res: OkEnvelope & { steps?: OwnFolderStep[] }) => {
          if (!alive) return;
          if (res.ok && res.steps) {
            stepsFor.current = where;
            setSteps(res.steps);
          } else {
            stepsFor.current = null;
            setSteps(null);
            setProblem({ text: placementErrorText(t, lang, res, "settings.error"), code: res.code });
          }
        });
    }, 400);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  if (safety && picked.length === 1) return <SectionPathWindow domain={picked[0]} onClose={onClose} />;

  const placeName = place ? place.name : address.trim();
  const homeName = (domain: OffsiteDomain) => homes.get(domain)?.name ?? "";
  const changes = steps?.some((step) => !step.same) ?? false;
  const lines = steps ? whatHappens(t, lang, steps, copyOver, placeName, homeName) : [];
  if (steps && changes && place && place.backend !== "local") {
    lines.push(t("own.lineProtection").replace("{name}", () => place.name));
  }

  const title =
    target && place
      ? t("own.titleTo").replace("{name}", () => place.name)
      : domains.length === 1
        ? t("own.titleOne").replace("{section}", () => t(DOMAIN_LABEL[domains[0]]))
        : t("own.titleMany");

  function toggle(domain: OffsiteDomain, on: boolean) {
    setPicked((list) => (on ? sections.filter((d) => d === domain || list.includes(d)) : list.filter((d) => d !== domain)));
  }

  async function go() {
    if (!request) return;
    setBusy(true);
    const res = await moveOwnFolders(request).catch((e: unknown): OkEnvelope & { steps?: OwnFolderStep[] } => ({
      ok: false,
      error: e instanceof Error ? e.message : undefined,
    }));
    setBusy(false);
    if (!res.ok || !res.steps) {
      setProblem({ text: placementErrorText(t, lang, res, "settings.error"), code: res.code });
      return;
    }
    const names = (list: OwnFolderStep[]) => formatList(lang, list.map((step) => t(DOMAIN_LABEL[step.domain])));
    const later = res.steps.filter((step) => !step.same && copyOver && step.pending > 0);
    const now = res.steps.filter((step) => !step.same && !later.includes(step));
    if (now.length > 0) push(t("own.moved").replace("{name}", () => placeName).replace("{list}", () => names(now)), "success");
    if (later.length > 0) push(t("own.copying").replace("{name}", () => placeName).replace("{list}", () => names(later)), "success");
    storageLocationsChanged();
    onClose();
  }

  return (
    <InstanceWindow
      title={title}
      hint={t("own.hint")}
      facts={t("own.facts")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />
          <Button
            label={t("own.switch")}
            labelKey="own.switch"
            glyph={<IconForward />}
            tone="accent"
            busy={busy}
            disabled={busy || !changes}
            onClick={() => void go()}
          />
        </>
      }
    >
      {!loaded ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : (
        <div className="flex flex-col gap-4">
          <section className="flex flex-col gap-2">
            <h3 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-widest text-carbon-textSub">
              {t("own.sections")}
              <InfoBubble tip={t("own.sectionsHint")} />
            </h3>
            <Rows>
              {sections.map((domain) => (
                <SettingRow key={domain} label={t(DOMAIN_LABEL[domain])} note={t("own.nowIn").replace("{name}", () => homeName(domain))}>
                  <Toggle hideLabel label={t(DOMAIN_LABEL[domain])} checked={picked.includes(domain)} disabled={busy} onChange={(on) => toggle(domain, on)} />
                </SettingRow>
              ))}
            </Rows>
          </section>

          <section className="flex flex-col gap-2">
            <h3 className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">{t("own.moveTo")}</h3>
            <div role="radiogroup" aria-label={t("own.moveTo")} className="flex flex-col gap-1.5">
              {choices.map((choice) => {
                const on = choice.id === to;
                const now = picked.length > 0 && picked.every((d) => homes.get(d)?.id === choice.id);
                const holds = picked.filter((d) => choice.sections.some((s) => s.domain === d && s.use === "copy"));
                const one = picked.length === 1 ? picked[0] : null;
                const where = one ? (ownFolderOf(choice, one)?.where ?? destinationFolder({ repo: choice.where }, one)) : choice.where;
                const line = signInLine(t, choice, sets);
                return (
                  <button
                    key={choice.id}
                    type="button"
                    role="radio"
                    aria-checked={on}
                    aria-label={choice.name}
                    disabled={busy}
                    onClick={() => setTo(choice.id)}
                    className={`flex items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2.5 text-start hover:bg-carbon-surface3 ${on ? "ring-2 ring-accent" : ""}`}
                  >
                    <RadioMark on={on} />
                    <LocationMark location={choice} />
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="flex flex-wrap items-center gap-2 text-sm font-semibold text-carbon-text">
                        <span className="min-w-0 truncate">{choice.name}</span>
                        {now && <Badge tone="active" size="small">{t("own.now")}</Badge>}
                        {holds.length > 0 && (
                          <Badge tone="neutral" size="small">
                            {t("own.holdsCopy").replace("{list}", () => formatList(lang, holds.map((d) => t(DOMAIN_LABEL[d]))))}
                          </Badge>
                        )}
                      </span>
                      <span dir="ltr" className="truncate text-start font-mono text-xs text-carbon-textMuted">
                        {where}
                      </span>
                      {line && <span className="text-xs text-carbon-textSub">{line}</span>}
                    </span>
                  </button>
                );
              })}
              <button
                type="button"
                role="radio"
                aria-checked={to === OTHER}
                aria-label={t("own.other")}
                disabled={busy}
                onClick={() => setTo(OTHER)}
                className={`flex items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2.5 text-start hover:bg-carbon-surface3 ${to === OTHER ? "ring-2 ring-accent" : ""}`}
              >
                <RadioMark on={to === OTHER} />
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-sm font-semibold text-carbon-text">{t("own.other")}</span>
                  <span className="text-xs text-carbon-textSub">{t("own.otherSub")}</span>
                </span>
              </button>
            </div>
            {to === OTHER && (
              <Rows>
                <SettingRow label={t("own.address")} hint={t("own.addressHint")} htmlFor={`${uid}-address`}>
                  <input
                    id={`${uid}-address`}
                    value={address}
                    onChange={(e) => setAddress(e.target.value)}
                    placeholder="s3:host/bucket/folder"
                    spellCheck={false}
                    dir="ltr"
                    className={`w-72 max-w-full font-mono text-start ${FIELD}`}
                  />
                </SettingRow>
                {remoteAddress && (
                  <SettingRow label={t("storage.credentials")} hint={t("own.signInHint")}>
                    {sets.length > 0 ? (
                      <SelectField
                        value={signIn}
                        onChange={setCredsRef}
                        label={t("storage.credentials")}
                        options={sets.map((s) => ({ value: s.id, label: signInOption(t, s) }))}
                        className={FIELD}
                      />
                    ) : (
                      <span className="text-sm text-carbon-textSub">{t("signIns.none")}</span>
                    )}
                  </SettingRow>
                )}
                {picked.length === 1 && (
                  <SettingRow label={t("own.safety")} hint={t("own.safetyHint")}>
                    <Button label={t("own.open")} labelKey="own.open" glyph={<IconForward />} tone="neutral" onClick={() => setSafety(true)} />
                  </SettingRow>
                )}
              </Rows>
            )}
          </section>

          <section className="flex flex-col gap-2">
            <h3 className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">{t("own.what")}</h3>
            {problem ? (
              <div role="alert" className="flex flex-col gap-1 rounded-card bg-statusWarnBg px-3 py-2 text-sm leading-relaxed text-statusWarn wrap-break-word">
                <p className="font-semibold">{t("own.blocked")}</p>
                <p>{problem.text}</p>
                {COPY_REFUSALS.has(problem.code ?? "") && copyOver && <p>{t("own.blockedByCopy").replace("{toggle}", () => t("own.copyOver"))}</p>}
              </div>
            ) : !request ? (
              <p className="text-sm text-carbon-textSub">{t("own.pick")}</p>
            ) : steps ? (
              <ul className="flex list-disc flex-col gap-1.5 ps-5 text-sm text-carbon-text">
                {lines.map((line) => (
                  <li key={line} className="wrap-break-word">
                    {line}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
            )}
            <SettingRow label={t("own.copyOver")} hint={t("own.copyOverHint")}>
              <Toggle hideLabel label={t("own.copyOver")} checked={copyOver} disabled={busy} onChange={setCopyOver} />
            </SettingRow>
          </section>
        </div>
      )}
    </InstanceWindow>
  );
}
