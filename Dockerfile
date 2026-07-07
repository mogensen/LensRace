# Multi-stage build: Node builds the frontend, Go embeds it into the binary
# (mirrors `make build`, since Render's native runtimes are single-language
# and this repo needs both pnpm and go in the same build).

FROM node:22-alpine AS frontend
WORKDIR /app/frontend
# Must stay a major version whose lockfile format matches pnpm-lock.yaml's
# `lockfileVersion` (currently '9.0', written by pnpm 9/10) — pnpm 8 reads
# an older format and fails `--frozen-lockfile` with
# ERR_PNPM_LOCKFILE_BREAKING_CHANGE the moment the lockfile is regenerated
# by a newer pnpm, as happened when a dependency was added with pnpm 10.
RUN npm install -g pnpm@10
COPY frontend/package.json frontend/pnpm-lock.yaml ./
COPY frontend/scripts/ ./scripts/
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -tags embed_frontend -o /out/server .

FROM alpine:3.20
RUN apk add --no-cache su-exec && adduser -D -u 10001 app
WORKDIR /app
COPY --from=backend /out/server ./server
COPY entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh
EXPOSE 3000
ENTRYPOINT ["./entrypoint.sh"]
