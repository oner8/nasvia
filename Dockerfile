# ---------- 1) 构建前端 ----------
FROM node:22-alpine AS web-builder

# npm 源（国内可传 --build-arg NPM_REGISTRY=https://registry.npmmirror.com）
ARG NPM_REGISTRY=https://registry.npmjs.org
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund --registry="${NPM_REGISTRY}"
COPY web/ ./
RUN npm run build

# ---------- 2) 构建后端（把前端产物内嵌进单二进制）----------
FROM golang:1.26-alpine AS go-builder

# Go 模块代理（国内可传 --build-arg GOPROXY=https://goproxy.cn,direct）
ARG GOPROXY=https://proxy.golang.org,direct
# 不指定 GOARCH：构建阶段与运行镜像同架构（本机构建或 buildx 按平台构建都成立），
# 旧版构建器（群晖 / 威联通自带的 Docker）不注入 TARGETARCH，写死回落值会在 ARM 机型上产出 amd64 二进制。
ENV CGO_ENABLED=0 GOOS=linux GOWORK=off GOPROXY=${GOPROXY}

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /build/web/dist ./web/dist
RUN go build -trimpath -ldflags="-s -w -X github.com/oner8/nasvia/internal/config.Version=1.0.0" -o /out/nasvia ./cmd/nasvia

# ---------- 3) 运行镜像 ----------
FROM alpine:3.21

# 容器以 root 启动，入口脚本把数据目录属主修正为 PUID:PGID 后用 su-exec 降权运行服务，
# 绑定宿主目录（群晖 / Unraid）无需手动改权限。
RUN apk add --no-cache ca-certificates tzdata su-exec \
    && mkdir -p /data \
    && chown 10001:10001 /data

COPY --from=go-builder /out/nasvia /usr/local/bin/nasvia
COPY deploy/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

WORKDIR /data
ENV NASVIA_PORT=3720 \
    NASVIA_DATA_DIR=/data \
    NASVIA_BIND=0.0.0.0 \
    PUID=10001 \
    PGID=10001

VOLUME ["/data"]
EXPOSE 3720

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${NASVIA_PORT:-3720}/api/health" || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
