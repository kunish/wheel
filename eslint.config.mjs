import antfu from "@antfu/eslint-config"

export default antfu(
  {
    type: "app",
    typescript: true,
    react: true,
    stylistic: false,
    ignores: ["**/dist/**"],
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
