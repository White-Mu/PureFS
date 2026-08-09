FROM golang:1.26-alpine AS backend-builder
WORKDIR /build
COPY PureFS-App/go.mod PureFS-App/go.sum ./
RUN go mod download
COPY PureFS-App/ .
RUN CGO_ENABLED=0 go build -o /purefsd ./cmd/purefsd

FROM node:22-alpine AS frontend-builder
WORKDIR /build
COPY PureFS-Web/package.json PureFS-Web/package-lock.json ./
RUN npm ci --registry=https://registry.npmmirror.com
COPY PureFS-Web/ .
RUN npm run build

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata curl
RUN addgroup -S purefs && adduser -S purefs -G purefs
WORKDIR /app
COPY --from=backend-builder /purefsd .
COPY --from=frontend-builder /build/dist ./web/dist
RUN mkdir -p /app/data && chown -R purefs:purefs /app
USER purefs
EXPOSE 8080
VOLUME ["/app/data"]
ENV PUREFS_WEB_ROOT=web/dist
HEALTHCHECK --interval=30s --timeout=5s CMD curl -sf http://localhost:8080/api/health || exit 1
CMD ["./purefsd"]
