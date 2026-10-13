import { useNavigate } from "react-router-dom";
import { Button } from "./Button";
import { IconBack } from "./glyphs";

/** BackButton leads from a page of one thing back to the list it belongs to.
 *  It stands in the page's head before the thing's mark, and the list's name
 *  is its tooltip. */
export function BackButton({ to, label }: { to: string; label: string }) {
  const navigate = useNavigate();
  return (
    <Button
      label={label}
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
