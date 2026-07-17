# Cutting a release

Releases are cross-compiled locally — no CI required. The dependency tree is
pure Go, so one host builds every platform.

```bash
git tag v0.X.Y                      # on main, after the release PR merges
make release                        # dist/: 4 tarballs + SHA256SUMS
                                    # darwin/linux × amd64/arm64, static,
                                    # version stamped from the tag
```

Then publish the assets. On GitHub:

```bash
gh release create v0.X.Y dist/*.tar.gz dist/SHA256SUMS --title v0.X.Y --notes "..."
```

Or dogfood the CLI against a Forgejo-hosted copy of the repo:

```bash
forgejo release create <owner>/forgejo-cli --tag=v0.X.Y --title=v0.X.Y
for f in dist/*.tar.gz dist/SHA256SUMS; do
  forgejo release asset upload <owner>/forgejo-cli v0.X.Y --file="$f"
done
```

Notes:

- `make release` refuses nothing on a dirty tree but stamps `-dirty` into
  the version — only publish clean, tagged builds.
- `forgejo --version` on the shipped binary must print the tag.
- Add a platform by appending to `PLATFORMS` in the Makefile.
