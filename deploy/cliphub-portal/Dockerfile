# Build context is portal/, so this image never sees the Go tree, the Studio
# web app, or any local demo data.
FROM node:24-alpine AS builder
WORKDIR /build
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm run build

FROM node:24-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# output: "standalone" leaves static assets outside the traced bundle, so they
# are copied in beside it. db/migrations rides along because the container
# applies migrations itself on boot (drizzle-kit is a dev dependency and needs
# the TypeScript schema, so it cannot run here).
COPY --from=builder /build/.next/standalone ./
COPY --from=builder /build/.next/static ./.next/static
COPY --from=builder /build/db/migrations ./db/migrations

USER node
EXPOSE 3000
CMD ["node", "server.js"]
