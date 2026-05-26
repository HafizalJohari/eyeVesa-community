#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/HafizalJohari/eyeVesa-community.git}"
REPO_PATH="${REPO_PATH:-}"
INSTALL_DIR="${INSTALL_DIR:-}"
BIN_NAME="${BIN_NAME:-eyevesa}"

usage() {
  cat <<EOF
eyeVesa updater repair

Usage:
  REPO_PATH=/path/to/eyevesa-community bash scripts/repair-update.sh
  bash scripts/repair-update.sh --repo /path/to/eyevesa-community

Environment:
  REPO_URL     Git URL used when cloning a missing repo.
  REPO_PATH    Existing clone to repair, or clone target.
  INSTALL_DIR  Binary install directory. Defaults to the installed binary's dir,
               then /usr/local/bin when writable, then ~/.local/bin.
EOF
}

log() {
  printf '  %s\n' "$*"
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo|-r)
      [[ $# -ge 2 ]] || die "--repo requires a path"
      REPO_PATH="$2"
      shift 2
      ;;
    --install-dir)
      [[ $# -ge 2 ]] || die "--install-dir requires a path"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

need_cmd git
need_cmd go

find_repo() {
  if [[ -n "$REPO_PATH" ]]; then
    printf '%s\n' "$REPO_PATH"
    return
  fi

  if command -v "$BIN_NAME" >/dev/null 2>&1; then
    local bin_dir
    bin_dir="$(dirname "$(command -v "$BIN_NAME")")"
    local dir="$bin_dir"
    while [[ "$dir" != "/" ]]; do
      if [[ -d "$dir/.git" && ( -f "$dir/cli/go.mod" || -f "$dir/go.mod" ) ]]; then
        printf '%s\n' "$dir"
        return
      fi
      dir="$(dirname "$dir")"
    done
  fi

  for guess in \
    "$PWD" \
    "$HOME/eyevesa-community" \
    "$HOME/eyeVesa-community" \
    "$HOME/eyeVesa"; do
    if [[ -d "$guess/.git" && ( -f "$guess/cli/go.mod" || -f "$guess/go.mod" ) ]]; then
      printf '%s\n' "$guess"
      return
    fi
  done

  printf '%s\n' "$HOME/eyevesa-community"
}

repo_path="$(find_repo)"
repo_path="$(cd "$(dirname "$repo_path")" && pwd)/$(basename "$repo_path")"

if [[ ! -d "$repo_path/.git" ]]; then
  log "cloning $REPO_URL"
  git clone "$REPO_URL" "$repo_path"
fi

[[ -d "$repo_path/.git" ]] || die "not a git repo: $repo_path"

git_root="$(git -C "$repo_path" rev-parse --show-toplevel)"
repo_path="$git_root"

if [[ -f "$repo_path/cli/go.mod" ]]; then
  cli_path="$repo_path/cli"
elif [[ -f "$repo_path/go.mod" ]]; then
  cli_path="$repo_path"
else
  die "cannot find CLI go.mod under $repo_path"
fi

if [[ -z "$INSTALL_DIR" ]]; then
  if command -v "$BIN_NAME" >/dev/null 2>&1; then
    INSTALL_DIR="$(dirname "$(command -v "$BIN_NAME")")"
  elif [[ -w /usr/local/bin ]]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR"
install_path="$INSTALL_DIR/$BIN_NAME"

log "repo: $repo_path"
log "cli:  $cli_path"
log "bin:  $install_path"

log "fetching origin"
git -C "$repo_path" fetch --prune origin

default_branch=""
if head_ref="$(git -C "$repo_path" symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null)"; then
  default_branch="${head_ref#origin/}"
fi
if [[ -z "$default_branch" ]]; then
  for branch in main master; do
    if git -C "$repo_path" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
      default_branch="$branch"
      break
    fi
  done
fi
[[ -n "$default_branch" ]] || die "could not detect origin default branch"

status="$(git -C "$repo_path" status --porcelain)"
if [[ -n "$status" ]]; then
  die "working tree has local changes; commit or stash before repair"
fi

current_branch="$(git -C "$repo_path" branch --show-current || true)"
if [[ "$current_branch" != "$default_branch" ]]; then
  log "switching to $default_branch"
  if git -C "$repo_path" show-ref --verify --quiet "refs/heads/$default_branch"; then
    git -C "$repo_path" switch "$default_branch"
  else
    git -C "$repo_path" switch --create "$default_branch" --track "origin/$default_branch"
  fi
fi

log "fast-forwarding from origin/$default_branch"
git -C "$repo_path" merge --ff-only "origin/$default_branch"

commit="$(git -C "$repo_path" rev-parse --short HEAD)"
ldflags="-X github.com/hafizaljohari/eyeVesa/cli/cmd.version=$commit"

tmp_bin="$(mktemp)"
trap 'rm -f "$tmp_bin"' EXIT

log "building repaired updater"
(cd "$cli_path" && go build -ldflags "$ldflags" -o "$tmp_bin" .)
chmod +x "$tmp_bin"
mv "$tmp_bin" "$install_path"

log "installed $BIN_NAME at $install_path"
log "repaired to $commit"
