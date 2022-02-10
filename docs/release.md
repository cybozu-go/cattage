Release procedure
=================

This document describes how to release a new version.

## Versioning

Follow [semantic versioning 2.0.0][semver] to choose the new version number.

## Prepare change log entries

Add notable changes since the last release to [CHANGELOG.md](CHANGELOG.md).
It should look like:

```markdown
(snip)
## [Unreleased]

### Added
- Implement ... (#35)

### Changed
- Fix a bug in ... (#33)

### Removed
- Deprecated `-option` is removed ... (#39)

(snip)
```

## Bump version

1. Determine a new version number.  Let it write `$VERSION` as `VERSION=x.y.z`.
2. Make a branch to release

    ```console
    $ git neco dev "bump-$VERSION"`
    ```

3. Update version strings in `version.go` in the top directory.
4. Edit `CHANGELOG.md` for the new version ([example][]).
5. Commit the change and push it.

    ```console
    $ git commit -a -m "Bump version to $VERSION"
    $ git neco review
    ```

6. Merge this branch.
7. Add a git tag to the main HEAD, then push it.

    ```console
    $ git checkout main
    $ git pull
    $ git tag -a -m "Release v$VERSION" "v$VERSION"
    $ git push origin "v$VERSION"
    ```

GitHub actions will build and push artifacts such as container images and
create a new GitHub release.

## Release Helm Chart

Cattage Helm Chart will be released independently of Cattage itself.
This will prevent the Cattage version from going up just by modifying the Helm Chart.

1. Determine a new version number of the chart. This version is not related to the version of Cattage. Let it write `$VERSION` as `VERSION=x.y.z`.
2. Make a branch to release

    ```console
    $ git neco dev "bump-chart-$VERSION"`
    ```

3. Change the version of `Chart.yaml`.
4. Commit the change and push it.

    ```console
    $ git commit -a -m "Bump chart version to $VERSION"
    $ git neco review
    ```

5. Merge this branch.
6. Add a git tag to the main HEAD, then push it.

    ```console
    $ git checkout main
    $ git pull
    $ git tag -a -m "Release Chart v$VERSION" "chart-v$VERSION"
    $ git push origin "chart-v$VERSION"
    ```

GitHub actions will upload the chart archive on `gh-pages` branch.

[semver]: https://semver.org/spec/v2.0.0.html
[example]: https://github.com/cybozu-go/etcdpasswd/commit/77d95384ac6c97e7f48281eaf23cb94f68867f79
