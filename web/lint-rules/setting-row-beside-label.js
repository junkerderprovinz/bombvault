// setting-row-beside-label: a one-line control stands beside its name.
//
// A dropdown, a segmented selector, a number field, a one-line text field and
// a time picker go in a SettingRow, which puts the label at the start of the
// row and the control at its end, so a card keeps one edge for names and one
// for controls. A list, a text area or a code editor needs the card's width and
// keeps its name above it, so a stack that holds one of those is left alone.
//
// What is flagged is a column (a `flex-col` label, or a `flex-col` box with a
// small gap) that holds a name and, below it, a one-line control. Boxes with a
// larger gap are layout columns for whole sections and are not read.
import { attrStringValue, baseUtility, classTokens, escapeHatch, getAttr, hasException, jsxName } from "./helpers.js";

const RULE_ID = "setting-row-beside-label";

/** Components that render one line: a dropdown, a field or a segmented strip. */
const ONE_LINE = new Set([
  "SelectField",
  "DropdownListbox",
  "NumberField",
  "NumberSetting",
  "RateField",
  "TextSetting",
  "RevealInput",
  "Selector",
  "TimePicker",
  "CompressionSelector",
]);

/** Components and tags that need the card's width. */
const WIDE = new Set([
  "textarea",
  "CadenceBuilder",
  "FolderBrowser",
  "RemoteFolderBrowser",
  "SelectionTree",
  "HooksEditor",
  "ListArea",
  "CodeEditor",
  "QRCode",
  "CopyBlock",
  "table",
  "ul",
  "ol",
]);

/** Rows that already stand a control beside its name. */
const ROWS = new Set(["SettingRow", "Rows", "ToggleRow"]);

/** input types that are not a line of text. */
const NOT_A_LINE = new Set(["checkbox", "radio", "range", "file", "hidden", "color", "submit", "button"]);

/** gap-1, gap-1.5 and gap-2 between a name and what it names. */
const SMALL_GAP = /^gap-(?:y-)?(?:0\.5|1|1\.5|2)$/;

function childElements(node) {
  const out = [];
  const visit = (n) => {
    if (!n) return;
    if (n.type === "JSXElement") out.push(n);
    else if (n.type === "JSXFragment") n.children.forEach(visit);
    else if (n.type === "JSXExpressionContainer") visit(n.expression);
    else if (n.type === "LogicalExpression") visit(n.right);
    else if (n.type === "ConditionalExpression") {
      visit(n.consequent);
      visit(n.alternate);
    }
  };
  node.children.forEach(visit);
  return out;
}

function isOneLine(node) {
  const name = jsxName(node);
  if (ONE_LINE.has(name)) return true;
  if (name === "select") return true;
  if (name === "input") {
    const type = attrStringValue(node, "type");
    return type === undefined || !NOT_A_LINE.has(type);
  }
  return false;
}

/** Does the subtree hold something that needs the card's width? */
function holdsWide(node) {
  return WIDE.has(jsxName(node)) || childElements(node).some(holdsWide);
}

/** The first one-line control in the subtree, found through boxes that do not
 *  stack their own content. */
function findControl(node, depth) {
  for (const child of childElements(node)) {
    if (isOneLine(child)) return child;
    if (ROWS.has(jsxName(child))) continue;
    const tokens = classTokens(child).map(baseUtility);
    const stacks = tokens.includes("flex-col") || tokens.includes("grid");
    if (depth > 0 && !stacks && jsxName(child) !== "label") {
      const found = findControl(child, depth - 1);
      if (found) return found;
    }
  }
  return undefined;
}

/** A name before the control: text, or a bubble that explains one. */
function hasName(node) {
  return node.children.some((c) => {
    if (c.type === "JSXText") return c.value.trim() !== "";
    if (c.type === "JSXExpressionContainer") return c.expression.type !== "JSXEmptyExpression" && !childElements({ children: [c] }).length;
    return false;
  }) || childElements(node).some((c) => ["span", "p", "label", "InfoBubble"].includes(jsxName(c)));
}

export default {
  meta: {
    type: "problem",
    docs: {
      description: "A one-line control stands beside its label in a SettingRow, never under it.",
    },
    schema: [],
    messages: {
      stacked:
        "This {{control}} stands under its name. Put the pair in a SettingRow so the name is at the start of the row and the control at its end; a list, a text area or a code editor is the only kind of field that keeps its name above it.{{hatch}}",
    },
  },

  create(context) {
    return {
      JSXElement(node) {
        const tokens = classTokens(node).map(baseUtility);
        if (!tokens.includes("flex-col")) return;
        const isLabel = jsxName(node) === "label";
        if (!isLabel && !tokens.some((t) => SMALL_GAP.test(t))) return;
        if (holdsWide(node)) return;
        if (!hasName(node)) return;
        const control = findControl(node, 3);
        if (!control) return;
        if (hasException(context, node.openingElement, RULE_ID)) return;
        const kind = jsxName(control);
        context.report({
          node: node.openingElement,
          messageId: "stacked",
          data: { control: kind === "input" ? "text field" : kind, hatch: escapeHatch(RULE_ID) },
        });
      },
    };
  },
};
