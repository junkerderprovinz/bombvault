import { useState } from "react";

import { useT } from "../../lib/i18n";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { ZFSConnectionStatus } from "../zfs/ZFSConnectionCard";

/** ZFSConnectionWindow shows whether zfs commands reach this server, and
 *  tests the connection again each time it opens. */
export function ZFSConnectionWindow({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const [foot, setFoot] = useState<HTMLElement | null>(null);
  return (
    <InstanceWindow
      title={t("zfs.connection.title")}
      hint={t("zfs.connection.hint")}
      footer={
        <>
          <Button label={t("common.close")} labelKey="common.close" tone="subtle" onClick={onClose} />
          <span ref={setFoot} className="contents" />
        </>
      }
      onClose={onClose}
    >
      <ZFSConnectionStatus foot={foot} />
    </InstanceWindow>
  );
}
