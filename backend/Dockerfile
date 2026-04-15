# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
ARG NODE_VERSION=20

FROM node:${NODE_VERSION}-alpine AS frontend-builder
WORKDIR /workspace/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:${GO_VERSION}-alpine AS backend-builder
WORKDIR /workspace/backend

RUN apk add --no-cache git ca-certificates tzdata

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
  go build -trimpath -ldflags='-s -w' -o /out/aigateway-portal ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
  addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=backend-builder /out/aigateway-portal /app/aigateway-portal
COPY --from=frontend-builder /workspace/frontend/dist /app/web

ENV PORTAL_LISTEN_ADDR=:8081 \
    PORTAL_WEB_ROOT=/app/web

EXPOSE 8081
USER app
ENTRYPOINT ["/app/aigateway-portal"]
