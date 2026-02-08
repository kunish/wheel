import antfu from "@antfu/eslint-config"

export default antfu(
  {
    type: "app",
    typescript: true,
    react: true,
    nextjs: true,
    stylistic: false,
    ignores: ["**/dist/**", "**/.next/**", "**/drizzle/**"],
  },
  {
    files: ["apps/worker/src/index.node.ts", "apps/worker/src/runtime/node.ts"],
    rules: {
      "no-console": "off",
      "node/prefer-global/process": "off",
      "ts/no-require-imports": "off",
    },
  },
  {
    files: ["apps/web/**/*.ts", "apps/web/**/*.tsx"],
    rules: {
      "node/prefer-global/process": "off",
      "react-refresh/only-export-components": "off",
    },
  },
  {
    rules: {
      "no-alert": "off",
    },
  },
)
