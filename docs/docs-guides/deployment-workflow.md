Our documentation is hosted on [GitHub Pages](https://pages.github.com/). Whenever a new push to **gh-pages** branch happens, it will deploy a new version of the documentation. 

The older versions of the documentation will still remain in the **gh-pages** branch, because we are supporting the docs versioning by [mike](https://github.com/jimporter/mike). That's also the reason, why we recommend making any changes and pushes to **gh-pages** branch with the usage of this tool. 

We are also using [mike](https://github.com/jimporter/mike) in our [release pipeline](https://github.com/berops/claudie/blob/master/.github/workflows/release.yml), where we firstly set up git author, then we fetch changes from **gh-pages** branch and only after that we deploy the new version of the docs with the command below.

```
mike deploy ${RELEASE} latest --update-aliases --push
```

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

* git checkout tags/<tag> -b <branch>
* create mkdocs.yml
* mike deploy <version> --push

You can find more [here](https://github.com/jimporter/mike).