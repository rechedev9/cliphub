FROM node:24-bookworm AS web-dependencies
WORKDIR /src/web
RUN corepack enable && corepack prepare pnpm@11.22.0 --activate
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

FROM web-dependencies AS web-build
COPY web/ ./
ENV CLIPHUB_WEB_MODE=hosted
RUN pnpm run build

FROM node:24-bookworm-slim AS web-runtime
ENV NODE_ENV=production \
    CLIPHUB_WEB_MODE=hosted \
    HOSTNAME=0.0.0.0 \
    PORT=3000
WORKDIR /app
COPY --from=web-build --chown=node:node /src/web/.next/standalone ./
COPY --from=web-build --chown=node:node /src/web/.next/static ./.next/static
COPY --from=web-build --chown=node:node /src/web/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]

FROM golang:1.26.6-bookworm AS control-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/zv-control-plane ./cmd/zv-control-plane
COPY internal/controlplane ./internal/controlplane
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/zv-control-plane ./cmd/zv-control-plane

FROM debian:bookworm-slim AS control-runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 cliphub \
    && useradd --system --uid 10001 --gid cliphub --home-dir /data cliphub \
    && install -d -o cliphub -g cliphub -m 0700 /data
COPY --from=control-build /out/zv-control-plane /usr/local/bin/zv-control-plane
USER cliphub
EXPOSE 8090
ENTRYPOINT ["zv-control-plane"]
