# Documentation deployment

Our documentation is hosted on [GitHub Pages](https://pages.github.com/). Whenever a new push to **gh-pages** branch happens, it will deploy a new version of the doc. All the commits and pushes to this branch are automated through our release-docs.yml pipeline with the usage of [mike](https://github.com/jimporter/mike) tool.

That's also the reason, why we do not recommend making any manual changes in **gh-pages** branch. However, in case you have to, use the commands below.

## Generate a new version of the docs

- To create new version of docs

```sh
mike deploy <version>
```

- To deploy new version to live page

```sh
mike deploy <version> --push
```

- To make new version the default version when visiting the docs page

```sh
mike set-default <version>
```

## Deploy docs from some older GitHub tags

- Checkout to the desired tag

```sh
git checkout tags/<tag>
```

- Create new version of `mkdocs.yml`

> To find out how, follow the [mkdocs documentation](https://www.mkdocs.org/getting-started/#creating-a-new-project)

- Create python virtual environment

```sh
python3 -m venv ./venv
```

- Activate python virtual environment

```sh
source ./venv/bin/activate
```

- Install python requirements

```sh
pip install -r requirements.txt
```

- Deploy new version of docs

```sh
mike deploy <version> --push
```

## Deploy docs for a new release manually

In case the [release-docs.yml](https://github.com/berops/claudie/blob/master/.github/workflows/release-docs.yml) fails, you can deploy the new version manually by following this steps:

- Checkout to a new branch

```sh
git checkout tags/<release tag>
```

- Create python virtual environment

```sh
python3 -m venv ./venv
```

- Activate python virtual environment

```sh
source ./venv/bin/activate
```

- Install python requirements

```sh
pip install -r requirements.txt
```

- Deploy new version of docs

```sh
mike deploy <release tag> latest --push -u
```

> :warning: Don't forget to use the `latest` tag in the last command, because otherwise the new version will not be loaded as default one, when visiting [docs.claudie.io](docs.claudie.io) :warning:

Find more about how to work with [mike](https://github.com/jimporter/mike).
