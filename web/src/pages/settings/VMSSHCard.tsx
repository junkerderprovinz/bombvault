import { Button } from "../../components/Button";
import { IconInfo } from "../../components/glyphs";
import { IconCopy } from "../../components/navGlyphs";
import { Rows, SettingRow } from "../../components/storage/rows";
import { getVMSSH, testVMSSH } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { authorizeCommand } from "../../lib/sshAuthorize";
import { useToast } from "../../lib/toast";
import { Card, SubCard } from "./shared";
import { TestButton, VerdictLine } from "../../components/TestButton";
import { useTestVerdict } from "../../lib/useTestVerdict";
import { useEffect, useState } from "react";

const GUIDE_URL = "https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md";

// VMSSHCard shows BombVault's SSH public key (to authorize on the Unraid host)
// and a connection test for the link VM and ZFS backups share. It fetches its
// own data so SettingsPage does not need extra state.
export function VMSSHCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [host, setHost] = useState("");
  const [pub, setPub] = useState("");
  const test = useTestVerdict(host, t("vm.ssh.testFail"));

  useEffect(() => {
    getVMSSH()
      .then((r) => {
        if (r.ok) {
          setHost(r.host ?? "");
          setPub(r.publicKey ?? "");
        }
      })
      .catch(() => undefined);
  }, []);

  // Without libvirt the link still works, and ZFS backups need nothing more,
  // so the test passes and the line above the button names what is missing.
  function handleTest() {
    void test.run(async () => {
      const r = await testVMSSH();
      if (!r.ok) return { ok: false, reason: r.error ?? t("vm.ssh.testFail") };
      return r.libvirt === false ? { ok: true, note: t("vm.ssh.testNoLibvirt"), caveat: true } : { ok: true };
    });
  }

  // copyText falls back to execCommand in non-secure contexts (#112), so a
  // failure here means both ways failed.
  async function copy(text: string) {
    const ok = await copyText(text);
    push(ok ? t("common.copied") : t("vm.ssh.copyFailed"), ok ? "success" : "fail");
  }

  return (
    <Card title={t("vm.ssh.title")} hint={t("vm.ssh.cardHint")} hueIndex={hueIndex}>
      <Rows>
        <SettingRow label={t("vm.ssh.host")} hint={t("vm.ssh.hostHint")}>
          <code dir="ltr" className="font-mono text-sm font-semibold text-carbon-text">
            {host || "-"}
          </code>
        </SettingRow>
        <SettingRow label={t("vm.ssh.keyLabel")} hint={t("vm.ssh.keyHint")}>
          <code
            dir="ltr"
            className="w-80 max-w-full truncate rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-xs text-carbon-text"
          >
            {pub || "-"}
          </code>
          <Button label={t("common.copy")} labelKey="common.copy" glyph={<IconCopy />} onClick={() => void copy(pub)} disabled={!pub} />
        </SettingRow>
      </Rows>

      <SubCard title={t("settings.widgetSetup")}>
        <ol className="flex list-decimal flex-col gap-2 ps-5 text-sm text-carbon-text">
          <li>
            <span className="flex flex-wrap items-center gap-2">
              {t("vm.ssh.stepCopy")}
              <Button
                label={t("vm.ssh.copyCmd")}
                labelKey="vm.ssh.copyCmd"
                glyph={<IconCopy />}
                onClick={() => void copy(authorizeCommand(pub))}
                disabled={!pub}
                hueIndex={hueIndex}
                className="glim-btn-wrap"
              />
            </span>
          </li>
          <li>{t("vm.ssh.stepTerminal")}</li>
          <li>{t("vm.ssh.stepPaste")}</li>
          <li>
            <VerdictLine verdict={test.verdict} />
            <span className="flex flex-wrap items-center gap-2">
              {t("vm.ssh.stepTest")}
              <TestButton
                label={t("vm.ssh.test")}
                labelKey="vm.ssh.test"
                test={test}
                onClick={handleTest}
                hueIndex={hueIndex}
                className="glim-btn-wrap"
              />
            </span>
          </li>
        </ol>
        <div className="flex justify-end pt-3">
          <Button
            label={t("vm.ssh.fullGuide")}
            labelKey="vm.ssh.fullGuide"
            glyph={<IconInfo />}
            className="glim-btn-wrap"
            onClick={() => window.open(GUIDE_URL, "_blank", "noopener,noreferrer")}
          />
        </div>
      </SubCard>
    </Card>
  );
}
