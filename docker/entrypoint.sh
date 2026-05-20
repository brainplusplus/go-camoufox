#!/usr/bin/env sh
set -eu

if [ "${GO_CAMOUFOX_FETCH_ON_START:-0}" = "1" ]; then
  if [ -n "${GO_CAMOUFOX_FETCH_VERSION:-}" ]; then
    go-camoufox fetch --version "$GO_CAMOUFOX_FETCH_VERSION"
  else
    go-camoufox fetch
  fi
fi

exec go-camoufox "$@"
