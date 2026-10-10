import { useEffect, useState } from "react";
import { setVMMethod } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { IconPower, IconLive } from "../Sidebar";
import { Selector } from "../Selector";
import { useToast } from "../../lib/toast";

// VMMethodSelect picks the per-VM backup method (graceful shutdown vs live
// snapshot) via PATCH /api/vms/{name}.
export function VMMethodSelect({
  name,
  initial,
  t,
}: {
  name: string;
  initial: string;
  t: ReturnType<typeof useT>["t"];
}) {
  const [method, setMethod] = useState(initial || "graceful");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();

  // Rows are keyed by libvirt name and do not remount, so re-seed when a list
  // reload hands down a new value.
  useEffect(() => setMethod(initial || "graceful"), [initial]);

  async function handleChange(next: string) {
    // Reverted on failure, so a rejected switch to "live" does not leave the
    // UI promising no downtime while the next backup shuts the VM down.
    const prev = method;
    setMethod(next);
    setBusy(true);
    try {
      const res = await setVMMethod(name, next);
      if (!res.ok) {
        setMethod(prev);
        push(res.error ?? t("vm.method.saveFailed"), "fail");
      }
    } catch (err) {
      setMethod(prev);
      push(err instanceof Error ? err.message : t("vm.method.saveFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  // Both options stay visible with the active one filled, rather than one
  // badge that cycles: this decides whether the VM is shut down for its
  // backup, so the alternative should be in view. The tip names the method.
  return (
    <Selector
      items={[
        {
          id: "graceful",
          label: t("vm.method.graceful"),
          icon: <IconPower />,
          tip: t("vm.method.graceful"),
        },
        {
          id: "live",
          label: t("vm.method.live"),
          icon: <IconLive />,
          tip: t("vm.method.live"),
        },
      ]}
      label={t("vm.method")}
      size="sm"
      select="one"
      equalWidth
      inline
      disabled={busy}
      active={method}
      onChange={(id) => void handleChange(id)}
    />
  );
}
