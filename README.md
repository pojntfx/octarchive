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

### 1. Do a manual backup with `octarchive`

First, export your GitHub (or Gitea) API token like so:

```shell
$ export GITHUB_TOKEN='mygithubtoken'
```

If you're using Gitea, also export your API endpoint like so:

```shell
$ export GITHUB_API='https://try.gitea.io/api/'
```

Now, start the manual backup, including all the repos of the organizations that you're part of:

```shell
$ octarchive --orgs
{"level":"info","time":"2022-08-15T00:25:39+02:00","message":"Getting user"}
{"level":"info","time":"2022-08-15T00:25:40+02:00","message":"Getting organizations for user"}
# ...
Cloning   6% [========>                                                                                                                                                           ] (16/263, 7 repo/s) [1s:32s]{"level":"info","cloneURL":"https://github.com/pojntfx/dwm.git","filePath":"/home/pojntfx/.local/share/octarchive/var/lib/octarchive/data/1660518181/pojntfx/dwm","time":"2022-08-15T01:03:09+02:00","message":"Cloning repo"}
# ...
```

You should now find the repos in `${HOME}/.local/share/octarchive/var/lib/octarchive/data`.

For more information, see the [reference](#reference).

### 2. Schedule backups with systemd Timers

In most cases, you'll want to schedule backups periodically; an excellent way to do so is to use [systemd Timers](https://wiki.archlinux.org/title/Systemd/Timers). To schedule a weekly backup of all of your repos, run the following:

```shell
$ sudo tee /etc/systemd/system/octarchive.service<<'EOT'
[Unit]
Description=Octarchive backup

[Service]
Type=oneshot
ExecStart=/usr/local/bin/octarchive --orgs
Environment="GITHUB_TOKEN=mygithubtoken"

[Install]
WantedBy=multi-user.target
EOT
$ sudo tee /etc/systemd/system/octarchive.timer<<'EOT'
[Unit]
Description=Run Octarchive weekly

[Timer]
OnCalendar=weekly
Persistent=true

[Install]
WantedBy=timers.target
EOT
$ sudo systemctl daemon-reload
$ sudo systemctl enable octarchive.timer --now
```

Note that this will create a fresh directory every time you run the backup, which might fill up your disk space quite quickly; if you want to instead remove the old backup every time you do a new one, append `--timestamp current` to the `ExecStart` line of the service.

For more information, see the [reference](#reference).

🚀 **That's it!** We hope you enjoy using Octarchive.

## Reference

### Command Line Arguments

```shell
$ octarchive --help
Usage of octarchive:
  -api string
        GitHub/Gitea API endpoint to use (can also be set using the GITHUB_API env variable) (default "https://api.github.com/")
  -concurrency int
        Maximum amount of repositories to clone concurrently (default 20)
  -dst string
        Base directory to clone repos into (default "/home/pojntfx/.local/share/octarchive/var/lib/octarchive/data")
  -fresh
        Clear timestamp directory before starting to clone
  -orgs
        Also clone repos of all orgs that the user is part of
  -timestamp string
        Timestamp to use as the directory for this clone session (default "1660513831")
  -token string
        GitHub/Gitea API access token (can also be set using the GITHUB_TOKEN env variable)
  -verbose int
        Verbosity level (0 is disabled, default is info, 7 is trace) (default 5)
```

### Environment Variables

You can set the following environment variables, which correspond to the values that can be set using the following flags:

| Environment Variable | Flag      |
| -------------------- | --------- |
| `GITHUB_API`         | `--api`   |
| `GITHUB_TOKEN`       | `--token` |

## Acknowledgements

- [go-git/go-git](https://github.com/go-git/go-git) provides the Git library.

To all the rest of the authors who worked on the dependencies used: **Thanks a lot!**

## Contributing

To contribute, please use the [GitHub flow](https://guides.github.com/introduction/flow/) and follow our [Code of Conduct](./CODE_OF_CONDUCT.md).

To build and start a development version of Octarchive locally, run the following:

```shell
$ git clone https://github.com/pojntfx/octarchive.git
$ cd octarchive
$ make depend
$ make && sudo make install
$ export GITHUB_TOKEN='mygithubtoken'
$ octarchive
```

Have any questions or need help? Chat with us [on Matrix](https://matrix.to/#/#octarchive:matrix.org?via=matrix.org)!

## License

Octarchive (c) 2022 Felicitas Pojtinger and contributors

SPDX-License-Identifier: AGPL-3.0
