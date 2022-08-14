# Octarchive

![Logo](./docs/logo-readme.png)

Simple tool to back up all repos on a GitHub/Gitea account to a local folder.

[![hydrun CI](https://github.com/pojntfx/octarchive/actions/workflows/hydrun.yaml/badge.svg)](https://github.com/pojntfx/octarchive/actions/workflows/hydrun.yaml)
[![Docker CI](https://github.com/pojntfx/octarchive/actions/workflows/docker.yaml/badge.svg)](https://github.com/pojntfx/octarchive/actions/workflows/docker.yaml)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/octarchive.svg)](https://pkg.go.dev/github.com/pojntfx/octarchive)
[![Matrix](https://img.shields.io/matrix/octarchive:matrix.org)](https://matrix.to/#/#octarchive:matrix.org?via=matrix.org)
[![Binary Downloads](https://img.shields.io/github/downloads/pojntfx/octarchive/total?label=binary%20downloads)](https://github.com/pojntfx/octarchive/releases)

## Overview

Octarchive is a simple backup utility that clones all repos from a GitHub/Gitea account to a local folder for storage.

It enables you too ...

- **Backup your work**: In case your GitHub account is breached or gets banned, Octarchive ensures you always have a local copy available.
- **Mirror your account**: If your internet connection is slow or GitHub is banned in your jurisdiction, you can use Octarchive and a web server to mirror your repos.
- **Automate processes**: By exposing all of your repos to the filesystem, otherwise tedious processes such as bumping copyright dates or updating names become easy.

## Installation

### Containerized

You can get the OCI image like so:

```shell
$ podman pull ghcr.io/pojntfx/octarchive
```

### Natively

Static binaries are available on [GitHub releases](https://github.com/pojntfx/octarchive/releases).

On Linux, you can install them like so:

```shell
$ curl -L -o /tmp/octarchive "https://github.com/pojntfx/octarchive/releases/latest/download/octarchive.linux-$(uname -m)"
$ sudo install /tmp/octarchive /usr/local/bin
```

On macOS, you can use the following:

```shell
$ curl -L -o /tmp/octarchive "https://github.com/pojntfx/octarchive/releases/latest/download/octarchive.darwin-$(uname -m)"
$ sudo install /tmp/octarchive /usr/local/bin
```

On Windows, the following should work (using PowerShell as administrator):

```shell
PS> Invoke-WebRequest https://github.com/pojntfx/octarchive/releases/latest/download/octarchive.windows-x86_64.exe -OutFile \Windows\System32\octarchive.exe
```

You can find binaries for more operating systems and architectures on [GitHub releases](https://github.com/pojntfx/octarchive/releases).

## Usage

🚧 This project is a work-in-progress! Instructions will be added as soon as it is usable. 🚧

## License

Octarchive (c) 2022 Felicitas Pojtinger and contributors

SPDX-License-Identifier: AGPL-3.0
