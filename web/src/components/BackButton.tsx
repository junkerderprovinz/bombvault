import { useNavigate } from "react-router-dom";
import { useT } from "../lib/i18n";
import { Button } from "./Button";
import { IconBack } from "./glyphs";

/** BackButton leads from a page of one thing back to the list it belongs to.
 *  It stands in the page's head before the thing's mark, and says where it
 *  leads in its tooltip. `label` is the list's name. */
export function BackButton({ to, label }: { to: string; label: string }) {
  const { t } = useT();
  const navigate = useNavigate();
  return (
    <Button
      label={t("common.backTo").replace("{page}", () => label)}
      labelKey={null}
      glyph={<IconBack />}
      tone="neutral"
      variant="icon"
      glyphOnly
      onClick={() => navigate(to)}
      className="glim-btn-key flex-none rounded-control"
    />
  );
}
