# Single-binary deploy: cgo (mattn/go-sqlite3) needs gcc at build time
# and glibc at runtime, so builder is golang/bookworm and runtime is
# debian-slim (not scratch/alpine). SQLite DB lives in /data.
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -o /loom ./cmd/loom

FROM debian:bookworm-slim
RUN mkdir -p /data /web
COPY --from=build /loom /loom
COPY --from=build /src/web /web
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/loom", "--addr", ":8080", "--data", "/data/loom.db", "--web", "/web"]
