saucectl
========
![build](https://github.com/saucelabs/saucectl-internal/workflows/saucectl%20pipeline/badge.svg?branch=master)
========

A command line interface for the Sauce Labs Testrunner Toolkit. This repository contains the Go binary that you use to kick off tests. If you look for more documentation on it, please have a look into our [example repo](https://github.com/saucelabs/testrunner-toolkit).

For information on how to contribute to `saucectl` please have a look into our [contribution guidelines](https://github.com/saucelabs/saucectl/blob/master/CONTRIBUTING.md).

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

#### `parallel`
```sh
saucectl run --parallel=<true|false>
```
Using the `--parallel` flag allows the parallization of tests across machines to be
turned on/off. 

Saucectl will use CI provider specific clues from the environment to generate
a `build ID`. This `build ID` is used a grouping mechanism to synchronize the different
machines that are running in the same pipeline to distribute the tests. 

Saucectl currently uses the following CI environment variables to determine a build ID.

| CI            | Environment Variables          |
|:-------------:|:------------------------------:|
| GitHub        | GITHUB_WORKFLOW, GITHUB_RUN_ID |
| GitLab        | CI_PIPELINE_ID, CI_JOB_STAGE   |
| Jenkins       | BUILD_NUMBER                   |

If your CI provider is not listed here, you'll have to specify your own `build ID`.
Please consult the `ci-build-id` flag for this option.

#### `ci-build-id`
```sh
saucectl run --ci-build-id <value>
```
Using the `--ci-build-id` flag will override the build ID that is otherwise determined
based on the CI provider. 

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
