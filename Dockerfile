# syntax=docker/dockerfile:1.7

# Vue 前端单独构建；发布镜像必须包含 dist，否则 API 能启动但页面会返回 503。
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# 为 amd64/arm64 交叉编译同一个 Go 控制端，不引入 CGO 运行时依赖。
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS backend
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/easynettopo ./cmd/easynettopo

FROM alpine:3.22
RUN apk add --no-cache ca-certificates git openssh-client \
    && addgroup -S -g 10001 easynettopo \
    && adduser -S -D -H -u 10001 -G easynettopo -h /home/easynettopo easynettopo \
    && mkdir -p /data /home/easynettopo \
    && chown easynettopo:easynettopo /data /home/easynettopo
COPY --from=backend --chown=root:root /out/easynettopo /usr/local/bin/easynettopo
COPY --from=frontend --chown=root:root /src/web/dist /opt/easynettopo/web
USER 10001:10001
ENV HOME=/home/easynettopo \
    EASYNETTOPO_DATABASE_PATH=/data/app.db \
    EASYNETTOPO_WEB_DIR=/opt/easynettopo/web \
    EASYNETTOPO_LISTEN_ADDRESS=127.0.0.1:8080
WORKDIR /home/easynettopo
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/easynettopo"]
