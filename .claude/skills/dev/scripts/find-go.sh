#!/usr/bin/env bash
# Prints an absolute path to a usable `go` binary, or exits non-zero.
#
# On this machine `go` is often missing from PATH (Homebrew's go is broken) but
# a Go toolchain lives in the module cache. Prefer a real `go` on PATH; fall
# back to the newest cached toolchain.
set -euo pipefail

if command -v go >/dev/null 2>&1; then
	command -v go
	exit 0
fi

# Newest toolchain in the module cache (version-sorted).
shopt -s nullglob
candidates=("$HOME"/go/pkg/mod/golang.org/toolchain@*/bin/go)
if ((${#candidates[@]})); then
	printf '%s\n' "${candidates[@]}" | sort -V | tail -1
	exit 0
fi

echo "no go binary found (PATH or module-cache toolchain)" >&2
exit 1
