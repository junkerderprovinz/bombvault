import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "../../components/Button";
import { IconGithub, IconLink } from "../../components/glyphs";
import { BrandMark, ReadmeButton } from "../../components/ReadmeButton";
import { DOCKER_SVG, UNRAID_SVG } from "../../lib/appMarks";
import { copyText } from "../../lib/clipboard";
import type { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { UnraidTileSection } from "./DashboardWidgetCard";
import { Card } from "./shared";

// The Apps page: the house's companions to BombVault, each with the README's
// buttons for the ways to get it.

const PARLEYPORT_REPO = "https://github.com/junkerderprovinz/parleyport";
const WIDGET_REPO = "https://github.com/junkerderprovinz/bombvault-widget";
// Community Apps gives a listing its address only once it is in the feed, and
// the search finds it under either name before and after.
const CA_SEARCH = "https://ca.unraid.net/apps?q=";
const PARLEYPORT_RUN = "docker run -d --name parleyport --restart unless-stopped -p 8760:8760 junkerderprovinz/parleyport:latest";

function open(url: string) {
  window.open(url, "_blank", "noopener,noreferrer");
}

export function ParleyPortCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const navigate = useNavigate();
  const { push } = useToast();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const id = setTimeout(() => setCopied(false), 1800);
    return () => clearTimeout(id);
  }, [copied]);

  return (
    <Card title={t("apps.parleyport.title")} hint={t("apps.parleyport.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap gap-3">
        <ReadmeButton
          tile="glim-tile-unraid"
          parts={[{ name: "Unraid", sub: t("apps.unraidSub"), href: CA_SEARCH + "parleyport" }]}
          mark={<BrandMark svg={UNRAID_SVG} />}
        />
        <ReadmeButton
          tile="glim-tile-docker"
          parts={[
            {
              name: "Docker",
              sub: copied ? t("common.copied") : t("apps.dockerSub"),
              onClick: () =>
                void copyText(PARLEYPORT_RUN).then((ok) => {
                  if (ok) setCopied(true);
                  else push(t("vm.ssh.copyFailed"), "fail");
                }),
            },
          ]}
          mark={<BrandMark svg={DOCKER_SVG} />}
          markClass="glim-docker-mark"
          note={copied}
          hint={`${t("apps.parleyport.dockerHint")} ${PARLEYPORT_RUN}`}
        />
        <ReadmeButton
          tile="glim-tile-github"
          parts={[{ name: "GitHub", sub: t("apps.repoSub"), onClick: () => open(PARLEYPORT_REPO) }]}
          mark={<IconGithub />}
          markClass="glim-github-mark"
        />
      </div>
      <Button
        label={t("apps.parleyport.toRelay")}
        labelKey="apps.parleyport.toRelay"
        glyph={<IconLink />}
        tone="neutral"
        hueIndex={hueIndex}
        onClick={() => navigate("/settings/pairing")}
        className="self-start"
      />
    </Card>
  );
}

export function WidgetAppCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  return (
    <Card title={t("apps.widget.title")} hint={t("apps.widget.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap gap-3">
        <ReadmeButton
          tile="glim-tile-unraid"
          parts={[{ name: "Unraid", sub: t("apps.unraidSub"), href: CA_SEARCH + "bombvault%20widget" }]}
          mark={<BrandMark svg={UNRAID_SVG} />}
        />
        <ReadmeButton
          tile="glim-tile-github"
          parts={[{ name: "GitHub", sub: t("apps.repoSub"), onClick: () => open(WIDGET_REPO) }]}
          mark={<IconGithub />}
          markClass="glim-github-mark"
        />
      </div>
      <UnraidTileSection t={t} hueIndex={hueIndex} />
    </Card>
  );
}
