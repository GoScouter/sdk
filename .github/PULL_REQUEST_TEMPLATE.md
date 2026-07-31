<!--
Thanks for contributing to the GoScouter SDK.

This package defines the contract between the host and every module written
against it. A change here can break module authors who never see this pull
request, so the compatibility sections below are the important ones.
-->

## Summary

<!-- What does this change do, and why? One or two paragraphs. -->

## Related issues

<!-- e.g. "Closes #123", "Part of #45". Write "None" if this isn't tracked. -->

## Type of change

<!-- Check every box that applies, and add the matching type/* label. -->

- [ ] Bug fix (`type/bug`)
- [ ] New feature (`type/feature`)
- [ ] Enhancement to existing behavior (`type/enhancement`)
- [ ] Refactor, no behavior change (`type/refactor`)
- [ ] Performance (`type/performance`)
- [ ] Documentation (`type/documentation`)
- [ ] Chore / maintenance (`type/chore`)

## Checklist

- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
      and the branch is named `<type>/<short-description>`
- [ ] `go vet ./...` and `staticcheck ./...` are clean
- [ ] `go mod tidy` leaves `go.mod` and `go.sum` unchanged
- [ ] `go test -race ./...` passes, and new behavior is covered by tests
- [ ] Exported identifiers have doc comments, and `doc.go` still describes the
      protocol accurately
- [ ] No new write to stdout on the module side, and no new required field on
      the wire without a fallback
- [ ] Applied the relevant `type/*` and `area/*` labels

