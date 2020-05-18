saucectl
========
![build](https://github.com/saucelabs/saucectl-internal/workflows/saucectl%20pipeline/badge.svg?branch=master)
========

A command line interface for the Sauce Labs Testrunner Toolkit. This repository contains the Go binary that you use to kick off tests. If you look for more documentation on it, please have a look into our [example repo](https://github.com/saucelabs/testrunner-toolkit).

# Development Requirements

- [Go](https://golang.org/) (v1.14 or higher)
- [Homebrew](https://brew.sh/) (v2.2.13 or higher)

# Install

Run the following to install all dependencies:

```sh
$ make install
```

# Build

To build the project, run:

```sh
$ make build
```

# Test

To execute unit tests, run:

```sh
$ make test
```

# Licensing
`saucectl` is licensed under the Apache License, Version 2.0. See [LICENSE](https://github.com/saucelabs/saucectl/blob/master/LICENSE) for the full license text.
