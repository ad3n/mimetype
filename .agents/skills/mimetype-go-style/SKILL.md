---
name: mimetype-go-style
description: Apply the repository's Go syntax and control-flow conventions when writing, refactoring, or reviewing handwritten Go code. Use for Go changes in this repository; do not apply these rules to generated code or external sources.
---

# Apply Mimetype Go Style

Keep control flow linear and make the successful path easy to scan without changing behavior.

## Control flow

- Avoid `else`. Handle invalid input, errors, exceptional states, and completed cases first with a guard clause and early `return`, `continue`, or `break`.
- Do not mechanically remove an `else` when doing so duplicates work, weakens variable scope, changes evaluation order, or makes correctness harder to verify. Retain it only when those constraints make it clearer or necessary.
- Prefer positive success-path code after guards. Keep conditions small; extract a well-named predicate only when it removes real complexity or duplication.
- Avoid deep nesting. In loops, reject or finish the current item early with `continue` or `break` instead of wrapping the remaining body in another conditional.
- Preserve error identity, side-effect order, cleanup, locking, and return values when flattening branches.

## Syntax and spacing

- Add one blank line after a completed control-flow block and after each `defer`, except before `case`, `default`, a closing brace, or where Go syntax requires adjacency.
- Register `defer` immediately after successful resource acquisition when ownership lasts until function return. Release locks and loop-local resources as soon as progress requires.
- Use unnamed parameters when every parameter is intentionally unused and an interface requires the signature. Do not retain unnecessary named or underscore parameters.
- Do not add a standalone blank assignment merely to discard a result; call a side-effecting function directly when Go permits it.
- Prefer modern standard-library and language idioms such as `copy`, `bytes.Clone`, `maps.Copy`, integer range, iterator APIs, and `for b.Loop()` when they improve clarity and are supported by the module's declared Go version.
- Apply these conventions manually, then run `gofmt` on changed Go files.

## Review and verification

- Inspect the complete changed function after restructuring it; do not judge an isolated branch without its surrounding ownership and side effects.
- Keep a necessary `else` and state why in review notes when flattening would make the implementation less correct or less clear.
- Run focused tests for affected behavior, followed by `go test ./...`, `go vet ./...`, and `git diff --check` when the repository and environment support them.
