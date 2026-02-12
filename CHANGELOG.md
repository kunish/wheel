# Changelog

## 1.0.0 (2026-02-12)


### Features

* add circuit breaker, session stickiness, and WS hub improvements ([f667bf1](https://github.com/kunish/wheel/commit/f667bf13c8a99717096c649c86285cfa9101b7b1))
* add data update animations and fix mobile layout ([b691a9d](https://github.com/kunish/wheel/commit/b691a9de8b7b0614f84a9b66d3704c3cc397d708))
* add drag-and-drop sorting for groups and group items ([82a329a](https://github.com/kunish/wheel/commit/82a329a2d0ab260f60594ab5dc70b9905f66c2c0))
* add i18n support with Chinese/English switching ([b05c542](https://github.com/kunish/wheel/commit/b05c542ba55b83f46ed7c6c168d6b76bc590bc7b))
* add i18n support with Chinese/English switching and misc improvements ([87264bf](https://github.com/kunish/wheel/commit/87264bf4edd0a32c8ec803ac866c3a60677aa946))
* add multi-platform Docker build support and docker:push script ([6a8045f](https://github.com/kunish/wheel/commit/6a8045f55d066e4a0c3857d6872a7fb4225b79cb))
* add Power Pipeline week view and Reactor Grid month view ([085a5ab](https://github.com/kunish/wheel/commit/085a5ab7b1625dc90442cd6d7cc442719822d2b9))
* add prev/next navigation and view-logs link to week/month/year views ([760df27](https://github.com/kunish/wheel/commit/760df272348e6f863442a19f35c9b7a69d13ee0a))
* add session-aware message deduplication in log detail view ([811ad47](https://github.com/kunish/wheel/commit/811ad4746ce49b0e30a179f1c61ae69b3479a0b8))
* enhance log message detail with request params, tools, response metadata and usage ([78a0c5b](https://github.com/kunish/wheel/commit/78a0c5b58cc195ec65da913f3aa8327199e2aa03))
* enhance log viewer, time range picker, and Docker x86 builds ([c33501c](https://github.com/kunish/wheel/commit/c33501cf4a8aa2211c7d3b66101cd7154da75457))
* improve logs page UX and channels page collapsibility ([db76a97](https://github.com/kunish/wheel/commit/db76a9729d2127bde73a6e41b798ba29b5bb8e15))
* increase default page size to 50 and remove log content truncation ([6a1258d](https://github.com/kunish/wheel/commit/6a1258d38f0ce8ca2ace0c5e7d80cb0850aca5fd))
* initial commit - Wheel LLM API gateway ([af10fe5](https://github.com/kunish/wheel/commit/af10fe54123475238bb4fb030245e91046f76643))
* merge Channels, Groups, and Prices into unified Model page ([693cb75](https://github.com/kunish/wheel/commit/693cb7525aa67cd7d6759687458d22f0efa035e2))
* migrate logs table to TanStack Table with tri-state sorting, TTFT sort, and grouping ([58ec60f](https://github.com/kunish/wheel/commit/58ec60f24e6de0c02477ce5537dd5dcf4e85687d))
* migrate worker to Go, web to Vite SPA, and remove dead code ([d68c057](https://github.com/kunish/wheel/commit/d68c057af9633d0c7a2382e3a7d6820534083d17))
* optimize DB writes with batched log writer and graceful shutdown ([966f6ed](https://github.com/kunish/wheel/commit/966f6edb896b28a39e0f9351bea6ad4cc502d931))
* optimize log page with channel navigation, upstream content, and merged messages tab ([a15cde4](https://github.com/kunish/wheel/commit/a15cde4aebd74c89fa5eb2dbcb61f8e2d4e1a6ab))
* polish UI with confirmation dialogs, loading states, and accessibility ([1aa7048](https://github.com/kunish/wheel/commit/1aa7048a29e495076ce610869f533ec289ceb682))
* real-time token and cost updates during streaming requests ([6dad4d0](https://github.com/kunish/wheel/commit/6dad4d07bb4331dc5506d5003da36c1e42939e8c))
* redesign day heatmap as 12-hour dual-ring gear clock ([399857d](https://github.com/kunish/wheel/commit/399857d3faaf082fb0d4b254fd84a040ca558645))
* redesign gear clock as Arc Reactor-style power core with data-driven rotation ([bd0ed0e](https://github.com/kunish/wheel/commit/bd0ed0e8b43f923e61ec1ba437234cf1a1fa01bc))
* redesign log detail Messages tab with conversation flow view ([9fd4dec](https://github.com/kunish/wheel/commit/9fd4deccdbc849f6741bf3795eae0827d219860e))
* replace Volcengine with OpenAI Responses, update README, add relay tests ([8c457f1](https://github.com/kunish/wheel/commit/8c457f10d4c94bfd4d5934b7728bd72e231a6926))
* show streaming requests in real-time in log list ([42bed1f](https://github.com/kunish/wheel/commit/42bed1fa51fa46b4e1adf8f91779a4f868c34b50))
* weekly heatmap with hourly breakdown and fix error truncation ([4619937](https://github.com/kunish/wheel/commit/4619937cd5250a7208c867ba25ab2d97f3848ee9))


### Bug Fixes

* add OpenAI SSE to Anthropic SSE streaming conversion ([f5f3419](https://github.com/kunish/wheel/commit/f5f3419aebd82588050b845b5c9d21ef73cd365b))
* add stop_reason/stop_sequence and input_tokens to Anthropic SSE message_start ([2c6de8a](https://github.com/kunish/wheel/commit/2c6de8ad51d0e07b26bb016bd9adb3956bd379b1))
* add stop_sequence field to Anthropic SSE message_delta events ([2f272cc](https://github.com/kunish/wheel/commit/2f272cc9bcbb5f5da6c4661a9b68ebc9d80e9a5d))
* cancel upstream request on client disconnect ([bc7c6a3](https://github.com/kunish/wheel/commit/bc7c6a39e831873553f9c2e739eaa81c677540dd))
* correct logo image path in README ([f25ee36](https://github.com/kunish/wheel/commit/f25ee36582ac24ed90b24aeedc07d1c08a625371))
* delay form reset until dialog close animation completes ([7851d1d](https://github.com/kunish/wheel/commit/7851d1dde8a54a2d93569dfed0763479742648e7))
* group item card showing wrong channel name for duplicate models ([89b7aac](https://github.com/kunish/wheel/commit/89b7aaccc312800fe0cd87aabd1034668ac3a404))
* harden backend security (SQL injection, DoS, timing attacks, concurrent writes) ([d176438](https://github.com/kunish/wheel/commit/d176438795c24606d40abde59f230a3971fb82cf))
* mobile dialog overflow and filter ghost channel from stats ([9c4f0ff](https://github.com/kunish/wheel/commit/9c4f0ff554c3a47e72d555d427f12becafe036d7))
* pin header and tabs in log detail panel, scroll content only ([b3b35ba](https://github.com/kunish/wheel/commit/b3b35ba2ab1c8a7e05c0cf744af3d99c2c05d5e0))
* prevent long model names from overflowing group dialog ([b5df57e](https://github.com/kunish/wheel/commit/b5df57e7566fada76cc96f0200f2a9bfe1eb0f0c))
* remove token fields from channel ranking to match backend API ([823a00a](https://github.com/kunish/wheel/commit/823a00a9ef39f1135a3653c1733cb33659c7f90c))
* replace findLast with reverse().find() for ES2022 compat ([49f8d8f](https://github.com/kunish/wheel/commit/49f8d8f403ec7afc4eced82850c4cd41d5cb4d99))
* replace Zap lightning icon with wheel icon matching favicon ([58226b6](https://github.com/kunish/wheel/commit/58226b69a749dced91d11d3d47e331ba385f26d7))
* resolve all 17 eslint warnings ([8f815d0](https://github.com/kunish/wheel/commit/8f815d0b72630f4ee8512afe167728cd6f21ad1b))
* return Anthropic-format error responses for Anthropic inbound requests ([962014f](https://github.com/kunish/wheel/commit/962014f10069af9cb11a2ffdf1f95498d240f2b5))
* revert layout auth check to two-pass render to prevent hydration mismatch ([3cd2a88](https://github.com/kunish/wheel/commit/3cd2a888ec7ed20fe88783e8e18e1b60ff1f7701))
* show full message content in log detail without truncation ([3a3fe9d](https://github.com/kunish/wheel/commit/3a3fe9dc4e59d5a61b8c823779d56fbae5ff1707))
* ui polish — scrollbar styling, layout overflow, and cleanup ([b28fe65](https://github.com/kunish/wheel/commit/b28fe659436bd1d1c6fff11c864fc0fd41b9a947))
* use cn() for className concatenation to avoid missing space ([7db8a31](https://github.com/kunish/wheel/commit/7db8a31489d6497fd0fa017ebc6fa352620d5856))
* use custom server instead of standalone mode to fix Docker startup ([edd57ac](https://github.com/kunish/wheel/commit/edd57ac85d880087f4b45bdf5627749b0c8808c2))
* use http.request upgrade proxy for WebSocket to fix invalid frame header ([32b4552](https://github.com/kunish/wheel/commit/32b45520af80a4d856e547059d43e4144f1d60a8))
* use React.Ref type for ModelCard ref prop ([db87334](https://github.com/kunish/wheel/commit/db873341a99bebac455961326a71ae63cdd552ad))
* use ws library for WebSocket proxy to fix invalid frame header ([592d11f](https://github.com/kunish/wheel/commit/592d11f8ca9b42cde3f0943d252e292adb5e31e8))


### Performance Improvements

* optimize dashboard loading with prefetch, lazy charts, and lighter animations ([83d893b](https://github.com/kunish/wheel/commit/83d893b3e82dbb6e081c32ceb355f4445c6230ba))
* optimize frontend with dynamic imports, code splitting, and memo ([a956571](https://github.com/kunish/wheel/commit/a9565719860e4f42dc3bcc29feeb619f3e84ff5f))
