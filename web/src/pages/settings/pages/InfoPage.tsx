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

const DOCS = "https://junkerderprovinz.github.io/bombvault/";
// A container cannot replace itself, so "How to update" leads to how it was set up.
const GUIDE = `${DOCS}getting-started/#install-on-unraid`;

const REFUSED: Record<UpdateCheckRefusal, TranslationKey> = {
  "dev-build": "tile.info.refused.devBuild",
  offline: "tile.info.refused.offline",
  "rate-limited": "tile.info.refused.rateLimited",
  "bad-response": "tile.info.refused.badResponse",
};

export function InfoPage() {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { push } = useToast();
  const unseen = useWhatsNew().unseen.length;
  const [versions, setVersions] = useState<ComponentVersions | null>(null);
  const [update, setUpdate] = useState<UpdateState | null>(null);
  const [release, setRelease] = useState("");
  const words = useMemo(() => componentText(lang), [lang]);
  const nextHue = hueCounter();

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
      if (res.ok && res.update?.updateAvailable) {
        setRelease(res.update.releaseUrl);
        setUpdate({ kind: "available", version: shownVersion(res.update.latest) });
      } else if (res.ok && res.update) {
        setUpdate({ kind: "current", checkedAt: new Date() });
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
        version={versions?.bombvault ?? ""}
        repoUrl={REPO}
        route="guide"
        state={update}
        onCheck={() => void check()}
        guideUrl={GUIDE}
        // Once a newer release is found, What's new tells what that one brings.
        onWhatsNew={() => (update?.kind === "available" ? window.open(release, "_blank", "noopener,noreferrer") : openWhatsNew(true))}
        unseen={unseen}
        newId="settings-version"
        hueIndex={nextHue()}
      />

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
