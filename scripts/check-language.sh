#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ALLOWLIST_FILE=".language-guard-allowlist"
ALLOWLIST_REGEX_FILE=""

cleanup() {
  if [[ -n "$ALLOWLIST_REGEX_FILE" && -f "$ALLOWLIST_REGEX_FILE" ]]; then
    rm -f "$ALLOWLIST_REGEX_FILE"
  fi
}
trap cleanup EXIT

INCLUDE_GLOBS=(
  "--glob" "*.go"
  "--glob" "*.md"
  "--glob" "*.yml"
  "--glob" "*.yaml"
)

EXCLUDE_GLOBS=(
  "--glob" "!.orch/**"
  "--glob" "!node_modules/**"
  "--glob" "!vendor/**"
  "--glob" "!.git/**"
)

prepare_allowlist() {
  if [[ ! -f "$ALLOWLIST_FILE" ]]; then
    return 0
  fi

  ALLOWLIST_REGEX_FILE="$(mktemp)"
  while IFS= read -r line; do
    # Ignore comments and blank lines.
    if [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]]; then
      continue
    fi
    printf '%s\n' "$line" >> "$ALLOWLIST_REGEX_FILE"
  done < "$ALLOWLIST_FILE"

  # If the allowlist has no effective patterns, skip filtering.
  if [[ ! -s "$ALLOWLIST_REGEX_FILE" ]]; then
    rm -f "$ALLOWLIST_REGEX_FILE"
    ALLOWLIST_REGEX_FILE=""
  fi
}

filter_allowlisted() {
  local input="$1"
  if [[ -z "$ALLOWLIST_REGEX_FILE" ]]; then
    printf '%s\n' "$input"
    return 0
  fi

  printf '%s\n' "$input" | rg -v -f "$ALLOWLIST_REGEX_FILE" || true
}

run_check() {
  local check_name="$1"
  local pattern="$2"

  local matches
  matches="$(rg -n "$pattern" "${INCLUDE_GLOBS[@]}" "${EXCLUDE_GLOBS[@]}" || true)"
  if [[ -z "$matches" ]]; then
    return 0
  fi

  local filtered
  filtered="$(filter_allowlisted "$matches")"
  if [[ -z "$filtered" ]]; then
    return 0
  fi

  printf '%s\n' "$filtered"
  echo
  echo "Error: ${check_name}. Repository language must be English."
  exit 1
}

prepare_allowlist

echo "Checking for non-English Turkish characters..."
run_check "Turkish characters detected" "[çğıöşüÇĞİÖŞÜ]"

echo "Checking for common Turkish words..."
run_check "Common Turkish words detected" "\\b(gorev|gorevi|calis|basari|yukle|dosya|degis|kullanici|konfigurasyon|ornek|adim|incele|uret|hata|tamamlandi|cikti|dizin|olustur|yazildi|bulunmuyor|sadece|goster|testler|parametreler)\\b"

echo "Language check passed."
