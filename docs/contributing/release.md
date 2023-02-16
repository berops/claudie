# How to release a new version of Claudie

The release process of Claudie consists of a few manual steps and a few automated steps.

## Manual steps

Whoever is responsible for creating a new release has to:

1. Write a new entry to a relevant [Changelog document](https://github.com/berops/claudie/tree/master/docs/CHANGELOG)
2. Add release notes to the Releases page
3. Publish a release

## Automated steps

After a new release is published, a [release pipeline](https://github.com/berops/claudie/blob/master/.github/workflows/release.yml) runs, which will:

1. Build new images tagged with the release tag
2. Push them to the container registry where anyone can pull them
3. Add Claudie manifest files to the release assets, with image tags referencing this release
