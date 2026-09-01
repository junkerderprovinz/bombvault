// The About card ([363]) — GlimStone's "The About card (replaces the version
// footer)", adopted here.
//
// It REPLACES the version footer rather than joining it. That is the failure
// mode the spec names explicitly, and it is easy to walk into because both are
// individually defensible: the result is one number in two type sizes twelve
// pixels apart. The footer is gone in the same commit that adds this.
//
// jdp asked for it in the System tab specifically, which is a departure from
// the spec's "end of Settings" — and the right one for a tabbed Settings page.
// "The end of Settings" assumes a single scrolling page; here it would mean
// either repeating the card on all seven tabs or picking one, and System is
// where a version number belongs among the host integration and the export.
//
// What the spec asks for, and what each part is doing:
//
//   - Both versions, app first, read from the build. Never typed into the
//     card: a number written down twice disagrees with itself the day one of
//     them is bumped.
//   - Every version links to ITS OWN tag's release page, not to the releases
//     index. A version string answers "which build is this"; the question
//     straight after is "and what changed", and a number nobody can follow
//     makes somebody search a repository for a tag they then retype.
//   - One sentence inviting a report, naming both routes.
//   - Two buttons: the repository, and a mailto with the subject prefilled and
//     NO body, because a prefilled body reads as a form to fill in.
//
// This is also the one card in the language whose body is prose rather than an
// info bubble, and that is deliberate: a bubble hangs an explanation off a
// control, and this card has no control to explain.
import { useEffect, useState } from "react";
import { Button } from "../../components/Button";
import { getHealth } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { Card } from "./shared";

/**
 * The GlimStone release this interface is built against.
 *
 * Bumped by hand when index.css / lib/appearance.ts / lib/controls.ts are
 * re-copied from a newer release; check that repo's CHANGELOG before moving
 * it. Raised 1.2.0 -> 1.6.0 in [362] after an audit found it four releases
 * behind while the engines themselves had been kept current - a number that
 * had come to say the opposite of what it is for. 1.7.0 in [411], the release
 * this app's own number field was built from.
 */
export const GLIMSTONE_VERSION = "1.7.1";
const REPO = "https://github.com/junkerderprovinz/bombvault";
const GLIMSTONE_REPO = "https://github.com/junkerderprovinz/glimstone";
const SUPPORT_MAIL = "jdp@braethoria.com";

/**
 * The tag behind a running version string.
 *
 * The build stamps `v8.3.1+feature-control-engine.59b73a6`; the tag is the part
 * before the build metadata, which is exactly what semver says that "+" means.
 * Built from the version rather than kept in a list, because a hand-maintained
 * list of links is wrong the first time somebody forgets it.
 */
export function releaseTag(version: string): string {
  const bare = version.split("+")[0].trim();
  if (!bare) return "";
  return bare.startsWith("v") ? bare : `v${bare}`;
}

function VersionLink({ label, version, repo }: { label: string; version: string; repo: string }) {
  const tag = releaseTag(version);
  // No tag means a dev build ("dev", ""), and a link to a release page that
  // does not exist is worse than plain text.
  if (!tag) {
    return (
      <span className="font-mono tabular-nums text-carbon-textMuted">
        {label} {version}
      </span>
    );
  }
  return (
    <a
      href={`${repo}/releases/tag/${encodeURIComponent(tag)}`}
      target="_blank"
      rel="noopener noreferrer"
      className="font-mono tabular-nums text-carbon-textMuted underline decoration-dotted underline-offset-4 hover:text-carbon-text"
    >
      {label} {version}
    </a>
  );
}

export function AboutCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const [version, setVersion] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    getHealth()
      .then((h) => {
        if (active) setVersion(h.version ?? null);
      })
      .catch(() => {
        /* best-effort: the card is still worth showing without the number */
      });
    return () => {
      active = false;
    };
  }, []);

  const subject = encodeURIComponent(`BombVault ${version ?? ""}`.trim());

  return (
    <Card title={t("about.title")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-1">
        {version && <VersionLink label="BombVault" version={version} repo={REPO} />}
        <VersionLink label="GlimStone" version={GLIMSTONE_VERSION} repo={GLIMSTONE_REPO} />
      </div>

      <p className="max-w-2xl text-sm text-carbon-textSub">{t("about.report")}</p>

      <div className="flex flex-wrap items-center gap-2">
        <Button
          label={t("about.repo")}
          labelKey="about.repo"
          tone="neutral"
          onClick={() => window.open(REPO, "_blank", "noopener,noreferrer")}
        />
        <Button
          label={t("about.mail")}
          labelKey="about.mail"
          tone="neutral"
          // No body, only a subject: the mail should arrive already saying
          // which product and build it is about, and then leave the writing to
          // the person writing it.
          onClick={() => {
            window.location.href = `mailto:${SUPPORT_MAIL}?subject=${subject}`;
          }}
        />
      </div>
    </Card>
  );
}
