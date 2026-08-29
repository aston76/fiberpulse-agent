FROM golang:1.26.7-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/fiberpulse-observatory ./cmd/fiberpulse-observatory

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system fiberpulse \
    && useradd --system --gid fiberpulse --home-dir /data fiberpulse \
    && install -d -o fiberpulse -g fiberpulse -m 0700 /data

COPY --from=build /out/fiberpulse-observatory /usr/local/bin/fiberpulse-observatory
USER fiberpulse
ENV FIBERPULSE_OBSERVATORY_ADDRESS=0.0.0.0:8090
ENV FIBERPULSE_OBSERVATORY_DATA_DIR=/data
EXPOSE 8090
VOLUME ["/data"]
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/fiberpulse-observatory"]
