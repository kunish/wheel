FROM node:22-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/apps/web/node_modules ./apps/web/node_modules
COPY . .
RUN pnpm --filter @wheel/web build

FROM caddy:2-alpine AS runner

RUN addgroup -g 1001 -S app && adduser -S app -u 1001 -G app \
    && mkdir -p /data /config \
    && chown -R app:app /srv /data /config

COPY --from=builder --chown=app:app /app/apps/web/dist /srv
COPY --chown=app:app apps/web/Caddyfile /etc/caddy/Caddyfile

USER app
EXPOSE 3000
