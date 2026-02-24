## 1. File Structure Reorganization

- [x] 1.1 Rename `pages/channels.tsx` to `pages/model.tsx` and update the default export name from `ChannelsAndGroupsPage` to `ModelPage`
- [x] 1.2 Rename `pages/channels/` directory to `pages/model/` (includes `channel-dialog.tsx`, `channel-model-picker-dialog.tsx`, `group-dialog.tsx`, `model-picker-dialog.tsx`)
- [x] 1.3 Create `pages/model/price-dialog.tsx` — extract `PriceForm` and price dialog logic from `pages/prices.tsx`
- [x] 1.4 Delete `pages/prices.tsx`
- [x] 1.5 Update `pages/groups.tsx` redirect target from `/channels` to `/model`

## 2. i18n Translation Updates

- [x] 2.1 Create `i18n/locales/en/model.json` merging keys from `channels.json` and `prices.json`, update `pageTitle` to "Model", add `nav.model` key
- [x] 2.2 Create `i18n/locales/zh-CN/model.json` merging keys from `channels.json` and `prices.json`, update `pageTitle` to "模型", add `nav.model` key
- [x] 2.3 Delete `i18n/locales/en/channels.json` and `i18n/locales/en/prices.json`
- [x] 2.4 Delete `i18n/locales/zh-CN/channels.json` and `i18n/locales/zh-CN/prices.json`
- [x] 2.5 Update `common.json` (both locales): change `nav.channels` to `nav.model` ("Model" / "模型"), remove `nav.prices`
- [x] 2.6 Update i18n config (`i18n/index.ts` or equivalent) to load `model` namespace instead of `channels` and `prices`

## 3. Route and Navigation Updates

- [x] 3.1 Update `routes.tsx`: replace `/channels` route with `/model`, remove `/prices` route, add redirects for `/channels` and `/prices` to `/model`
- [x] 3.2 Update `routes.tsx`: update lazy import from `pages/channels` to `pages/model`, remove `PricesPage` import
- [x] 3.3 Update `protected-layout.tsx`: change `navOrder` from `["/dashboard", "/channels", "/prices", "/logs", "/settings"]` to `["/dashboard", "/model", "/logs", "/settings"]`
- [x] 3.4 Update `app-layout.tsx`: change `navItemDefs` — replace channels entry with `{ href: "/model", labelKey: "nav.model", icon: Boxes }`, remove prices entry
- [x] 3.5 Update `app-layout.tsx`: replace `Radio` and `DollarSign` icon imports with `Boxes` from lucide-react

## 4. Price Integration into Model Page

- [x] 4.1 In `pages/model.tsx`, add TanStack Query for `model-prices` and `price-update-time` data
- [x] 4.2 In `pages/model.tsx`, add price management toolbar buttons (Sync Prices, Add Price) alongside existing Models button, with last sync time display
- [x] 4.3 In `pages/model.tsx`, add sync mutation (`syncModelPrices`), create mutation (`createModelPrice`), and wire up the price dialog
- [x] 4.4 Build a `priceMap` (Map<string, {inputPrice, outputPrice}>) from price query data and pass to Channel/Group components

## 5. ModelCard Price Display

- [x] 5.1 Update `ModelCard` component props to accept optional `price` data (`{ inputPrice: number, outputPrice: number }`)
- [x] 5.2 Render compact price info in ModelCard when price data is provided (e.g., "↓0.15 ↑0.60")
- [x] 5.3 Add `onPriceClick` callback prop to ModelCard for opening the edit price dialog
- [x] 5.4 Pass `priceMap` lookup results to `DraggableModelTag` and `GroupItemList` render functions

## 6. Price Edit/Delete in Model Page

- [x] 6.1 Import and wire `PriceDialog` (from `pages/model/price-dialog.tsx`) in `pages/model.tsx` for editing prices
- [x] 6.2 Add delete price mutation and confirmation dialog in `pages/model.tsx`
- [x] 6.3 Update the model list dialog to show pricing info alongside each model name

## 7. Update Internal References

- [x] 7.1 Search codebase for any imports or links referencing `/channels` or `pages/channels` and update to `/model` or `pages/model`
- [x] 7.2 Search for any references to `prices` translation namespace and update to `model`
- [x] 7.3 Verify all `useTranslation("channels")` calls are updated to `useTranslation("model")`

## 8. Verification

- [x] 8.1 Verify the app builds without errors (`pnpm build`)
- [x] 8.2 Verify `/model` route loads correctly with Channels, Groups, and Price management
- [x] 8.3 Verify `/channels`, `/prices`, `/groups` all redirect to `/model`
- [x] 8.4 Verify bottom navigation shows 4 items with correct labels and active states
- [x] 8.5 Verify price sync, add, edit, delete all work from the Model page
- [x] 8.6 Verify ModelCard displays prices inline when available
- [x] 8.7 Verify both EN and ZH-CN translations render correctly
