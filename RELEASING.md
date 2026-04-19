# Releasing atlasent-sdk-go

This repo ships several Go modules from one tree. Tag names are
multi-module-aware, so the release dance is a few steps more than a
single-module repo.

## One-time per release

1. **Finish the branch.** CI green on every module. `CHANGELOG.md` has
   a new `[x.y.z]` section.

2. **Bump `atlasent.Version`** in `atlasent/client.go` if you haven't
   already. User-Agent is derived from it.

3. **Merge the PR to the default branch.**

## Tag in order

Tags live at the repo root. Main module gets a plain `vX.Y.Z`; each
submodule gets a path-prefixed tag at the same commit.

```sh
VER=v0.3.0
git checkout main && git pull

git tag -a "$VER"                            -m "atlasent-sdk-go $VER"
git tag -a "grpc/$VER"                       -m "grpc adapter $VER"
git tag -a "connectrpc/$VER"                 -m "connectrpc adapter $VER"
git tag -a "otel/$VER"                       -m "otel adapter $VER"
git tag -a "cacheredis/$VER"                 -m "cacheredis adapter $VER"
git tag -a "middleware/gin/$VER"             -m "gin middleware $VER"
git tag -a "middleware/echo/$VER"            -m "echo middleware $VER"
git tag -a "middleware/fiber/$VER"           -m "fiber middleware $VER"
git tag -a "atlasenttest/$VER"               -m "atlasenttest $VER"

git push origin "$VER" \
                "grpc/$VER" \
                "connectrpc/$VER" \
                "otel/$VER" \
                "cacheredis/$VER" \
                "middleware/gin/$VER" \
                "middleware/echo/$VER" \
                "middleware/fiber/$VER" \
                "atlasenttest/$VER"
```

Pushing `v$VER` triggers `.github/workflows/release.yml` which runs
goreleaser and drafts a GitHub Release.

## Update submodule requires

After the main module is tagged, open a follow-up PR:

1. In each `grpc/go.mod`, `connectrpc/go.mod`, etc., change
   ```
   require github.com/atlasent-systems-inc/atlasent-sdk-go v0.0.0
   ```
   to
   ```
   require github.com/atlasent-systems-inc/atlasent-sdk-go vX.Y.Z
   ```
   and **remove** the `replace` directive.

2. Run `go mod tidy` in each submodule.

3. Merge. Re-tag the submodules if necessary (they now resolve the
   main module from the registry instead of via local replace).

Downstream consumers can now `go get github.com/.../atlasent-sdk-go/grpc@vX.Y.Z`
without any replace-directive gymnastics.

## Rollback

If a release ships broken:

1. Mark the GitHub Release as pre-release (don't delete — module proxy
   has already seen the tag).
2. Publish a `vX.Y.(Z+1)` with the fix. Module proxies are append-only;
   there is no "unpublish".
3. Add a `retract vX.Y.Z` directive in `go.mod` on the next release so
   tooling warns on the broken version.
