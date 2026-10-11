import { useEffect, useState, type ReactNode } from "react";
import { Button } from "../../../components/Button";
import { CheckDraw } from "../../../components/CheckDraw";
import { IconSearch } from "../../../components/glyphs";
import { IconClose, IconDiagnostics } from "../../../components/navGlyphs";
import { BrandMark, ReadmeButton } from "../../../components/ReadmeButton";
import { Rows, SettingRow } from "../../../components/storage/rows";
import {
  checkForUpdate,
  downloadDiagnostics,
  getVersions,
  type ComponentVersions,
  type UpdateCheck,
  type UpdateCheckRefusal,
} from "../../../lib/api";
import { MANUAL_SVG } from "../../../lib/appMarks";
import { groupStage } from "../../../lib/controls";
import { useT, type TranslationKey } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { AboutGive, GLIMSTONE_REPO, GLIMSTONE_VERSION, REPO, ReportButtons, releaseUrl } from "../AboutCard";
import { Card, hueCounter } from "../shared";
import { WhatsNewCard } from "../WhatsNewCard";

const DOCS = "https://junkerderprovinz.github.io/bombvault/";

const REFUSED: Record<UpdateCheckRefusal, TranslationKey> = {
  "dev-build": "tile.info.refused.devBuild",
  offline: "tile.info.refused.offline",
  "rate-limited": "tile.info.refused.rateLimited",
  "bad-response": "tile.info.refused.badResponse",
};

/** A version number that opens its release page. A build without a tag has
 *  none and shows the number alone. */
function Version({ version, repo }: { version: string; repo?: string }) {
  const href = repo ? releaseUrl(repo, version) : "";
  const number = "font-mono text-sm tabular-nums";
  if (!href) return <span className={`${number} text-carbon-textSub`}>{version}</span>;
  return (
    // Under a coarse pointer the padding widens the target and the negative
    // margin takes it back, so the row keeps its height.
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={`${number} text-accentText no-underline hover:underline pointer-coarse:-m-2.5 pointer-coarse:p-2.5`}
    >
      {version}
    </a>
  );
}

type Check = { state: "idle" } | { state: "running" } | { state: "done"; update: UpdateCheck } | { state: "refused"; reason: string };

function UpdateButton() {
  const { t } = useT();
  const [check, setCheck] = useState<Check>({ state: "idle" });
  const [shake, setShake] = useState(0);

  async function run() {
    setCheck({ state: "running" });
    let next: Check;
    try {
      const res = await checkForUpdate();
      if (res.ok && res.update) next = { state: "done", update: res.update };
      else next = { state: "refused", reason: res.code ? t(REFUSED[res.code]) : (res.error ?? t("tile.info.refused.badResponse")) };
    } catch (e) {
      next = { state: "refused", reason: e instanceof Error && e.message ? e.message : t("tile.info.refused.offline") };
    }
    if (next.state === "refused") setShake((n) => n + 1);
    setCheck(next);
  }

  const resting = t("tile.info.check");
  const current = t("tile.info.upToDate");
  const waiting = t("tile.info.updates", 1);
  const failed = t("verdict.failed");
  let label = resting;
  let labelKey = "tile.info.check";
  let glyph: ReactNode = <IconSearch />;
  let tone: "neutral" | "ok" | "warn" | "danger" = "neutral";
  if (check.state === "done") {
    const behind = check.update.updateAvailable;
    label = behind ? waiting : current;
    labelKey = behind ? "tile.info.updates" : "tile.info.upToDate";
    glyph = behind ? <IconSearch /> : <CheckDraw />;
    tone = behind ? "warn" : "ok";
  } else if (check.state === "refused") {
    label = failed;
    labelKey = "verdict.failed";
    glyph = <IconClose />;
    tone = "danger";
  }

  return (
    <div className="flex flex-col items-end gap-2">
      {check.state === "refused" && <p className="text-xs text-statusFail wrap-break-word">{check.reason}</p>}
      {check.state === "done" && check.update.updateAvailable && (
        <p className="text-xs text-statusWarn">
          <a href={check.update.releaseUrl} target="_blank" rel="noopener noreferrer" className="underline-offset-2 hover:underline">
            {t("tile.info.available").replace("{version}", () => check.update.latest)}
          </a>
        </p>
      )}
      <Button
        key={shake}
        label={label}
        labelKey={labelKey}
        glyph={glyph}
        tone={tone}
        stage={groupStage([resting, current, waiting, failed])}
        onClick={() => void run()}
        disabled={check.state === "running"}
        busy={check.state === "running"}
        title={check.state === "idle" || check.state === "running" ? undefined : resting}
        className={check.state === "refused" ? "glim-shake" : ""}
      />
    </div>
  );
}

export function InfoPage() {
  const { t } = useT();
  const { push } = useToast();
  const [versions, setVersions] = useState<ComponentVersions | null>(null);
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

  async function bugReport() {
    const refused = await downloadDiagnostics();
    if (refused) push(refused, "fail");
  }

  const unknown = <span className="text-sm text-carbon-textMuted">{t("tile.info.versionUnknown")}</span>;
  const tool = (version: string | null | undefined) => (version ? <Version version={version} /> : unknown);

  return (
    <>
      <Card title={t("about.version")} hint={t("tile.info.versionHint")} hueIndex={nextHue()} newId="settings-version">
        <Rows>
          <SettingRow label="BombVault">
            {versions?.bombvault ? <Version version={versions.bombvault} repo={REPO} /> : unknown}
          </SettingRow>
          <SettingRow label="GlimStone" hint={t("tile.info.glimstoneHint")}>
            <Version version={GLIMSTONE_VERSION} repo={GLIMSTONE_REPO} />
          </SettingRow>
          <SettingRow label="restic" hint={t("tile.info.resticHint")}>
            {tool(versions?.restic)}
          </SettingRow>
          <SettingRow label="rclone" hint={t("tile.info.rcloneHint")}>
            {tool(versions?.rclone)}
          </SettingRow>
        </Rows>
        <UpdateButton />
      </Card>

      <Card title={t("tile.info.help")} hint={t("tile.info.helpHint")} hueIndex={nextHue()}>
        <p className="text-sm text-carbon-textSub">{t("about.report")}</p>
        <div className="glim-readme-btn-rows">
          <ReportButtons />
          <ReadmeButton
            tile="glim-tile-house"
            parts={[
              {
                name: t("tile.info.manual"),
                sub: t("instances.open"),
                onClick: () => window.open(DOCS, "_blank", "noopener,noreferrer"),
              },
            ]}
            mark={<BrandMark svg={MANUAL_SVG} />}
            markClass="glim-house-mark"
          />
          <ReadmeButton
            tile="glim-tile-house"
            parts={[{ name: t("tile.info.bugReport"), sub: t("tile.info.download"), onClick: () => void bugReport() }]}
            mark={<IconDiagnostics />}
            markClass="glim-house-mark"
          />
        </div>
      </Card>

      <WhatsNewCard hueIndex={nextHue()} />

      <Card title={t("about.title")} hueIndex={nextHue()}>
        <AboutGive />
      </Card>
    </>
  );
}
