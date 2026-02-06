# Codesign Tool

This directory contains the codesign binary used for signing macOS binaries during the release process.

## Overview

The `sign` binary is a tool that signs macOS binaries using Sauce Labs' code signing infrastructure. This is required for Homebrew Casks, as macOS Gatekeeper requires signed binaries.

## Binary Placement

Place the pre-built `sign` binary in this directory:

```
tools/codesign/
├── README.md
└── sign          # The codesign binary (macOS executable)
```

## Building the Codesign Tool

The codesign tool source is located at:
https://gitlab.tools.saucelabs.net/sauce-connect-5/codesign/-/tree/main/cmd/sign

To build:

```bash
# Clone the repository (requires VPN access)
git clone https://gitlab.tools.saucelabs.net/sauce-connect-5/codesign.git
cd codesign

# Build the sign binary
go build -o sign ./cmd/sign

# Copy to saucectl
cp sign /path/to/saucectl/tools/codesign/
```

## Usage

The sign binary is called automatically during the release process via GoReleaser hooks.

```bash
./tools/codesign/sign <binary-path>
```

## Environment Variables

The codesign tool uses the following environment variables (configured as secrets in GitHub Actions):

**Signing:**
- `QUILL_SIGN_P12`: Base64-encoded P12 certificate file containing the Apple Developer ID
- `QUILL_SIGN_PASSWORD`: Password for the P12 certificate

**Notarization:**
- `QUILL_NOTARY_KEY`: Apple App Store Connect API private key (`.p8` file contents)
- `QUILL_NOTARY_KEY_ID`: The Key ID for the App Store Connect API key
- `QUILL_NOTARY_ISSUER`: The Issuer ID from App Store Connect

These variables are used by [Quill](https://github.com/anchore/quill), a tool for signing and notarizing macOS binaries.
