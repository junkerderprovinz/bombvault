import { useState } from "react";
import { adoptIntoDestination, deleteDestination, updateDestination, type AdoptableTarget, type Destination } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { destinationsChanged, useDestinations } from "../../lib/useDestinations";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { DestinationWizard } from "../../components/destinations/DestinationWizard";
import { InfoBubble } from "../../components/InfoBubble";
import { ProviderMark } from "../../components/destinations/ProviderPicker";
import { IconAdd } from "../../components/navGlyphs";
import { Toggle } from "../../components/Toggle";
import { Card } from "./shared";

const DOMAIN_LABEL: Record<string, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  flash: "nav.flash",
  files: "nav.files",
  zfs: "nav.zfs",
  config: "nav.config",
};

const FIELD = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";

/** The destinations, each set up once, and the wizard that adds one.
 *  onFieldCleared hears of a domain whose off-site field a takeover emptied,
 *  so the page does not save the old value back. */
export function DestinationsCard({ hueIndex, onFieldCleared }: { hueIndex?: number; onFieldCleared?: (domain: string) => void }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const { destinations, loaded, error } = useDestinations();
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<Destination | null>(null);
  const domainLabel = (x: string) => (DOMAIN_LABEL[x] ? t(DOMAIN_LABEL[x]) : x);

  async function adopt(d: Destination, a: AdoptableTarget) {
    const fill = (s: string) =>
      s.replaceAll("{target}", () => a.name).replaceAll("{domain}", () => domainLabel(a.domain)).replaceAll("{name}", () => d.name);
    const ask = fill(t("dest.adopt.ask")) + (a.primary ? ` ${fill(t("dest.adopt.askPrimary"))}` : "");
    if (!(await confirm(ask, { confirmKey: "dest.adopt.take" }))) return;
    const r = await adoptIntoDestination(d.id, a.id).catch((e: unknown) => ({ ok: false, error: e instanceof Error ? e.message : undefined }));
    if (!r.ok) {
      push(r.error ?? t("settings.error"), "fail");
      return;
    }
    push(fill(t("dest.adopted")), "success");
    if (a.primary) onFieldCleared?.(a.domain);
    destinationsChanged();
    offsiteTargetsChanged();
  }

  async function remove(d: Destination) {
    if (!(await confirm(t("dest.removeAsk").replace("{name}", () => d.name), { confirmKey: "dest.remove" }))) return;
    const r = await deleteDestination(d.id).catch((e: unknown) => ({ ok: false, error: e instanceof Error ? e.message : undefined }));
    if (!r.ok) {
      push(r.error ?? t("settings.error"), "fail");
      return;
    }
    push(t("dest.removed").replace("{name}", () => d.name), "success");
    destinationsChanged();
  }

  async function saveEdit() {
    if (!editing) return;
    const r = await updateDestination(editing.id, {
      name: editing.name,
      storageClass: editing.storageClass,
      immutable: editing.immutable,
    }).catch((e: unknown) => ({ ok: false, error: e instanceof Error ? e.message : undefined }));
    if (!r.ok) {
      push(r.error ?? t("settings.error"), "fail");
      return;
    }
    push(t("dest.saved").replace("{name}", () => editing.name), "success");
    setEditing(null);
    destinationsChanged();
    offsiteTargetsChanged();
  }

  return (
    <Card title={t("dest.title")} hint={t("dest.hint")} hueIndex={hueIndex}>
      {confirmDialog}
      <div className="flex flex-col gap-2">
        {error !== null && <p className="text-xs text-statusFail">{error || t("dest.loadError")}</p>}
        {loaded && destinations.length === 0 && !adding && <p className="text-sm text-carbon-textMuted">{t("dest.empty")}</p>}
        {destinations.map((d) => (
          <div key={d.id} className="glim-tile flex items-start justify-between gap-3 rounded-card p-3 max-md:flex-col">
            <div className="flex min-w-0 items-start gap-3">
              <span className="mt-0.5 flex h-6 w-7 shrink-0 justify-center text-carbon-textSub [&_svg]:h-full [&_svg]:w-full">
                <ProviderMark provider={d} />
              </span>
              <div className="flex min-w-0 flex-col gap-1">
                {editing?.id === d.id ? (
                  <input
                    value={editing.name}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                    className={FIELD}
                    aria-label={t("dest.name")}
                  />
                ) : (
                  <span className="truncate text-sm text-carbon-text">{d.name}</span>
                )}
                <span dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textMuted">{d.repo}</span>
                <span className="flex flex-wrap gap-2">
                  {d.domains.length > 0 ? (
                    <Badge tone="neutral" size="medium" wrap className="glim-tile-raise">
                      {t("dest.usedBy").replace("{domains}", () => formatList(lang, d.domains.map(domainLabel)))}
                    </Badge>
                  ) : (
                    <Badge tone="muted" size="medium" wrap>{t("dest.unused")}</Badge>
                  )}
                  {d.immutable && (
                    <Badge tone="ok" size="medium" wrap>{t("offsite.immutable")}</Badge>
                  )}
                </span>
                {editing?.id !== d.id && d.adoptable && d.adoptable.length > 0 && (
                  <div className="mt-1 flex flex-col gap-1.5">
                    <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                      {t("dest.adopt.title")}
                      <InfoBubble tip={t("dest.adopt.tip")} />
                    </span>
                    {d.adoptable.map((a) => (
                      <div key={a.id} className="flex flex-wrap items-center gap-2">
                        <span className="text-xs text-carbon-text">{domainLabel(a.domain)}</span>
                        <span dir="ltr" className="min-w-0 break-all text-start font-mono text-xs text-carbon-textMuted">{a.repo}</span>
                        <Button
                          label={t("dest.adopt.take")}
                          labelKey="dest.adopt.take"
                          tone="neutral"
                          onClick={() => void adopt(d, a)}
                          className="glim-tile-raise"
                        />
                      </div>
                    ))}
                  </div>
                )}
                {editing?.id === d.id && (
                  <Toggle
                    label={t("offsite.immutable")}
                    checked={editing.immutable}
                    onChange={(v) => setEditing({ ...editing, immutable: v })}
                  />
                )}
              </div>
            </div>
            <div className="flex shrink-0 flex-wrap items-start gap-2">
              {editing?.id === d.id ? (
                <>
                  <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={() => setEditing(null)} className="glim-tile-raise" />
                  <Button label={t("settings.save")} labelKey="settings.save" tone="accent" onClick={() => void saveEdit()} />
                </>
              ) : (
                <>
                  <Button label={t("dest.edit")} labelKey="dest.edit" tone="neutral" onClick={() => setEditing(d)} className="glim-tile-raise" />
                  <Button label={t("dest.remove")} labelKey="dest.remove" tone="neutral" onClick={() => void remove(d)} className="glim-tile-raise" />
                </>
              )}
            </div>
          </div>
        ))}
        {adding ? (
          <DestinationWizard
            onDone={() => {
              setAdding(false);
              destinationsChanged();
            }}
            onCancel={() => setAdding(false)}
          />
        ) : (
          <Button
            label={t("dest.add")}
            labelKey="dest.add"
            glyph={<IconAdd />}
            tone="accent"
            onClick={() => setAdding(true)}
            className="self-start"
          />
        )}
      </div>
    </Card>
  );
}
