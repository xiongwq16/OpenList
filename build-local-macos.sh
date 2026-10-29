#!/bin/sh
# Build the local macOS service binary without adding it to Git.
set -eu
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$repo_dir"
out_dir=output/local
mkdir -p "$out_dir"
tmp="$out_dir/.openlist.$$"
trap 'rm -f "$tmp"' EXIT HUP INT TERM
revision=$(git rev-parse --short=7 HEAD)
version=${OPENLIST_VERSION:-4.2.9}
web_version=${OPENLIST_WEB_VERSION:-$version}
go build -o "$tmp" -ldflags="-X github.com/OpenListTeam/OpenList/v4/internal/conf.Version=$version -X github.com/OpenListTeam/OpenList/v4/internal/conf.WebVersion=$web_version -X github.com/OpenListTeam/OpenList/v4/internal/conf.GitCommit=$revision" .
chmod 755 "$tmp"
mv -f "$tmp" "$out_dir/openlist"
trap - EXIT HUP INT TERM
printf 'Built %s\n' "$repo_dir/$out_dir/openlist"
