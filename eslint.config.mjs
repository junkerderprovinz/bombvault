// ESLint 9 flat config. Next 16 removed `next lint`, so we invoke the ESLint CLI
// directly (see the "lint" script). eslint-config-next@16 ships flat-config
// arrays; eslint-config-prettier disables stylistic rules that conflict with
// Prettier (run separately). Mirrors the previous .eslintrc.json extends, and
// re-enables `no-console` / `no-explicit-any` so the codebase's existing
// eslint-disable directives stay meaningful (Next 16's flat preset drops them).
import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import prettier from "eslint-config-prettier";
import tseslint from "typescript-eslint";

const config = [
  { ignores: [".next/**", "node_modules/**", "data/**", "next-env.d.ts"] },
  ...nextCoreWebVitals,
  prettier,
  { rules: { "no-console": "warn" } },
  {
    files: ["**/*.ts", "**/*.tsx"],
    plugins: { "@typescript-eslint": tseslint.plugin },
    rules: { "@typescript-eslint/no-explicit-any": "warn" },
  },
];

export default config;
