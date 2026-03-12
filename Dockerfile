# syntax=docker/dockerfile:1

# ---------- build stage ----------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# モジュールキャッシュを先にコピーして依存解決を高速化
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /m3bridge .

# ---------- runtime stage ----------
FROM alpine:3.21

# 証明書バンドルが必要（Microsoft認証エンドポイントへのTLS接続のため）
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /m3bridge /usr/local/bin/m3bridge

# 設定・トークンキャッシュは /data にマウント
VOLUME ["/data"]

# デフォルトのSMTPポート
EXPOSE 2525

ENTRYPOINT ["m3bridge"]
CMD ["serve"]
