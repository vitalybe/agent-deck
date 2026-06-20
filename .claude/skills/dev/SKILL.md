---
name: dev
description: >-
  The standard agent-deck development flow: implement a fix or feature on its
  own branch, build and test with the project's Go toolchain, commit with the
  house conventions, merge to main, and rebuild the binary. Use this whenever
  you're asked to fix a bug, add a feature, or make a code change in the
  agent-deck repo and carry it through to a merged, built result - especially
  when there are several independent changes that should each land as their own
  branch. Triggers on "/dev", and on requests like "fix X and merge it", "make
  this change and build", or "do these as separate branches".
---

# agent-deck dev flow

This skill captures how changes land in agent-deck: **one branch per logical
change, green build + tests before merge, house-style commits, auto-merge to
`main` with `--no-ff`, then rebuild the binary.** The goal is that every change
is isolated, verified, and honestly reported - not that you mechanically run a
checklist. Understand *why* each step exists and adapt when a situation doesn't
fit.

## Ground rules for this machine

- **Toolchain is flox.** The reproducible dev/test toolchain (Go, clang,
  golangci-lint, lefthook, tmux, gh, …) is provided by a [flox](https://flox.dev)
  environment defined in `.flox/env/manifest.toml`. `go` and friends are **not on
  PATH** outside it. Run build/test/vet commands inside the env:

  ```bash
  flox activate -- go build ./...
  flox activate -- go test ./internal/<pkg>/ -count=1
  ```

  Go specifics: nixpkgs has no 1.25.11, so the env ships a 1.26.x base and
  redirects via `GOTOOLCHAIN=go1.25.11` (set in the manifest's `[vars]`, so both
  `make` and lefthook's bare-`go` pre-push hook agree). The `go.mod` floor is
  `>=1.25.11`. Run `flox install <pkg>` to add a tool; don't `brew install` -
  Homebrew's go is broken here and the env is the source of truth.
- **Fallback `find-go.sh`**: if flox isn't available, resolve a `go` from the
  module cache once per session and reuse it:
  `GO=$(.claude/skills/dev/scripts/find-go.sh)`, then `"$GO" build ./...`. Prefer
  flox when present.
- **Shell is fish**; GNU coreutils are installed (use `rg`, `fd`). Avoid `cd` in
  compound commands (permission prompt) - prefer absolute paths.
- **`build/` is gitignored** - the rebuilt binary never shows up in `git status`.
- **Never push.** Everything stays local on `main`; the user pushes when ready.

## One change = one branch

Each logical fix/feature gets its own branch off `main`, even when the user asks
for several at once. This keeps merges atomic and the history readable.

```bash
git checkout main
git checkout -b fix/<short-slug>     # or feat/<short-slug>
```

Branch prefix mirrors the commit type: `fix/` for bug fixes, `feat/` for
features, `chore/` / `refactor/` otherwise. Slug is a few hyphenated words
(e.g. `fix/empty-groups-visible`).

When the work spans several independent changes, finish one branch end-to-end
(build → test → commit → merge), return to `main`, then start the next. Don't
interleave unrelated changes in one branch.

## Make the change

Implement it, matching surrounding code style. Where a change fixes a real
behavioral bug or adds logic with edge cases, **add a focused regression test**
in the affected package - the test should fail before your fix and pass after.

Watch for **global state in tests.** Things like `lipgloss.SetColorProfile`
mutate package-global state and leak across tests in the same package, breaking
unrelated tests downstream. Save and restore such state (`old := ...; t.Cleanup(
func(){ ...restore... })`).

## Build, vet, test

Run from the repo root inside the flox env:

```bash
flox activate -- go build ./...                 # whole module compiles
flox activate -- go vet ./internal/<pkg>/       # the package(s) you touched
flox activate -- go test ./internal/<pkg>/ -count=1   # affected package(s)
```

To run several commands in one shell, activate once: `flox activate` then run
them interactively. The `find-go.sh` fallback (`GO=$(...); "$GO" build ./...`)
is only for when flox is unavailable.

Start with a targeted test run (e.g. `-run 'Group|Render|Tool'`) for fast
feedback, then run the **full affected package** before merging so you don't
miss collateral breakage (golden/snapshot tests are sensitive to render
changes).

### Honest failure triage

If tests fail, find out whether *you* caused it before claiming anything. Stash
your work, run the same test on clean `main`, and compare:

```bash
git stash push -u -m wip
git checkout main
"$GO" test ./internal/<pkg>/ -run '<FailingTest>' -count=1
git checkout - && git stash pop
```

If it fails on `main` too, it's pre-existing and environmental - say so plainly
and move on. If it only fails with your change, it's yours - fix it. Report
outcomes truthfully: failing tests get surfaced with their output, skipped
steps get named.

## Commit (house style)

Multiline messages go through a temp file - never heredocs/`echo`/`printf` (see
the global CLAUDE.md rule):

1. Write the message to `/tmp/claude-<epoch-millis>.md`.
2. `git add -A && git commit -F /tmp/claude-<epoch>.md`

Message format:

- **Subject**: conventional commit - `type(scope): summary` in the imperative,
  e.g. `fix(ui): keep genuinely-empty group headers visible in active view`.
- **Body**: explain the *why* and the mechanism - what was wrong, what root
  cause you found, what the fix does, and any behavior preserved. Wrap prose.
- **Trailer** (always, last line):
  `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>`
- **Short dashes only** (`-`), never em dashes.

## Merge to main, then rebuild

Once the branch is green and committed, merge with a merge commit (no
fast-forward) so the branch boundary stays visible, then rebuild the binary:

```bash
git checkout main
git merge --no-ff <branch> -m "Merge branch '<branch>'"

VERSION=$(git describe --tags --always --dirty || echo dev)
flox activate -- go build -ldflags "-X main.Version=$VERSION" -o ./build/agent-deck ./cmd/agent-deck
```

The binary at `./build/agent-deck` is on the user's PATH, so rebuilding makes
the change available immediately (they restart their TUI to pick it up).

## Wrap-up report

After all branches are merged, give a tight summary: one line per change
(branch name + what it does), the verification status (build/vet/test green,
any pre-existing failures noted), and that the binary was rebuilt. Mention that
nothing was pushed. Offer to push or to clean up the merged local branches
(`git branch -d`) rather than doing it unprompted.

## Quick reference

| Step | Command |
| --- | --- |
| Toolchain | `flox activate` (or `flox activate -- <cmd>`) |
| Branch | `git checkout main && git checkout -b fix/<slug>` |
| Build | `flox activate -- go build ./...` |
| Test | `flox activate -- go test ./internal/<pkg>/ -count=1` |
| Commit | `git commit -F /tmp/claude-<epoch>.md` |
| Merge | `git merge --no-ff <branch> -m "Merge branch '<branch>'"` |
| Rebuild | `flox activate -- go build -ldflags "-X main.Version=$VERSION" -o ./build/agent-deck ./cmd/agent-deck` |
