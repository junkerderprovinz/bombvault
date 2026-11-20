import template from "../../../deploy/docker-compose.node.yml?raw";
import type { NodeCompose } from "./api";

/** composeFile is deploy/docker-compose.node.yml with a node's values filled
 *  in. A double-quoted JSON string is a valid YAML scalar, so any name fits. */
export function composeFile(values: NodeCompose): string {
  return template.replace(/^(\s+)(APP_KEY|NODE_HOME|NODE_NAME): "".*$/gm, (_, indent: string, key: keyof NodeCompose) => `${indent}${key}: ${JSON.stringify(values[key])}`);
}
