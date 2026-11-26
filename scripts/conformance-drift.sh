#!/usr/bin/env bash
#
# What the go.mod pin costs on next2, in conformance entries.
#
# The Carve 0.2 counterpart of corpus-drift.sh (read that file for the
# reasoning): drives spec/0.2/conformance through Convert twice, once with the
# pinned carve-go and once with carve-go `next2` HEAD, and fails only on the
# entries the pin renders wrongly that carve-go next2 renders correctly. The
# residual, entries that diverge with carve-go next2 too, is carve-go's embedded
# wasm trailing the spec and is printed as a warning.
#
# Requires CARVE_CONFORMANCE, pointing at <carve next2>/spec/0.2/conformance.

set -euo pipefail

if [ -z "${CARVE_CONFORMANCE:-}" ] || [ ! -d "${CARVE_CONFORMANCE}" ]; then
    echo "conformance-drift: CARVE_CONFORMANCE must name the spec/0.2/conformance directory." >&2
    echo "TestConformance skips without it, and two skipped runs compare as equal." >&2
    exit 2
fi

export CARVE_CORPUS_MAX_DIVERGENCE=100000

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

measure() {
    local label="$1"
    local out

    out="$(go test ./internal/convert/ -run 'TestConformance$' -v -count=1 | tee /dev/stderr)"

    printf '%s\n' "${out}" | sed -n 's/.*divergent-list=//p' | tail -1 \
        | tr ',' '\n' | sed '/^$/d' | sort > "${work}/${label}.divergent" || true
    printf '%s\n' "${out}" | grep -oE 'comparable=[0-9]+' | tail -1 | cut -d= -f2 \
        > "${work}/${label}.comparable" || true

    if [ ! -s "${work}/${label}.comparable" ]; then
        echo "conformance-drift: the ${label} run logged no comparable= count, so TestConformance did not execute." >&2
        exit 1
    fi
    if [ "$(cat "${work}/${label}.comparable")" -eq 0 ]; then
        echo "conformance-drift: the ${label} run compared 0 entries" >&2
        exit 1
    fi
}

echo "== the conformance corpus through Convert, with the go.mod pin =="
echo "go.mod requires $(grep -oE 'github\.com/markup-carve/carve-go v[^ ]+' go.mod | head -1 | awk '{print $2}')"
measure pinned

echo
echo "== the conformance corpus through Convert, with carve-go next2 =="
go get "github.com/markup-carve/carve-go@next2"
echo "replaced with $(grep -oE 'github\.com/markup-carve/carve-go v[^ ]+' go.mod | head -1 | awk '{print $2}')"
measure current

comparable="$(cat "${work}/pinned.comparable")"
pinned_count="$(wc -l < "${work}/pinned.divergent" | tr -d ' ')"
current_count="$(wc -l < "${work}/current.divergent" | tr -d ' ')"

echo
echo "pinned carve-go : ${pinned_count} of ${comparable} entries diverge"
echo "carve-go next2  : ${current_count} of ${comparable} entries diverge"

if [ "${current_count}" -gt 0 ]; then
    echo "::warning::${current_count} of ${comparable} conformance entries diverge with carve-go next2 too - that is carve-go's embedded wasm trailing the spec, not fixable here"
fi

comm -23 "${work}/pinned.divergent" "${work}/current.divergent" > "${work}/pin-only"
attributable="$(wc -l < "${work}/pin-only" | tr -d ' ')"
if [ "${attributable}" -gt 0 ]; then
    echo
    echo "entries the pin renders wrongly and carve-go next2 renders correctly:"
    sed 's/^/  /' "${work}/pin-only"
    echo "::error::the go.mod pin renders ${attributable} conformance entries wrongly that carve-go next2 renders correctly. Bump it: go get github.com/markup-carve/carve-go@next2"
    exit 1
fi

echo
echo "the pin costs no conformance entries against carve-go next2"
