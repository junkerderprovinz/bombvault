// What the steps of the emergency window are laid out with.
import type { ReactNode } from "react";
import { InfoBubble } from "../InfoBubble";

export const EMERGENCY_FIELD =
  "rounded-control bg-carbon-surface3 px-3 py-2 text-start font-mono text-sm text-carbon-text glim-field-focus";

/** The folder each section keeps its backups in, by the setting that holds it. */
export const SECTION_PATHS = [
  ["containersPath", "settings.containersPath"],
  ["vmsPath", "settings.vmsPath"],
  ["flashPath", "settings.flashPath"],
  ["filesPath", "settings.filesPath"],
  ["zfsPath", "settings.zfsPath"],
] as const;

/** The off-site address of each section, by the setting that holds it. */
export const SECTION_OFFSITE = [
  ["containersOffsite", "nav.containers"],
  ["vmsOffsite", "nav.vms"],
  ["flashOffsite", "nav.flash"],
  ["filesOffsite", "nav.files"],
  ["zfsOffsite", "nav.zfs"],
] as const;

export function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-widest text-carbon-textSub">
        {title}
        {hint && <InfoBubble tip={hint} />}
      </h3>
      {children}
    </section>
  );
}

export function Note({ tone, children }: { tone: "ok" | "warn" | "fail" | "neutral"; children: ReactNode }) {
  const look = {
    ok: "bg-statusOkBg text-statusOk",
    warn: "bg-statusWarnBg text-statusWarn",
    fail: "bg-statusFailBgSoft text-statusFail",
    neutral: "bg-carbon-surface2 text-carbon-text",
  }[tone];
  return <div className={`rounded-card px-3 py-2.5 text-sm leading-relaxed wrap-break-word ${look}`}>{children}</div>;
}
