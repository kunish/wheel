## 1. Remove Tab Structure

- [x] 1.1 Remove Tabs/TabsList/TabsTrigger/TabsContent import from `settings/page.tsx`
- [x] 1.2 Replace `SettingsPage()` component: remove Tabs wrapper, render all sections vertically with `flex flex-col gap-6`

## 2. Wrap API Keys in Card

- [x] 2.1 Wrap `ApiKeysSection` content in `Card` > `CardHeader` + `CardContent`, with CardTitle "API Keys"

## 3. Verify

- [x] 3.1 Confirm all 4 sections render vertically on the page without tabs
- [x] 3.2 Confirm all existing interactions work (API key CRUD, account changes, export/import)
