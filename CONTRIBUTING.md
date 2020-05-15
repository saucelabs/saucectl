# Contributing to Docker

Want to hack on saucectl? Awesome! This page contains information about reporting issues as well as some tips and guidelines useful to experienced open source contributors. Finally, make sure you read our [community guidelines](#docker-community-guidelines) before you start participating.

## Topics

* [Reporting Security Issues](#reporting-security-issues)
* [Design and Cleanup Proposals](#design-and-cleanup-proposals)
* [Reporting Issues](#reporting-other-issues)
* [Quick Contribution Tips and Guidelines](#quick-contribution-tips-and-guidelines)
* [Community Guidelines](#docker-community-guidelines)

## Reporting security issues

For information on handling security vulnerabilities, please see our [Reporting Security Issues](/SECURITY.md) documentation.

## Reporting other issues

A great way to contribute to the project is to send a detailed report when you encounter an issue. We always appreciate a well-written, thorough bug report, and will thank you for it!

Check that [our issue database](https://github.com/saucelabs/saucectl/issues) doesn't already include that problem or suggestion before submitting an issue. If you find a match, you can use the "subscribe" button to get notified on updates. Do *not* leave random "+1" or "I have this too" comments, as they only clutter the discussion, and don't help resolving it. However, if you have ways to reproduce the issue or have additional information that may help resolving the issue, please leave a comment.

When reporting issues, always include the output of `saucectl -v`.

Also include the steps required to reproduce the problem if possible and applicable. This information will help us review and fix your issue faster. When sending lengthy log-files, consider posting them as a gist (https://gist.github.com).

Don't forget to remove sensitive data from your logfiles before posting (you can replace those parts with "REDACTED").

## Quick contribution tips and guidelines

This section gives the experienced contributor some tips and guidelines.

### Pull requests are always welcome

Not sure if that typo is worth a pull request? Found a bug and know how to fix it? Do it! We will appreciate it. Any significant improvement should be documented as [a GitHub issue](https://github.com/docker/cli/issues) before anybody starts working on it.

We are always thrilled to receive pull requests. We do our best to process them quickly. If your pull request is not accepted on the first try, don't get discouraged! Our contributor's guide explains [the review process we use for simple changes](https://docs.docker.com/opensource/workflow/make-a-contribution/).

### Conventions

Fork the repository and make changes on your fork in a feature branch:

- If it's a bug fix branch, name it XXXX-something where XXXX is the number of the issue. 
- If it's a feature branch, create an enhancement issue to announce your intentions, and name it XXXX-something where XXXX is the number of the issue.

Submit unit tests for your changes. Go has a great test framework built in; use it! Take a look at existing tests for inspiration. [Run the full test suite](README.md) on your branch before submitting a pull request.

Update the documentation when creating or modifying features. Test your documentation changes for clarity, concision, and correctness, as well as a clean documentation build. See our contributors guide for [our style guide](https://docs.docker.com/opensource/doc-style) and instructions on [building the documentation](https://docs.docker.com/opensource/project/test-and-docs/#build-and-test-the-documentation).

Write clean code. Universally formatted code promotes ease of writing, reading, and maintenance. Always run `gofmt -s -w file.go` on each changed file before committing your changes. Most editors have plug-ins that do this automatically.

Pull request descriptions should be as clear as possible and include a reference to all the issues that they address.

Commit messages must start with a capitalized and short summary (max. 50 chars) written in the imperative, followed by an optional, more detailed explanatory text which is separated from the summary by an empty line.

Code review comments may be added to your pull request. Discuss, then make the suggested modifications and push additional commits to your feature branch. Post a comment after pushing. New commits show up in the pull request automatically, but the reviewers are notified only when you comment.

Pull requests must be cleanly rebased on top of master without multiple branches mixed into the PR.

**Git tip**: If your PR no longer merges cleanly, use `rebase master` in your feature branch to update your pull request rather than `merge master`.

After every commit, make sure the test suite passes. Include documentation changes in the same pull request so that a revert would remove all traces of the feature or fix.

Include an issue reference like `Closes #XXXX` or `Fixes #XXXX` in the pull request description that close an issue. Including references automatically closes the issue on a merge.

Please do not add yourself to the `AUTHORS` file, as it is regenerated regularly from the Git history.

## Coding Style

Unless explicitly stated, we follow all coding guidelines from the Go community. While some of these standards may seem arbitrary, they somehow seem to result in a solid, consistent codebase.

It is possible that the code base does not currently comply with these guidelines. We are not looking for a massive PR that fixes this, since that goes against the spirit of the guidelines. All new contributions should make a best effort to clean up and make the code base better than they left it. Obviously, apply your best judgement. Remember, the goal here is to make the code base easier for humans to navigate and understand. Always keep that in mind when nudging others to comply.

The rules:

1. All code should be formatted with `gofmt -s`.
2. All code should pass the default levels of [`golint`](https://github.com/golang/lint).
3. All code should follow the guidelines covered in [Effective Go](http://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
4. Comment the code. Tell us the why, the history and the context.
5. Document _all_ declarations and methods, even private ones. Declare expectations, caveats and anything else that may be important. If a type gets exported, having the comments already there will ensure it's ready.
6. Variable name length should be proportional to its context and no longer. noCommaALongVariableNameLikeThisIsNotMoreClearWhenASimpleCommentWouldDo`. In practice, short methods will have short variable names and globals will have longer names.
7. No underscores in package names. If you need a compound name, step back, and re-examine why you need a compound name. If you still think you need a compound name, lose the underscore.
8. No utils or helpers packages. If a function is not general enough to warrant its own package, it has not been written generally enough to be a part of a util package. Just leave it unexported and well-documented.
9. All tests should run with `go test` and outside tooling should not be required. No, we don't need another unit testing framework. Assertion packages are acceptable if they provide _real_ incremental value.
10. Even though we call these "rules" above, they are actually just guidelines. Since you've read all the rules, you now know that.

If you are having trouble getting into the mood of idiomatic Go, we recommend reading through [Effective Go](https://golang.org/doc/effective_go.html). The [Go Blog](https://blog.golang.org) is also a great resource. Drinking the kool-aid is a lot easier than going thirsty.