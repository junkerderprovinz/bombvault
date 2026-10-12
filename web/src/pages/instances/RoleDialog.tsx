import type { ReactNode } from "react";

import { InstanceWindow } from "./InstanceWindow";

const stay = () => undefined;

/** RoleDialog is the small window a role is asked for in. While it is
 *  `locked`, neither Escape nor a click beside it closes it, for content
 *  that cannot be shown again. */
export function RoleDialog({
  title,
  hint,
  lead,
  footer,
  locked = false,
  onClose,
  children,
}: {
  title: string;
  hint: string;
  lead: string;
  footer: ReactNode;
  locked?: boolean;
  onClose: () => void;
  children?: ReactNode;
}) {
  return (
    <InstanceWindow title={title} hint={hint} onClose={locked ? stay : onClose} footer={footer}>
      <p className="text-sm text-carbon-textSub">{lead}</p>
      {children}
    </InstanceWindow>
  );
}
