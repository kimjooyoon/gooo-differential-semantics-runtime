# Gooo differential semantics runtime

This public repository implements a small, executable Gooo language slice. The
authoritative meaning lives in [`.gooo/semantics.gooo`](.gooo/semantics.gooo): it
declares integer, boolean, and string values; `let` bindings; conditional
branches; pure function signatures; explicit effects; UNKNOWN records; and the
differential comparison fields. Go supplies the parser, reference executor,
Go emitter, generated-binary runner, and evidence writer.

The fixed corpus is declared by [`.gooo/corpus.gooo`](.gooo/corpus.gooo) and has
exactly four normal cases, two UNKNOWN cases, two REFUTED cases, and one replay
case. The UNKNOWN cases cover an undetermined external value and a missing
effect grant. The REFUTED cases cover a type mismatch and an intentionally
divergent generated effect trace.

## Execution contract

Every case produces a typed value (when available), an ordered effect trace,
and a deterministic terminal explanation digest. The CI reference interpreter
and a real generated Go 1.27 binary are compared field-by-field. A trace change
cannot be hidden by matching printed output. Status precedence is
`REFUTED > UNKNOWN > CLOSED`.

UNKNOWN results always preserve `stage`, `step`, `reason`, `unknown_class`,
`next_operation`, and `blocked_by`. Missing matched scenario/source/contract/
toolchain before-and-after integer evidence keeps improvement at `UNKNOWN`.

## Commands

```text
gooo-runtime reference --semantics .gooo/semantics.gooo --program corpus/normal-basic.gooo
gooo-runtime emit --semantics .gooo/semantics.gooo --program corpus/normal-basic.gooo --output caller-owned/main.go
gooo-runtime compare --case-id normal-basic --expected CLOSED --reference reference.json --generated generated.json --output comparison.json
gooo-runtime corpus --path .gooo/corpus.gooo
```

The emitter writes a generated entrypoint that imports only this module's
runtime package. CI places it in a caller-owned generated directory, builds
each binary, executes it, and uploads the source, binaries, outcomes,
comparisons, replay evidence, and exact integer measurements.

## Repository policy

The initial repository commit is the `gooo-repository-bootstrap` `v0.1.1`
bootstrap exception. Subsequent changes are PR-only and the active `main`
ruleset requires a pull request. GitHub Actions is the verification authority;
local test/build/vet/conformance runs are intentionally not used for release
claims. The root README is excluded from inventory measurements.
