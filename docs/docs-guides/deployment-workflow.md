Our documentation is hosted on [GitHub Pages](https://pages.github.com/). Whenever a new push to **gh-pages** branch happens, it will deploy a new version of the doc. All the commits and pushes to this branch are automated through our release-docs.yml pipeline with the usage of [mike](https://github.com/jimporter/mike) tool.

That's also the reason, why we do not recommend making any manual changes in **gh-pages** branch. However, in case you have to, use the commands below.

To generate a new version of the docs you can run the command below.

```
mike deploy <version>
```

If you would like to deploy that version to our production, you have to run:

```
mike deploy <version> --push
```

If you want to make that version the default one, you should run this command:

```
mike set-default <version>
```

In case you want to deploy a docs from some older GitHub tags to production, you will have to:

* `git checkout tags/<tag> -b <branch>`
* `create mkdocs.yml`
* `python3 -m venv ./venv`
* `source ./venv/bin/activate`
* `pip install -r requirements.txt`
* `mike deploy <version>` --push`

In case the [release-docs.yml](https://github.com/berops/claudie/blob/master/.github/workflows/release-docs.yml) fails, you can deploy the new version manually by following this steps:

* `git checkout -b <branch>`
* `python3 -m venv ./venv`
* `source ./venv/bin/activate`
* `pip install -r requirements.txt`
* `mike deploy <version> latest --push`

!!! Warning "Don't forget to use the `latest` tag in the last command, because otherwise the new version will not be loaded as default one, when visiting [docs.claudie.io](docs.claudie.io)"

Find more about how to work with [mike](https://github.com/jimporter/mike).