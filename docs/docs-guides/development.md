# Development of the Claudie official docs

First of all, it is worth to mention, that we are using [MkDocs](https://www.mkdocs.org/) to generate HTML documents from markdown ones. To make our documentation prettier, we have used [Material theme](https://squidfunk.github.io/mkdocs-material/) for MkDocs. Regarding the version of our docs we are using [mike](https://github.com/jimporter/mike).

## How to run

First install the dependencies from **requirements.txt** in your local machine. However before doing that we recommend creating a virtual environment by running the command below.

```sh
python3 -m venv ./venv
```

After that you want to activate that newly create virtual environment by running:

```sh
source ./venv/bin/activate
```

Now, we can install the docs dependencies, which we mentioned before.

```sh
pip install -r requirements.txt
```

After successfull instalation, you can run command below, which generates HTML files for the docs and host in on your local server.

```sh
mkdocs serve
```

## How to test changes

Whenever you make some changes in docs folder or in mkdocs.yml file, you can see if the changes were applied as you expected by running the command below, which starts the server with newly generated docs.

```sh
mkdocs serve
```

:warning: Using this command you will not see the docs versioning, because we are using [mike](https://github.com/jimporter/mike) tool for this. :warning:

In case you want to test the docs versioning, you will have to run:

```sh
mike serve
```

Keep in mind, that [mike](https://github.com/jimporter/mike) takes the docs versions from **gh-pages** branch. That means, you will not be able to see your changes, in case you didn't run the command below before.

```sh
mike deploy <version>
```

:warning: Be careful, because this command creates a new version of the docs in your local **gh-pages** branch. :warning:
