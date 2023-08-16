# How to release a new version of Claudie

The release process of Claudie consists of a few manual steps and a few automated steps.

## Manual steps

Whoever is responsible for creating a new release has to:

1. Write a new entry to a relevant [Changelog document](https://github.com/berops/claudie/tree/master/docs/CHANGELOG)
2. Add release notes to the Releases page
3. Publish a release

## Automated steps

After a new release is published, a [release pipeline](https://github.com/berops/claudie/blob/master/.github/workflows/release.yml) and a [release-docs pipeline](https://github.com/berops/claudie/blob/master/.github/workflows/release-docs.yml) runs.

A [release pipeline](https://github.com/berops/claudie/blob/master/.github/workflows/release.yml) consist of the following steps:

1. Build new images tagged with the release tag
2. Push them to the container registry where anyone can pull them
3. Add Claudie manifest files to the release assets, with image tags referencing this release

A [release-docs pipeline](https://github.com/berops/claudie/blob/master/.github/workflows/release-docs.yml) consist of the following steps:

1. Check if there is a new Changelog file.
2. In case there is a new Changelog file:
    1. Checkout to a new feature branch
    2. Add reference to that new Changelog file in [mkdocs.yml](https://github.com/berops/claudie/blob/master/mkdocs.yml)
    3. Create PR to merges changes from new feature branch to master (PR need to be created due to `master` branch protection)
3. Deploy new version of docs on [docs.claudie.io](https://docs.claudie.io)