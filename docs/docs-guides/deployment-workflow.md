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

* git checkout tags/<tag> -b <branch>
* create mkdocs.yml
* mike deploy <version> --push

Find more [here](https://github.com/jimporter/mike).