import i18n from "i18next"
import LanguageDetector from "i18next-browser-languagedetector"
import { initReactI18next } from "react-i18next"

import enCommon from "./locales/en/common.json"
import enDashboard from "./locales/en/dashboard.json"
import enLogin from "./locales/en/login.json"
import enLogs from "./locales/en/logs.json"
import enModel from "./locales/en/model.json"
import enSettings from "./locales/en/settings.json"

import zhCNCommon from "./locales/zh-CN/common.json"
import zhCNDashboard from "./locales/zh-CN/dashboard.json"
import zhCNLogin from "./locales/zh-CN/login.json"
import zhCNLogs from "./locales/zh-CN/logs.json"
import zhCNModel from "./locales/zh-CN/model.json"
import zhCNSettings from "./locales/zh-CN/settings.json"

export const defaultNS = "common"
export const ns = ["common", "login", "dashboard", "model", "logs", "settings"] as const

export const resources = {
  en: {
    common: enCommon,
    login: enLogin,
    dashboard: enDashboard,
    model: enModel,
    logs: enLogs,
    settings: enSettings,
  },
  "zh-CN": {
    common: zhCNCommon,
    login: zhCNLogin,
    dashboard: zhCNDashboard,
    model: zhCNModel,
    logs: zhCNLogs,
    settings: zhCNSettings,
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
