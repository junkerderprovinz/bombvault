import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { componentText, type ComponentRow, type UpdateState } from "@glimstone/core";
import { Button } from "../../../components/Button";
import { IconForward } from "../../../components/glyphs";
import { IconDiagnostics } from "../../../components/navGlyphs";
import { BrandMark, ReadmeButton } from "../../../components/ReadmeButton";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { checkForUpdate, downloadDiagnostics, getVersions, type ComponentVersions, type UpdateCheckRefusal } from "../../../lib/api";
import { MANUAL_SVG } from "../../../lib/appMarks";
import { useT, type TranslationKey } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { openWhatsNew, useWhatsNew } from "../../../lib/whatsNewStore";
import { AboutGive, REPO, ReportButtons, shownVersion } from "../AboutCard";
import { InfoComponentsCard, InfoVersionCard } from "../InfoCards";
import { Card, hueCounter } from "../shared";
import { NewReleaseWindow } from "../WhatsNew";

const DOCS = "https://junkerderprovinz.github.io/bombvault/";
// A container cannot replace itself, so "How to update" leads to the steps for each host.
const GUIDE = `${DOCS}getting-started/#update`;

const REFUSED: Record<UpdateCheckRefusal, TranslationKey> = {
  "dev-build": "tile.info.refused.devBuild",
  offline: "tile.info.refused.offline",
  "rate-limited": "tile.info.refused.rateLimited",
  "bad-response": "tile.info.refused.badResponse",
};

const LAST_CHECK = "bombvault.lastUpdateCheck";

/** The answer of the last check this browser ran, so the card can say when it looked. */
type LastCheck = { at: string; running: string; latest: string; available: boolean };

function readLastCheck(running: string): LastCheck | null {
  try {
    const last = JSON.parse(localStorage.getItem(LAST_CHECK) ?? "null") as LastCheck | null;
    // An answer about the release that ran before an update says nothing
    // about this one.
    return last && last.running === running ? last : null;
  } catch {
    return null;
  }
}

function keepLastCheck(last: LastCheck) {
  try {
    localStorage.setItem(LAST_CHECK, JSON.stringify(last));
  } catch {
    // Without storage the card forgets the answer on the next visit.
  }
}

function stateOf(last: LastCheck): UpdateState {
  return last.available ? { kind: "available", version: shownVersion(last.latest) } : { kind: "current", checkedAt: new Date(last.at) };
}

export function InfoPage() {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { push } = useToast();
  const unseen = useWhatsNew().unseen.length;
  const [versions, setVersions] = useState<ComponentVersions | null>(null);
  const [update, setUpdate] = useState<UpdateState>({ kind: "unchecked" });
  const [notes, setNotes] = useState<string | null>(null);
  const words = useMemo(() => componentText(lang), [lang]);
  const nextHue = hueCounter();

  const running = versions?.bombvault ?? "";
  useEffect(() => {
    const last = running ? readLastCheck(running) : null;
    if (!last) return;
    setUpdate(stateOf(last));
  }, [running]);

  useEffect(() => {
    let alive = true;
    getVersions()
      .then((res) => {
        if (alive && res.ok) setVersions(res.versions);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  async function check() {
    setUpdate({ kind: "checking" });
    try {
      const res = await checkForUpdate();
      if (res.ok && res.update) {
        const last = {
          at: new Date().toISOString(),
          running,
          latest: res.update.latest,
          available: res.update.updateAvailable,
        };
        keepLastCheck(last);
        setUpdate(stateOf(last));
      } else {
        setUpdate({ kind: "failed", reason: res.code ? t(REFUSED[res.code]) : (res.error ?? t("tile.info.refused.badResponse")) });
      }
    } catch (e) {
      setUpdate({ kind: "failed", reason: e instanceof Error && e.message ? e.message : t("tile.info.refused.offline") });
    }
  }

  async function bugReport() {
    const refused = await downloadDiagnostics();
    if (refused) push(refused, "fail");
  }

  const tool = (name: string, repo: string, hint: TranslationKey, version: string | null | undefined): ComponentRow => ({
    name,
    kind: "tool",
    hint: t(hint),
    version: version ?? "",
    repoUrl: repo,
    status: versions && !version ? t("tile.info.versionUnknown") : undefined,
  });

  return (
    <>
      <InfoVersionCard
        text={words.versionCard}
        name="BombVault"
        version={running}
        repoUrl={REPO}
        route="guide"
        state={update}
        onCheck={() => void check()}
        guideUrl={GUIDE}
        // Once a newer release is found, What's new tells what that one brings.
        onWhatsNew={() => (update.kind === "available" ? setNotes(update.version) : openWhatsNew(true))}
        unseen={unseen}
        newId="settings-version"
        hueIndex={nextHue()}
      />
      {notes && <NewReleaseWindow version={notes} onClose={() => setNotes(null)} />}

      <InfoComponentsCard
        text={words.componentsCard}
        name="BombVault"
        rows={[
          tool("restic", "https://github.com/restic/restic", "tile.info.resticHint", versions?.restic),
          tool("rclone", "https://github.com/rclone/rclone", "tile.info.rcloneHint", versions?.rclone),
        ]}
        hueIndex={nextHue()}
      />

      <Card title={t("tile.info.help")} hint={t("tile.info.helpHint")} hueIndex={nextHue()}>
        <p className="text-sm text-carbon-textSub">{t("about.report")}</p>
        <div className="glim-readme-btn-rows">
          <ReportButtons />
          {/* The README's docs button, so whoever opened the manual there knows it. */}
          <ReadmeButton
            tile="glim-tile-docs"
            parts={[
              {
                name: t("tile.info.manual"),
                sub: t("instances.open"),
                onClick: () => window.open(DOCS, "_blank", "noopener,noreferrer"),
              },
            ]}
            mark={<BrandMark svg={MANUAL_SVG} />}
            markClass="glim-docs-mark"
            newId="settings-manual"
          />
          {/* The file goes with an issue on GitHub, so it lights up beside it. */}
          <ReadmeButton
            tile="glim-tile-github"
            parts={[{ name: t("tile.info.bugReport"), sub: t("tile.info.download"), onClick: () => void bugReport() }]}
            mark={<IconDiagnostics />}
            markClass="glim-github-mark"
          />
        </div>
        {/* The welcome page passes an install with a backup on to the
            overview, so this is the way back to it on purpose. */}
        <Rows>
          <SettingRow label={t("tile.info.welcome")} hint={t("tile.info.welcomeHint")}>
            <Button
              label={t("instances.open")}
              labelKey="instances.open"
              glyph={<IconForward />}
              onClick={() => navigate("/welcome?again")}
            />
          </SettingRow>
        </Rows>
      </Card>

      <Card title={t("about.title")} hueIndex={nextHue()}>
        <AboutGive />
      </Card>
    </>
  );
}
