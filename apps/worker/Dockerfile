FROM --platform=linux/amd64 node:22-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/worker/package.json apps/worker/
COPY packages/core/package.json packages/core/
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/apps/worker/node_modules ./apps/worker/node_modules
COPY --from=deps /app/packages/core/node_modules ./packages/core/node_modules
COPY . .
RUN pnpm --filter @wheel/core build
RUN cd apps/worker && npx tsc

FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup --system --gid 1001 appgroup && adduser --system --uid 1001 appuser

# Copy built output and migrations
COPY --from=builder /app/apps/worker/dist ./dist
COPY --from=builder /app/apps/worker/drizzle ./drizzle

# Copy node_modules from deps (includes workspace packages)
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/apps/worker/node_modules ./apps/worker/node_modules
COPY --from=builder /app/packages/core ./packages/core

# Data directory for SQLite
RUN mkdir -p /app/data && chown appuser:appgroup /app/data
VOLUME ["/app/data"]

USER appuser
EXPOSE 8787

# NODE_PATH so requires resolve from the monorepo node_modules
ENV NODE_PATH=/app/node_modules:/app/apps/worker/node_modules
CMD ["node", "dist/index.js"]
