import i18n from "i18next"
import LanguageDetector from "i18next-browser-languagedetector"
import { initReactI18next } from "react-i18next"

import enApiReference from "./locales/en/api-reference.json"
import enBudgets from "./locales/en/budgets.json"
import enCommon from "./locales/en/common.json"
import enDashboard from "./locales/en/dashboard.json"
import enGuardrails from "./locales/en/guardrails.json"
import enKeys from "./locales/en/keys.json"
import enLogin from "./locales/en/login.json"
import enLogs from "./locales/en/logs.json"
import enMcp from "./locales/en/mcp.json"
import enModelLimits from "./locales/en/model-limits.json"
import enModel from "./locales/en/model.json"
import enPlayground from "./locales/en/playground.json"
import enSettings from "./locales/en/settings.json"
import enTags from "./locales/en/tags.json"
import enUsage from "./locales/en/usage.json"

import zhCNApiReference from "./locales/zh-CN/api-reference.json"
import zhCNBudgets from "./locales/zh-CN/budgets.json"
import zhCNCommon from "./locales/zh-CN/common.json"
import zhCNDashboard from "./locales/zh-CN/dashboard.json"
import zhCNGuardrails from "./locales/zh-CN/guardrails.json"
import zhCNKeys from "./locales/zh-CN/keys.json"
import zhCNLogin from "./locales/zh-CN/login.json"
import zhCNLogs from "./locales/zh-CN/logs.json"
import zhCNMcp from "./locales/zh-CN/mcp.json"
import zhCNModelLimits from "./locales/zh-CN/model-limits.json"
import zhCNModel from "./locales/zh-CN/model.json"
import zhCNPlayground from "./locales/zh-CN/playground.json"
import zhCNSettings from "./locales/zh-CN/settings.json"
import zhCNTags from "./locales/zh-CN/tags.json"
import zhCNUsage from "./locales/zh-CN/usage.json"

export const defaultNS = "common"
export const ns = [
  "common",
  "login",
  "dashboard",
  "model",
  "logs",
  "settings",
  "mcp",
  "keys",
  "model-limits",
  "playground",
  "usage",
  "budgets",
  "guardrails",
  "api-reference",
  "tags",
] as const

export const resources = {
  en: {
    common: enCommon,
    login: enLogin,
    dashboard: enDashboard,
    model: enModel,
    logs: enLogs,
    settings: enSettings,
    mcp: enMcp,
    keys: enKeys,
    "model-limits": enModelLimits,
    playground: enPlayground,
    usage: enUsage,
    budgets: enBudgets,
    guardrails: enGuardrails,
    "api-reference": enApiReference,
    tags: enTags,
  },
  "zh-CN": {
    common: zhCNCommon,
    login: zhCNLogin,
    dashboard: zhCNDashboard,
    model: zhCNModel,
    logs: zhCNLogs,
    settings: zhCNSettings,
    mcp: zhCNMcp,
    keys: zhCNKeys,
    "model-limits": zhCNModelLimits,
    playground: zhCNPlayground,
    usage: zhCNUsage,
    budgets: zhCNBudgets,
    guardrails: zhCNGuardrails,
    "api-reference": zhCNApiReference,
    tags: zhCNTags,
  },
} as const

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    defaultNS,
    ns: [...ns],
    fallbackLng: "en",
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["localStorage", "navigator"],
      lookupLocalStorage: "wheel-language",
      caches: ["localStorage"],
    },
  })

export default i18n
