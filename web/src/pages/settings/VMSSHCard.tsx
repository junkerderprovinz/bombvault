import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconCopy } from "../../components/Sidebar";
import { getVMSSH, testVMSSH } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { useToast } from "../../lib/toast";
import { Card } from "./shared";
import { useEffect, useState } from "react";

// VMSSHCard shows BombVault's SSH public key (to authorize on the Unraid host)
// and a connection test. It fetches its own data so SettingsPage does not
// need extra state.
export function VMSSHCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [host, setHost] = useState("");
  const [pub, setPub] = useState("");
  const [testState, setTestState] = useState<"idle" | "testing" | "ok" | "fail">("idle");
  // Bumped on a failed test; the Test button is keyed on it, so each bump
  // replays its shake.
  const [shake, setShake] = useState(0);

  // Ready-to-paste command that authorizes this key on the Unraid host, both for
  // the live session and persistently (Unraid restores root.pubkeys on boot).
  const authorizeCmd = pub
    ? `mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
echo '${pub}' | tee -a /root/.ssh/authorized_keys /boot/config/ssh/root.pubkeys >/dev/null
chmod 600 /root/.ssh/authorized_keys`
    : "";

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

  async function handleTest() {
    setTestState("testing");
    try {
      const r = await testVMSSH();
      if (r.ok) {
        setTestState("ok");
      } else {
        setTestState("fail");
        push(r.error ?? t("vm.ssh.testFail"), "fail");
        setShake((n) => n + 1);
      }
    } catch {
      setTestState("fail");
      push(t("vm.ssh.testFail"), "fail");
      setShake((n) => n + 1);
    }
  }

  // copyText falls back to execCommand in non-secure contexts (#112).
  async function handleCopy() {
    if (await copyText(pub)) {
      push(t("common.copied"), "success");
    } else {
      // copyText fails only when the Clipboard API and the execCommand
      // fallback both failed, so this is a real failure, not routine noise.
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  async function handleCopyCmd() {
    if (await copyText(authorizeCmd)) {
      push(t("common.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <Card title={t("vm.ssh.title")} hint={t("vm.ssh.desc")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-3">
        <div className="text-sm text-carbon-text">
          {t("vm.ssh.host")}: <span dir="ltr" className="font-mono text-start">{host || "—"}</span>
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textMuted">{tLtr(t, "vm.ssh.publicKey")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {pub || "—"}
            </code>
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              glyph={<IconCopy />}
              tone="accent"
              onClick={() => void handleCopy()}
              disabled={!pub}
              hueIndex={hueIndex}
              className={"shrink-0"}
            />
          </div>
        </div>

        {/* One-time setup instructions */}
        <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-2">
          <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
            {t("vm.ssh.setupTitle")}
          </span>
          <ol className="list-decimal ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
            <li>{t("vm.ssh.step1")}</li>
            <li>{t("vm.ssh.step2")}</li>
            <li>{t("vm.ssh.step3")}</li>
          </ol>
          <div className="flex items-start gap-2">
            <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">{authorizeCmd || "—"}</pre>
            <Button
              label={t("vm.ssh.copyCmd")}
              labelKey="vm.ssh.copyCmd"
              glyph={<IconCopy />}
              tone="accent"
              onClick={() => void handleCopyCmd()}
              disabled={!pub}
              hueIndex={hueIndex}
              className={"shrink-0"}
            />
          </div>
          <Badge
            as="a"
            href="https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md"
            target="_blank"
            rel="noreferrer"
            tone="neutral"
            size="small"
            className="self-start"
          >
            {t("vm.ssh.guide")} →
          </Badge>
        </div>

        <div className="flex items-center gap-3">
          <Button
            key={shake || 0}
            label={t("vm.ssh.test")}
            labelKey="vm.ssh.test"
            // Accent, not neutral: it is the one thing this card asks you to
            // do, and the card is useless until it has been done once.
            tone="accent"
            onClick={handleTest}
            disabled={testState === "testing"}
            busy={testState === "testing"}
            title={testState === "testing" ? t("vm.ssh.testing") : undefined}
            className={shake ? "glim-shake" : ""}
            hueIndex={hueIndex}
          />
          {testState === "ok" && (
            <span className="text-sm text-statusOk">{t("vm.ssh.testOk")}</span>
          )}
          {/* The error itself went to the toast; this only marks the state. */}
          {testState === "fail" && (
            <span className="text-sm text-statusFail">{t("vm.ssh.testFail")}</span>
          )}
        </div>
      </div>
    </Card>
  );
}
