# saucectl cypress & cucumber example

Example running saucectl with cypress & cucumber.

## What You'll Need

The steps below illustrate one of the quickest ways to get set up. If you'd like a more in-depth guide, please check out
our [documentation](https://docs.saucelabs.com/dev/cli/saucectl/#installing-saucectl).

_If you're using VS Code, you can use [Runme](https://marketplace.visualstudio.com/items?itemName=stateful.runme) to run the following commands directly from VS Code._

### Install `saucectl`

```shell
npm install -g saucectl
```

### Set Your Sauce Labs Credentials

```shell
saucectl configure
```

## Install Local NPM Dependencies

Run the following command inside the `examples/cucumber` folder :rocket:

```bash
npm install
```

## Running The Examples

Run the following command inside the `examples/cucumber` folder :rocket:

```bash
saucectl run
```

### Running With Tags

Run the following command inside the `examples/cucumber` folder :rocket:

```bash
saucectl run --env "CYPRESS_TAGS=@smoke"
```

### Generating JSON report

Specify [.cypress-cucumber-preprocessorrc.json](./.cypress-cucumber-preprocessorrc.json) and enable JSON report as follows. To get the JSON report, you should set the output file under `__assets__`.
Check out [here](https://github.com/badeball/cypress-cucumber-preprocessor/blob/master/docs/json-report.md) for more details.

```
"json": {
  "enabled": true,
  "output": "__assets__/<MY_CUCUMBER_REPORT>.json"
}
```

### Generating HTML report

Specify [.cypress-cucumber-preprocessorrc.json](./.cypress-cucumber-preprocessorrc.json) and enable HTML report as follows. To get the report, you should set the output file under `__assets__`.

```
"html": {
  "enabled": true,
  "output": "__assets__/<MY_CUCUMBER_REPORT>.html"
}
```

The HTML report is not displayed on the web UI but can be downloaded by configuring the `artifacts` setting in [.sauce/config.yml](.sauce/config.yml).

```
# Controls what artifacts to fetch when the suites have finished.
artifacts:
  download:
    when: always
    match:
      - "*.html"
    directory: ./artifacts/
```

## The Config

[Follow me](.sauce/config.yml) if you'd like to see how saucectl is configured for this example.

