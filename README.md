saucectl
========
![build](https://github.com/saucelabs/saucectl-internal/workflows/saucectl%20pipeline/badge.svg?branch=master)

A command line interface for the Sauce Labs Testrunner Toolkit. This repository contains the Go binary that you use to kick off tests. If you look for more documentation on it, please have a look into our [example repo](https://github.com/saucelabs/testrunner-toolkit).

For information on how to contribute to `saucectl` please have a look into our [contribution guidelines](https://github.com/saucelabs/saucectl/blob/master/CONTRIBUTING.md).

## Requirements

- [Docker](https://docs.docker.com/get-docker/) installed
- Make sure the Docker daemon is running (e.g. `docker info` works in your terminal)

## Install

### cURL
```bash title="Using curl"
curl -L https://saucelabs.github.io/saucectl/install | bash
```

### NPM
```bash title="Using NPM"
npm install -g saucectl
```

### Homebrew
```bash title="Using Homebrew (macOS)"
brew tap saucelabs/saucectl
brew install saucectl
```

# FAQ
Please consult the [FAQ](https://github.com/saucelabs/testrunner-toolkit/blob/master/docs/FAQS.md) before using saucectl.

# Using `saucectl`

## The `new` Command
```sh
saucectl new
```

This command will ask you to choose one of the frameworks: 
- [Puppeteer](https://github.com/puppeteer/puppeteer)
- [Playwright](https://github.com/microsoft/playwright)
- [TestCafe](https://github.com/DevExpress/testcafe) 
- [Cypress](https://github.com/cypress-io/cypress) 

After that, a `./sauce/config.yml` file and an example test under
the `tests` directory will be created, where you can start working from.

## The `run` Command
```sh
saucectl run
```
This command will run the test based on the `./.sauce/config.yml` file.

## The `configure` Command
```sh
saucectl configure
```

This command ask you to type-in your SauceLabs username and access key.

You can also use the following command to do it in batch mode:

```sh
saucectl configure -u <MyUser> -a <MyAccessKey>
```

The credentials are store in `$HOME/.sauce/credentials.yml`

## The `signup` Command
```sh
saucectl signup
```

This command provides a link to sign up for a SauceLabs free trial account.

### Flags

#### `config`
```sh
saucectl run --config <path>
```
Using the `--config` flag will run the tests specified by that config file.

#### `env`
```sh
saucectl run --env <key1>=<value1> --env <key2>=<value2> ...
```
Using the `--env` flag will define environment variables that are then available
for use by the test framework.

#### `ci-build-id`
```sh
saucectl run --ci-build-id <value>
```
Using the `--ci-build-id` flag will override the build ID that is otherwise determined
based on the CI provider. The config file hash will still be used in addition to this
provided CI build ID.

#### `region`
```sh
saucectl run --region <region>
```
Using the `--region` flag will set the Sauce Labs region for the test execution.
The region corresponds to the available regions at saucelabs.com and affects
where your job information and assets are going to be stored.

#### `timeout`
```sh
saucectl run --timeout <seconds>
```
Using the `--timeout` flag will set the test timeout for the test runner framework. 

#### `suite`
```sh
saucectl run --suite <suite_name>
```
Using the `--suite` flag will only run specified suite by name.

### Private registry
In case you need to use an image from a private registry you can use environment variables for authentification;
```
export REGISTRY_USERNAME=registry-user
export REGISTRY_PASSWORD=registry-pass
```
and in your config.yml setup the image name to your registry like:
```
image:
  base: quay.io/saucelabs/stt-cypress-mocha-node
```

# Licensing
`saucectl` is licensed under the Apache License, Version 2.0. See [LICENSE](https://github.com/saucelabs/saucectl/blob/master/LICENSE) for the full license text.
