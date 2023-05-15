Our documentation is hosted on [GitHub Pages](https://pages.github.com/). Whenever a new push to **gh-pages** branch happens, it will deploy a new version of the documentation. 

The older versions of the documentation will still remain in the **gh-pages** branch, because we are supporting the docs versioning by [mike](https://github.com/jimporter/mike). That's also the reason, why we recommend makinng any changes and pushes to **gh-pages** branch with the usage of this tool.


To generate a new version you can run the command below.

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

You can find more [here](https://github.com/jimporter/mike).