# Azure Linux Image Tools' Contribution Guide

## Table of Contents

Please use the [auto-generated table of contents](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-readmes#auto-generated-table-of-contents-for-readme-files) GitHub creates. To reveal it, select the three bar menu icon at the top of the page.

## Contributing License Agreement

This project welcomes contributions and suggestions. Most contributions require you to
agree to a Contributor License Agreement (CLA) declaring that you have the right to,
and actually do, grant us the rights to use your contribution. For details, visit
<https://cla.microsoft.com>.

When you submit a pull request, a CLA-bot will automatically determine whether you need
to provide a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the
instructions provided by the bot. You will only need to do this once across all repositories using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/)
or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Security Vulnerabilities

<!-- BEGIN MICROSOFT SECURITY.MD V0.0.3 BLOCK -->

### Security

Microsoft takes the security of our software products and services seriously, which includes all source code repositories managed through our GitHub organizations, which include [Microsoft](https://github.com/Microsoft), [Azure](https://github.com/Azure), [DotNet](https://github.com/dotnet), [AspNet](https://github.com/aspnet), [Xamarin](https://github.com/xamarin), and [our GitHub organizations](https://opensource.microsoft.com/).

If you believe you have found a security vulnerability in any Microsoft-owned repository that meets Microsoft's [Microsoft's definition of a security vulnerability](https://docs.microsoft.com/en-us/previous-versions/tn-archive/cc751383(v=technet.10)) of a security vulnerability, please report it to us as described below.

### Reporting Security Issues

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them to the Microsoft Security Response Center (MSRC) at [https://msrc.microsoft.com/create-report](https://msrc.microsoft.com/create-report).

If you prefer to submit without logging in, send email to [secure@microsoft.com](mailto:secure@microsoft.com).  If possible, encrypt your message with our PGP key; please download it from the the [Microsoft Security Response Center PGP Key page](https://www.microsoft.com/en-us/msrc/pgp-key-msrc).

You should receive a response within 24 hours. If for some reason you do not, please follow up via email to ensure we received your original message. Additional information can be found at [microsoft.com/msrc](https://www.microsoft.com/msrc).

Please include the requested information listed below (as much as you can provide) to help us better understand the nature and scope of the possible issue:

* Type of issue (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
* Full paths of source file(s) related to the manifestation of the issue
* The location of the affected source code (tag/branch/commit or direct URL)
* Any special configuration required to reproduce the issue
* Step-by-step instructions to reproduce the issue
* Proof-of-concept or exploit code (if possible)
* Impact of the issue, including how an attacker might exploit the issue

This information will help us triage your report more quickly.

If you are reporting for a bug bounty, more complete reports can contribute to a higher bounty award. Please visit our [Microsoft Bug Bounty Program](https://microsoft.com/msrc/bounty) page for more details about our active programs.

### Preferred Languages

We prefer all communications to be in English.

### Policy

Microsoft follows the principle of [Coordinated Vulnerability Disclosure](https://www.microsoft.com/en-us/msrc/cvd).

<!-- END MICROSOFT SECURITY.MD BLOCK -->

## Development

We welcome contributions. When contributing, please adhere to `golang` formatting as described by the [fmt](https://pkg.go.dev/fmt) package. To format using this package, you can run `make go-tidy-all`.

### Documentation

We welcome documentation improvements. See [toolkit/docs](toolkit/docs) for the latest documentation.

## Pull Request Guidelines

Please direct pull requests to  `main` branch.

### Branch structure

An overview of how the branches are structured can be seen below

| Branch / Tag | For PRs | Published | Notes                          |
| :----------- | :------ | :-------- | :----------------------------- |
| main         | Yes     | No        | **Primary development branch** |

### PR Titles

PR titles should start with an action

```bash
- Add <feature>
- Bump Release version for <release>
- Change whatever you changed.
- Fix <thing you fixed> <reason you fixed it>
```

Please avoid ambiguous titles.

### PR Checklist

When creating your PR, please ensure the following:

* Ensure that the formatting meets golang's standards and that the tests still pass.

   ```bash
   cd toolkit
   # fix formatting
   sudo make go-tidy-all
   # check tests
   sudo make go-test-coverage
   ```

* Documentation has been updated to match any changes to the build system.

## Bugs

If the bug is security related, please use the [security guidelines](#security-vulnerabilities) above. Otherwise, please use the [issues page](https://github.com/microsoft/azure-linux-image-tools/issues) on Azure Linux Image Tools to file bugs.
