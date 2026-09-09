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

# libsql resolves its platform-specific native binding through a runtime
# require that Next's file tracer cannot follow, so the traced bundle ships
# libsql itself but not @libsql/linux-x64-musl, and the server dies on boot.
# Restoring the real dependency tree is what puts the binding back.
COPY --from=builder /build/node_modules/.pnpm ./node_modules/.pnpm

# Docker initializes a fresh named volume from the image directory it covers,
# ownership included. Creating these as node: here is what lets the unprivileged
# process write to them; without it SQLite fails to open with CANTOPEN.
RUN mkdir -p /data /uploads && chown node:node /data /uploads

USER node
EXPOSE 3000
CMD ["node", "server.js"]
