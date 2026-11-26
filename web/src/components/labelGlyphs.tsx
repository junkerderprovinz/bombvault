import type { ReactNode } from "react";
import type { LabelMode } from "../lib/controls";
import { IconLabelGlyph, IconLabelReactive, IconLabelText, IconLabelTextGlyph } from "./glyphs";

/** LABEL_GLYPHS gives every label mode its glyph, which a selector of them
 *  shows alone where the words do not fit. */
export const LABEL_GLYPHS: Record<LabelMode, ReactNode> = {
  text: <IconLabelText />,
  textGlyph: <IconLabelTextGlyph />,
  glyph: <IconLabelGlyph />,
  reactive: <IconLabelReactive />,
};
