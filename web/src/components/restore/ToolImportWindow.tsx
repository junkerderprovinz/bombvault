import { useRef, useState } from "react";
import {
  applyToolImport,
  previewToolImport,
  type ToolImportChoice,
  type ToolImportInput,
  type ToolImportJob,
  type ToolImportNote,
  type ToolImportResult,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { SubCard } from "../../pages/settings/shared";
import { Button } from "../Button";
import { formatCadence } from "../CadenceBuilder";
import { Checkbox } from "../Checkbox";
import { FolderBrowser } from "../FolderBrowser";
import { IconBack, IconConfirm, IconImport, IconSearch } from "../glyphs";
import { RadioMark } from "../RadioMark";
import { Rows, SettingRow } from "../storage/rows";
import { StepBar } from "./StepBar";

const FIELD = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus w-72 max-w-full";

const NOTE: Record<string, TranslationKey> = {
  "encryption-key": "toolImport.note.encryptionKey",
  retention: "toolImport.note.retention",
  filter: "toolImport.note.filter",
  include: "toolImport.note.include",
  options: "toolImport.note.options",
  schedule: "toolImport.note.schedule",
  "no-schedule": "toolImport.note.noSchedule",
  destination: "toolImport.note.destination",
  "ssh-key": "toolImport.note.sshKey",
  "rclone-remote": "toolImport.note.rcloneRemote",
  disabled: "toolImport.note.disabled",
  "sign-in-encrypted": "toolImport.note.signInEncrypted",
};

/** What the person decided for one job in the preview. */
interface Draft {
  sets: Record<string, { on: boolean; path: string }>;
  useLocation: boolean;
  location: string;
  endpoint: string;
  keyId: string;
  secret: string;
}

function draftFor(job: ToolImportJob): Draft {
  const sets: Draft["sets"] = {};
  for (const s of job.sets) sets[s.key] = { on: s.path !== "" && !s.taken, path: s.path };
  return { sets, useLocation: job.dest.kind !== "", location: job.dest.location, endpoint: "", keyId: "", secret: "" };
}

function locationOf(job: ToolImportJob, d: Draft): string {
  if (job.dest.kind !== "b2") return d.location.trim();
  const host = d.endpoint.trim().replace(/^https?:\/\//, "").replace(/\/+$/, "");
  return host ? `s3:https://${host}/${job.dest.b2Path ?? ""}` : "";
}

function needsSignIn(job: ToolImportJob, d: Draft): boolean {
  return d.useLocation && job.dest.signIn === "needed" && !job.dest.existing && locationOf(job, d).startsWith("s3:");
}

/** The request for one job, or null when none of its sets is picked. */
function choiceFor(job: ToolImportJob, d: Draft): ToolImportChoice | null {
  const sets = job.sets
    .filter((s) => d.sets[s.key].on)
    .map((s) => (d.sets[s.key].path === s.path ? { key: s.key } : { key: s.key, path: d.sets[s.key].path }));
  if (sets.length === 0) return null;
  const choice: ToolImportChoice = { key: job.key, sets };
  if (!d.useLocation) {
    if (job.dest.kind !== "") choice.location = "";
    return choice;
  }
  const location = locationOf(job, d);
  if (location !== job.dest.location) choice.location = location;
  if (needsSignIn(job, d)) {
    choice.keyId = d.keyId.trim();
    choice.secret = d.secret.trim();
  }
  return choice;
}

function ready(job: ToolImportJob, d: Draft): boolean {
  if (job.sets.some((s) => d.sets[s.key].on && d.sets[s.key].path === "")) return false;
  if (!d.useLocation) return true;
  if (locationOf(job, d) === "") return false;
  return !needsSignIn(job, d) || (d.keyId.trim() !== "" && d.secret.trim() !== "");
}

/**
 * ToolImportWindow turns the backup jobs of Duplicati and Duplicacy into
 * folder sets. It reads their settings only: the old backups stay where they
 * are, and nothing is created before the person has seen the preview.
 */
export function ToolImportWindow({ hostMountRoot, onClose }: { hostMountRoot: string; onClose: () => void }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [tool, setTool] = useState<"duplicati" | "duplicacy">("duplicati");
  const [files, setFiles] = useState<{ name: string; text: string }[]>([]);
  const [folder, setFolder] = useState("");
  const [input, setInput] = useState<ToolImportInput | null>(null);
  const [jobs, setJobs] = useState<ToolImportJob[] | null>(null);
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [results, setResults] = useState<Record<string, ToolImportResult> | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement | null>(null);

  const source: ToolImportInput | null =
    tool === "duplicati"
      ? files.length > 0
        ? { tool, files }
        : null
      : folder.trim() !== ""
        ? { tool, path: folder.trim() }
        : null;

  async function pickFiles(list: FileList | null) {
    const picked = await Promise.all([...(list ?? [])].map(async (f) => ({ name: f.name, text: await f.text() })));
    setFiles(picked);
  }

  async function read() {
    if (!source) return;
    setBusy(true);
    try {
      const r = await previewToolImport(source);
      if (r.ok && r.jobs) {
        setInput(source);
        setJobs(r.jobs);
        setDrafts(Object.fromEntries(r.jobs.map((j) => [j.key, draftFor(j)])));
        setResults(null);
      } else push(r.error ?? t("toolImport.readFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("toolImport.readFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  function edit(key: string, change: (d: Draft) => Draft) {
    setDrafts((prev) => ({ ...prev, [key]: change(prev[key]) }));
  }

  const choices = (jobs ?? []).flatMap((j) => {
    const c = choiceFor(j, drafts[j.key]);
    return c ? [{ job: j, choice: c }] : [];
  });
  const canImport = choices.length > 0 && choices.every(({ job }) => ready(job, drafts[job.key]));

  async function startImport() {
    if (!input) return;
    setBusy(true);
    try {
      const r = await applyToolImport(input, choices.map((c) => c.choice));
      if (r.ok && r.jobs) {
        setResults(Object.fromEntries(r.jobs.map((res) => [res.key, res])));
        if (r.jobs.some((res) => res.sets.length > 0)) placementChanged();
      } else push(r.error ?? t("toolImport.importFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("toolImport.importFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  const noteText = (n: ToolImportNote) => {
    if (n.code === "destination" && !n.detail) return t("toolImport.note.destinationUnknown");
    const key = NOTE[n.code];
    return key ? t(key).replace("{detail}", () => n.detail ?? "") : n.detail ?? n.code;
  };

  const listed = jobs !== null;

  return (
    <InstanceWindow
      title={t("toolImport.title")}
      hint={t("toolImport.intro")}
      onClose={onClose}
      footer={
        !listed ? (
          <>
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
            <Button
              label={t("toolImport.read")}
              labelKey="toolImport.read"
              glyph={<IconSearch />}
              tone="accent"
              onClick={() => void read()}
              disabled={!source || busy}
              busy={busy}
            />
          </>
        ) : results ? (
          <Button label={t("common.done")} labelKey="common.done" glyph={<IconConfirm />} tone="accent" onClick={onClose} />
        ) : (
          <>
            <Button
              label={t("common.back")}
              labelKey="common.back"
              glyph={<IconBack />}
              tone="subtle"
              onClick={() => setJobs(null)}
              disabled={busy}
            />
            <Button
              label={t("toolImport.import")}
              labelKey="toolImport.import"
              glyph={<IconImport />}
              tone="accent"
              onClick={() => void startImport()}
              disabled={!canImport || busy}
              busy={busy}
            />
          </>
        )
      }
    >
      <StepBar
        steps={[t("toolImport.stepSource"), t("toolImport.stepCheck")]}
        current={listed ? 2 : 1}
        reach={listed && !results ? 2 : listed ? 0 : 1}
        label={t("toolImport.title")}
        onGo={(n) => {
          if (n === 1) setJobs(null);
        }}
      />

      {!listed && (
        <>
          <div role="radiogroup" aria-label={t("toolImport.stepSource")} className="flex flex-col gap-2">
            <ToolChoice
              on={tool === "duplicati"}
              title={t("toolImport.fromDuplicati")}
              sub={t("toolImport.fromDuplicatiSub")}
              onPick={() => setTool("duplicati")}
            />
            <ToolChoice
              on={tool === "duplicacy"}
              title={t("toolImport.fromDuplicacy")}
              sub={t("toolImport.fromDuplicacySub")}
              onPick={() => setTool("duplicacy")}
            />
          </div>
          {tool === "duplicati" ? (
            <div className="flex flex-wrap items-center gap-3">
              <input
                ref={fileRef}
                type="file"
                multiple
                accept="application/json,.json"
                className="hidden"
                onChange={(e) => void pickFiles(e.target.files)}
              />
              <Button
                label={t("toolImport.chooseFiles")}
                labelKey="toolImport.chooseFiles"
                glyph={<IconImport />}
                tone="neutral"
                onClick={() => fileRef.current?.click()}
              />
              <span className="min-w-0 text-sm text-carbon-textMuted wrap-anywhere">
                {files.length === 0 ? t("toolImport.noFiles") : <bdi dir="ltr">{files.map((f) => f.name).join(", ")}</bdi>}
              </span>
            </div>
          ) : (
            <FolderBrowser
              label={t("toolImport.folder")}
              value={folder}
              hostMountRoot={hostMountRoot}
              onChange={setFolder}
              hint={t("toolImport.folderHint")}
            />
          )}
        </>
      )}

      {listed && jobs.length === 0 && <p className="text-sm text-carbon-text">{t("toolImport.nothing")}</p>}

      {listed &&
        jobs.map((job) => {
          const d = drafts[job.key];
          const result = results?.[job.key];
          return (
            <SubCard key={job.key} title={job.name}>
              <Rows>
                {job.sets.map((s) => (
                  <SettingRow
                    key={s.key}
                    label={s.name}
                    name={
                      <Checkbox
                        label={s.name}
                        checked={d.sets[s.key].on}
                        disabled={results !== null}
                        onChange={(on) => edit(job.key, (x) => ({ ...x, sets: { ...x.sets, [s.key]: { ...x.sets[s.key], on } } }))}
                      />
                    }
                    note={
                      <>
                        <bdi dir="ltr" className="font-mono">
                          {s.source}
                          {s.path && ` → ${s.path}`}
                        </bdi>
                        {s.taken && <span>{t("toolImport.taken").replace("{name}", () => s.taken ?? "")}</span>}
                        <span>
                          {s.excludes.length > 0
                            ? t("toolImport.excludes").replace("{list}", () => s.excludes.join(", "))
                            : t("toolImport.noExcludes")}
                        </span>
                      </>
                    }
                  >
                    {s.path === "" && !results && (
                      <FolderBrowser
                        label={t("toolImport.pickFolder").replace("{name}", () => s.name)}
                        value={d.sets[s.key].path}
                        hostMountRoot={hostMountRoot}
                        onChange={(path) =>
                          edit(job.key, (x) => ({ ...x, sets: { ...x.sets, [s.key]: { on: path.trim() !== "", path: path.trim() } } }))
                        }
                      />
                    )}
                  </SettingRow>
                ))}
                <SettingRow label={t("toolImport.scheduleLabel")}>
                  <span className="text-end text-sm text-carbon-text">
                    {formatCadence(job.schedule, t, lang) || t("toolImport.followsFolders")}
                  </span>
                </SettingRow>
                <DestinationRows job={job} draft={d} locked={results !== null} hostMountRoot={hostMountRoot} onEdit={(c) => edit(job.key, c)} />
              </Rows>
              {job.notes.length > 0 && (
                <ul className="mt-3 flex list-disc flex-col gap-1 ps-5 text-xs text-carbon-textMuted">
                  {job.notes.map((n, i) => (
                    <li key={i} className="wrap-anywhere">
                      {noteText(n)}
                    </li>
                  ))}
                </ul>
              )}
              {result && (
                <div className="mt-3 flex flex-col gap-1 text-sm">
                  {result.sets.length > 0 && (
                    <p className="flex flex-wrap gap-x-1 rounded-card bg-statusOkBg px-3 py-2 text-statusOk">
                      <span>{t("toolImport.created").replace("{list}", () => result.sets.join(", "))}</span>
                      {result.location && <span>{t("toolImport.createdAt").replace("{location}", () => result.location)}</span>}
                    </p>
                  )}
                  {result.error && <p className="rounded-card bg-statusFailBgSoft px-3 py-2 text-statusFail wrap-anywhere">{result.error}</p>}
                </div>
              )}
            </SubCard>
          );
        })}
    </InstanceWindow>
  );
}

function ToolChoice({ on, title, sub, onPick }: { on: boolean; title: string; sub: string; onPick: () => void }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      onClick={onPick}
      className={`flex items-center gap-3 rounded-control bg-carbon-surface2 p-3 text-start ring-2 ring-inset transition-colors hover:bg-carbon-surface3 ${
        on ? "ring-accent" : "ring-transparent"
      }`}
    >
      <RadioMark on={on} />
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{title}</span>
        <span className="text-xs text-carbon-textMuted wrap-anywhere">{sub}</span>
      </span>
    </button>
  );
}

function DestinationRows({
  job,
  draft,
  locked,
  hostMountRoot,
  onEdit,
}: {
  job: ToolImportJob;
  draft: Draft;
  locked: boolean;
  hostMountRoot: string;
  onEdit: (change: (d: Draft) => Draft) => void;
}) {
  const { t } = useT();
  const { dest } = job;
  if (dest.kind === "") {
    return <SettingRow label={t("toolImport.locationLabel")} note={t("toolImport.locationNone")} />;
  }
  const signIn = needsSignIn(job, draft);
  return (
    <>
      <SettingRow
        label={t("toolImport.locationLabel")}
        name={
          <Checkbox
            label={t("toolImport.useLocation")}
            checked={draft.useLocation}
            disabled={locked}
            onChange={(useLocation) => onEdit((d) => ({ ...d, useLocation }))}
          />
        }
        note={
          draft.useLocation
            ? dest.existing
              ? t("toolImport.locationExisting").replace("{name}", () => dest.existing ?? "")
              : dest.signIn === "carried"
                ? t("toolImport.signInCarried")
                : undefined
            : t("toolImport.locationNone")
        }
      >
        {draft.useLocation &&
          !locked &&
          (dest.kind === "b2" ? null : dest.kind === "local" ? (
            <FolderBrowser
              label={t("toolImport.locationLabel")}
              value={draft.location}
              hostMountRoot={hostMountRoot}
              onChange={(location) => onEdit((d) => ({ ...d, location }))}
            />
          ) : (
            <input
              value={draft.location}
              aria-label={t("toolImport.locationLabel")}
              dir="ltr"
              onChange={(e) => onEdit((d) => ({ ...d, location: e.target.value }))}
              className={`${FIELD} font-mono`}
            />
          ))}
      </SettingRow>
      {draft.useLocation && dest.kind === "b2" && !locked && (
        <SettingRow label={t("toolImport.b2Endpoint")} hint={t("toolImport.b2EndpointHint")}>
          <input
            value={draft.endpoint}
            aria-label={t("toolImport.b2Endpoint")}
            placeholder="s3.eu-central-003.backblazeb2.com"
            dir="ltr"
            onChange={(e) => onEdit((d) => ({ ...d, endpoint: e.target.value }))}
            className={FIELD}
          />
        </SettingRow>
      )}
      {signIn && !locked && (
        <SettingRow label={t("toolImport.signInNeeded")}>
          <input
            value={draft.keyId}
            aria-label={t("toolImport.keyId")}
            placeholder={t("toolImport.keyId")}
            dir="ltr"
            autoComplete="off"
            onChange={(e) => onEdit((d) => ({ ...d, keyId: e.target.value }))}
            className={FIELD}
          />
          <input
            type="password"
            value={draft.secret}
            aria-label={t("toolImport.secret")}
            placeholder={t("toolImport.secret")}
            dir="ltr"
            autoComplete="new-password"
            onChange={(e) => onEdit((d) => ({ ...d, secret: e.target.value }))}
            className={FIELD}
          />
        </SettingRow>
      )}
    </>
  );
}
