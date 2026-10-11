import { useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { Badge } from "../../components/Badge";
import { InfoBubble } from "../../components/InfoBubble";
import { useDialogKeys } from "../../lib/useConfirm";

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
  const box = useRef<HTMLDivElement>(null);
  const leave = locked ? stay : onClose;
  useDialogKeys(true, box, leave);
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) leave();
      }}
    >
      <div className="relative w-full max-w-lg">
        <h2 className="flex items-center px-5">
          <Badge tone="heading" size="heading" wrap>
            {title}
            <InfoBubble tip={hint} onAccent />
          </Badge>
        </h2>
        <div
          ref={box}
          role="dialog"
          aria-modal="true"
          aria-label={title}
          className="glim-modal-card flex max-h-[90vh] w-full flex-col gap-4 overflow-y-auto rounded-card bg-carbon-surface p-5 pt-6 shadow-2xl"
        >
          <p className="text-sm text-carbon-textSub">{lead}</p>
          {children}
          <div className="flex flex-wrap items-center justify-end gap-2 pt-1">{footer}</div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
