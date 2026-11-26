import { useEffect, useState } from "react";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconClose } from "../components/navGlyphs";
import { ReleaseNotesText, releasePage } from "../components/ReleaseNotes";
import { useT } from "../lib/i18n";
import { RELEASES } from "../lib/whatsNew";
import { InstanceWindow } from "../pages/instances/InstanceWindow";

// Every release's notes ride along in the APK, each in a chunk of its own, so
// the window opens offline and loads only the notes it shows.
const NOTES = import.meta.glob<string>("../../../internal/releasenotes/notes/v*.md", { query: "?raw", import: "default" });

const notesPath = (version: string) => `../../../internal/releasenotes/notes/v${version}.md`;

/** notesOf loads the notes of a release from the APK, or gives null for a
 *  version without any. */
export function notesOf(version: string): Promise<string | null> {
  const load = NOTES[notesPath(version)];
  return load ? load() : Promise.resolve(null);
}

/** The release the window shows: the app's own, or for a build of no release
 *  the newest one What's new lists, as the web's window does. */
function releaseShown(version: string): string {
  return NOTES[notesPath(version)] ? version : RELEASES[0].version;
}

/** What a release brought as the web's What's new lists it, without the
 *  changes on pages a paired phone does not reach. */
function changesOf(version: string) {
  const release = RELEASES.find((r) => r.version === version);
  return release ? release.changes.filter((change) => !(change.page !== undefined && change.unpaired)) : [];
}

/** WhatsNewWindow shows the notes of the app's own release and what it
 *  brought, as the web's window does for the release a server runs. */
export function WhatsNewWindow({ version: running, onClose }: { version: string; onClose: () => void }) {
  const { t } = useT();
  const version = releaseShown(running);
  const [notes, setNotes] = useState<string | null>();
  useEffect(() => {
    let active = true;
    void notesOf(version).then((body) => {
      if (active) setNotes(body);
    });
    return () => {
      active = false;
    };
  }, [version]);
  const changes = changesOf(version);

  return (
    <InstanceWindow
      title={t("whatsNew.windowTitle").replace("{version}", version)}
      onClose={onClose}
      footer={
        <>
          <Badge as="a" href={releasePage(`v${version}`)} target="_blank" rel="noopener noreferrer" tone="neutral" size="large" className="me-auto">
            {t("whatsnew.viewOnGitHub")}
          </Badge>
          <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={onClose} />
        </>
      }
    >
      {notes === undefined ? null : notes === null ? (
        <p className="text-sm text-carbon-textSub">{t("whatsnew.loadFailed")}</p>
      ) : (
        <ReleaseNotesText body={notes} />
      )}
      {changes.length > 0 && (
        <ul className="flex flex-col gap-1.5">
          {changes.map((change) => (
            <li key={change.id} className="rounded-control bg-carbon-surface2 px-3 py-2.5 text-sm text-carbon-text wrap-break-word">
              {t(change.text)}
            </li>
          ))}
        </ul>
      )}
    </InstanceWindow>
  );
}
