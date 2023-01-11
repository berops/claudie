# How to release a new version of Claudie

Release process of Claudie consists of a few manual steps and a few automated steps.

## Manual steps

Whoever is responsible for creating a new release has to:

1. Write a new entry to a relevant [Changelog document](https://github.com/Berops/claudie/tree/master/docs/CHANGELOG)
2. Write a release notes on the Release page
3. Publish a release

## Automated steps

After a new release is published, a [release pipeline](https://github.com/Berops/claudie/blob/master/.github/workflows/release.yml) will run which will:

1. Build a new images with a release tag
2. Push them to the container registry where anyone can pull them
3. Append Claudie manifest files to the release, containing reference to the images from a release
