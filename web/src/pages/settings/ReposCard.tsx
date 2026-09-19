import { useEffect, useRef, useState } from "react";
import { Card } from "./shared";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { Toggle } from "../../components/Toggle";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { createRepo, deleteRepo, listRepos, updateRepo, type NamedRepo } from "../../lib/api";
import { useT } from "../../lib/i18n";

// ReposCard — where the named repositories of #204 are written down.
//
// The issue asked to back a VM or a folder up straight to a B2 bucket or a NAS
// share, past the domain's own repository. The first cut took a free-text
// location typed into each item; jdp's read was that this is the wrong shape,
// and he is right: ten containers meant typing the same bucket path ten times,
// and correcting it later meant finding all ten. A location is written here
// ONCE and picked per item.
//
// Two refusals carry the weight of this card, and both come from the same fact:
// nothing ever re-homes a backup that has been written.
//
//   - the LOCATION of a repository in use cannot be moved. Everything written
//     so far stays where it is, so the next backup would succeed into an empty
//     repository, which is indistinguishable from a working one.
//   - a repository in use cannot be deleted. The items would fall back to their
//     domain repository silently, and their next backup would land somewhere
//     else while looking exactly as green as before.
//
// The interface says so BEFORE the attempt (the in-use count is on every row)
// rather than only in the error afterwards.
export function ReposCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [repos, setRepos] = useState<NamedRepo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [name, setName] = useState("");
  const [repo, setRepo] = useState("");
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const nameRef = useRef<HTMLInputElement>(null);

  // A failed list leaves the card empty and LOADED, the same answer the picker
  // gives: the alternative is a discarded rejection (void on a promise that can
  // reject), which shows up as an unhandled error and leaves the card stuck on
  // its loading state with nothing on screen to say why.
  async function reload() {
    try {
      const r = await listRepos();
      setRepos(r.ok ? (r.repos ?? []) : []);
    } catch {
      setRepos([]);
    } finally {
      setLoaded(true);
    }
  }

  useEffect(() => {
    void reload();
  }, []);

  async function add() {
    if (busy) return;
    setBusy(true);
    try {
      const r = await createRepo({ name: name.trim(), repo: repo.trim(), enabled: true });
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
        return;
      }
      setName("");
      setRepo("");
      nameRef.current?.focus();
      await reload();
    } finally {
      setBusy(false);
    }
  }

  async function setEnabled(row: NamedRepo, next: boolean) {
    // Switching a repository OFF while items point at it is allowed - it is how
    // a share that went away is parked - but it stops those items backing up,
    // and nothing else on screen says so. The count is already here, so ask
    // before doing it rather than letting it be found in a failed run.
    if (!next && row.inUse !== 0) {
      // Its own sentence rather than a phrase substituted into a numeric slot:
      // "Items backing up here: in use: unknown" is not a sentence anybody wrote.
      const question =
        row.inUse < 0
          ? t("repos.disableWarnUnknown")
          : t("repos.disableWarn").replace("{n}", String(row.inUse));
      if (!(await confirm(question))) return;
    }
    const r = await updateRepo(row.id, { enabled: next });
    if (!r.ok) {
      push(r.error ?? t("settings.error"), "fail");
      return;
    }
    await reload();
  }

  async function setImmutable(row: NamedRepo, next: boolean) {
    const r = await updateRepo(row.id, { immutable: next });
    if (!r.ok) {
      push(r.error ?? t("settings.error"), "fail");
      return;
    }
    await reload();
  }

  async function remove(row: NamedRepo) {
    // An UNKNOWN count gets its own sentence. "Still in use" is a statement of
    // fact the server did not make: it could not read the count at all, and
    // telling somebody their repository is in use when nobody knows sends them
    // looking for items that may not exist.
    if (row.inUse < 0) {
      push(t("repos.deleteBlockedUnknown"), "fail");
      return;
    }
    if (row.inUse !== 0) {
      push(t("repos.deleteBlocked"), "fail");
      return;
    }
    if (!(await confirm(`${row.name} - ${row.repo}`))) return;
    const r = await deleteRepo(row.id);
    if (!r.ok) {
      push(r.error ?? t("settings.error"), "fail");
      return;
    }
    await reload();
  }

  return (
    <Card title={t("repos.title")} hint={t("repos.intro")} hueIndex={hueIndex}>
      {confirmDialog}
      <div className="flex flex-col gap-3">
        {loaded && repos.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("repos.empty")}</p>
        )}

        {repos.map((r) => (
          <div
            key={r.id}
            className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2"
          >
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm text-carbon-text font-semibold truncate">{r.name}</span>
                {/* A negative count means the server could not read it. That is
                    treated as IN USE everywhere below: an unknown answer must
                    not be the one that unlocks deleting. */}
                {r.inUse < 0 ? (
                  <Badge tone="neutral" wrap>
                    {t("repos.inUseUnknown")}
                  </Badge>
                ) : r.inUse > 0 ? (
                  <Badge tone="neutral" wrap>
                    {t("repos.inUse").replace("{n}", String(r.inUse))}
                  </Badge>
                ) : (
                  <Badge tone="neutral" wrap>
                    {t("repos.unused")}
                  </Badge>
                )}
              </div>
              <p className="text-xs text-carbon-textSub font-mono truncate" dir="ltr">
                {r.repo}
              </p>
            </div>
            {/* Append-only. Prune and snapshot delete refuse on a repository
                marked this way, which is the only thing standing between an
                on-box credential and an archive somebody meant to keep. It was
                readable by the engine and settable nowhere, so the refusal its
                own doc comment promised could not be reached. */}
            <label className="flex items-center gap-2 text-xs text-carbon-textSub">
              {t("repos.immutable")}
              <InfoBubble tip={t("repos.immutableHint")} />
              <Toggle
                checked={r.immutable}
                onChange={(v) => void setImmutable(r, v)}
                label={t("repos.immutable")}
                hideLabel
              />
            </label>
            <label className="flex items-center gap-2 text-xs text-carbon-textSub">
              {t("repos.enabled")}
              <Toggle
                checked={r.enabled}
                onChange={(v) => void setEnabled(r, v)}
                label={t("repos.enabled")}
                hideLabel
              />
            </label>
            {/* The location is not editable here on purpose - see the card's
                own note. The tooltip says why rather than leaving a greyed
                field to puzzle over. */}
            {r.inUse !== 0 && <InfoBubble tip={t("repos.locationLocked")} />}
            <Button
              label={t("offsite.targets.remove")}
              labelKey="offsite.targets.remove"
              tone="neutral"
              onClick={() => void remove(r)}
              disabled={r.inUse !== 0}
            />
          </div>
        ))}

        <div className="flex items-end gap-2 flex-wrap pt-1">
          <div className="flex flex-col gap-1 flex-1 min-w-[10rem]">
            <label className="text-xs text-carbon-textSub" htmlFor="repo-name">
              {t("repos.name")}
            </label>
            <input
              id="repo-name"
              ref={nameRef}
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("repos.namePlaceholder")}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </div>
          <div className="flex flex-col gap-1 flex-[2] min-w-[14rem]">
            <label className="flex items-center gap-1 text-xs text-carbon-textSub" htmlFor="repo-location">
              {t("repos.location")}
              <InfoBubble tip={t("repos.locationHint")} />
            </label>
            <input
              id="repo-location"
              type="text"
              value={repo}
              onChange={(e) => setRepo(e.target.value)}
              placeholder={t("repos.locationPlaceholder")}
              dir="ltr"
              spellCheck={false}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
            />
          </div>
          <Button
            key={shake}
            label={t("repos.add")}
            labelKey="repos.add"
            tone="accent"
            onClick={() => void add()}
            disabled={busy || name.trim() === "" || repo.trim() === ""}
          />
        </div>
      </div>
    </Card>
  );
}
