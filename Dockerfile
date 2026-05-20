# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/go-camoufox ./cmd/go-camoufox

FROM debian:bookworm-slim AS runtime

ENV GO_CAMOUFOX_CACHE=/home/camoufox/.cache/go-camoufox
ENV DISPLAY=:99

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      bzip2 \
      curl \
      dbus-x11 \
      fontconfig \
      fonts-dejavu \
      fonts-liberation \
      libasound2 \
      libatk-bridge2.0-0 \
      libatk1.0-0 \
      libcairo2 \
      libcups2 \
      libdbus-1-3 \
      libdrm2 \
      libgbm1 \
      libgtk-3-0 \
      libnss3 \
      libpango-1.0-0 \
      libx11-6 \
      libxcb1 \
      libxcomposite1 \
      libxdamage1 \
      libxext6 \
      libxfixes3 \
      libxrandr2 \
      libxrender1 \
      libxtst6 \
      procps \
      xvfb \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --shell /usr/sbin/nologin camoufox \
    && mkdir -p /home/camoufox/.cache/go-camoufox \
    && chown -R camoufox:camoufox /home/camoufox

COPY --from=build /out/go-camoufox /usr/local/bin/go-camoufox
COPY docker/entrypoint.sh /usr/local/bin/go-camoufox-entrypoint
RUN chmod +x /usr/local/bin/go-camoufox-entrypoint

USER camoufox
WORKDIR /home/camoufox

EXPOSE 9222
ENTRYPOINT ["go-camoufox-entrypoint"]
CMD ["server", "--listen", "0.0.0.0:9222", "--headless", "--no-default-addons", "--os", "windows", "--i-know-what-im-doing"]
