#!/usr/bin/env bash
#
# What the go.mod pin costs, in documents.
#
# This repository renders with an engine it never builds, two pins deep: go.mod
# pins a carve-go pseudo-version, and that carve-go embeds a prebuilt `.wasm`
# compiled from some carve-rs commit. A commit distance between any two of them
# is a proxy - 200 carve-rs commits can change nothing and one can change a
# construct - so this drives the mandatory spec corpus through Convert twice,
# once with the pinned carve-go and once with carve-go's current main, and
# reports the DIFFERENCE.
#
# That difference is the gate, and it is the only thing here that fails. It is
# also the only part this repository can act on: bumping go.mod clears it. The
# residual - documents that diverge with carve-go main too - is carve-go's
# embedded wasm trailing carve-rs, which markup-carve/carve-go's own corpus gate
# answers and which nothing in this repository can fix. Failing on it would put
# this script permanently red for someone else's lag, which is how a gate gets
# ignored, so it is printed as a warning and not asserted on.
#
# Lives in a script rather than inline in a workflow because two workflows run
# it: ci.yml on push and pull_request, where it blocks a merge, and
# engine-drift.yml on its schedule, where it catches the case that has no push
# here at all (carve-go publishes a new wasm and this pin becomes costly without
# a single commit in this repository).
#
# Requires CARVE_SPEC_CORPUS, pointing at <spec>/tests/corpus.

set -euo pipefail

if [ -z "${CARVE_SPEC_CORPUS:-}" ]; then
    echo "corpus-drift: CARVE_SPEC_CORPUS is unset." >&2
    echo "TestSpecCorpus SKIPS without it, and a skipped run logs no divergent" >&2
    echo "list, so the comparison below would compare two empty lists and report" >&2
    echo "that the pin costs nothing. That silent pass is the defect this script" >&2
    echo "exists to remove (hugo-carve#11), so an unset variable is an error." >&2
    exit 2
fi

if [ ! -d "${CARVE_SPEC_CORPUS}" ]; then
    echo "corpus-drift: CARVE_SPEC_CORPUS=${CARVE_SPEC_CORPUS} is not a directory" >&2
    exit 2
fi

# Report rather than fail on either individual run. The verdict is decided at
# the bottom, from the difference between the two, and a run that stopped early
# on its own tolerance could not be compared against anything.
export CARVE_CORPUS_MAX_DIVERGENCE=100000

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

# measure <label> - runs the corpus and records which documents diverged.
#
# Names only, and never a count alone: two runs can diverge on the same NUMBER
# of documents and on different ones, a construct the pin gets right traded
# against one it gets wrong, and subtracting counts would report that swap as
# "the pin costs nothing" - precisely the shape of check this exists to stop
# shipping.
measure() {
    local label="$1"
    local out

    # -count=1 so neither run can be answered from the test cache. The two runs
    # differ only in go.mod, which does key the cache, but relying on that is
    # relying on an implementation detail to make a measurement honest.
    out="$(go test ./internal/convert/ -run TestSpecCorpus -v -count=1 | tee /dev/stderr)"

    # `|| true` on both, and not as a shrug. Under `set -o pipefail` a grep that
    # matches nothing kills the script on the spot, with the shell's exit status
    # and no explanation - and "matched nothing" is exactly the state the guard
    # below exists to explain. Letting the extraction come back empty is what
    # lets that guard speak.
    printf '%s\n' "${out}" | sed -n 's/.*divergent-list=//p' | tail -1 \
        | tr ',' '\n' | sed '/^$/d' | sort > "${work}/${label}.divergent" || true
    printf '%s\n' "${out}" | grep -oE 'comparable=[0-9]+' | tail -1 | cut -d= -f2 \
        > "${work}/${label}.comparable" || true

    # The ablation. A skipped or filtered-out TestSpecCorpus logs neither
    # counter, and without this the empty divergent list it leaves behind reads
    # exactly like a clean run.
    if [ ! -s "${work}/${label}.comparable" ]; then
        echo "corpus-drift: the ${label} run logged no comparable= count, so" >&2
        echo "TestSpecCorpus did not execute - it skipped, or a -run filter" >&2
        echo "excluded it. Nothing was measured, so nothing here is a result." >&2
        exit 1
    fi
    if [ "$(cat "${work}/${label}.comparable")" -eq 0 ]; then
        echo "corpus-drift: the ${label} run compared 0 documents" >&2
        exit 1
    fi
}

# What THIS REPOSITORY does to a document, before anything about the pin.
#
# The comparison below is a difference between two runs that both go through
# Convert, so a defect in this package cancels out of it exactly: both runs
# render the same document wrongly, the set difference is empty, and the verdict
# at the bottom is "the pin costs no documents" while the damage is attributed
# to carve-go's wasm trailing carve-rs. Measured, not supposed - a one-line
# change to ConvertWithOptions put 873 of 1190 documents wrong and left this
# script exiting 0 with a warning, alongside a green `go test ./...`.
#
# So this runs first, and it is not a second conformance run: it compares
# Convert against the SAME linked engine, which makes it immune to engine lag
# and sensitive only to this package. If this repository is mangling documents,
# what the pin costs is not yet the interesting question.
echo "== what this repository does to a document, against the engine it holds =="
go test ./internal/convert/ -run TestConvertAddsNothingToTheEngine -v -count=1

echo
echo "== the corpus through Convert, with the go.mod pin =="
echo "go.mod requires $(grep -oE 'github\.com/markup-carve/carve-go v[^ ]+' go.mod | head -1 | awk '{print $2}')"
measure pinned

echo
echo "== the corpus through Convert, with carve-go main =="
go get "github.com/markup-carve/carve-go@main"
echo "replaced with $(grep -oE 'github\.com/markup-carve/carve-go v[^ ]+' go.mod | head -1 | awk '{print $2}')"
measure current

comparable="$(cat "${work}/pinned.comparable")"
pinned_count="$(wc -l < "${work}/pinned.divergent" | tr -d ' ')"
current_count="$(wc -l < "${work}/current.divergent" | tr -d ' ')"

echo
echo "pinned carve-go : ${pinned_count} of ${comparable} documents diverge"
echo "carve-go main   : ${current_count} of ${comparable} documents diverge"

if [ "${current_count}" -gt 0 ]; then
    echo "::warning::${current_count} of ${comparable} documents diverge with carve-go main too - that is carve-go's embedded wasm trailing carve-rs, answered by markup-carve/carve-go's own corpus gate and not fixable here"
fi

# A set difference, not a subtraction. See the comment on measure().
comm -23 "${work}/pinned.divergent" "${work}/current.divergent" > "${work}/pin-only"
attributable="$(wc -l < "${work}/pin-only" | tr -d ' ')"
if [ "${attributable}" -gt 0 ]; then
    echo
    echo "documents the pin renders wrongly and carve-go main renders correctly:"
    sed 's/^/  /' "${work}/pin-only"
    echo "::error::the go.mod pin renders ${attributable} documents wrongly that carve-go main renders correctly. Bump it: go get github.com/markup-carve/carve-go@main"
    exit 1
fi

echo
echo "the pin costs no documents against carve-go main"
