# ASICamera2 management driver

This folder contains a windows daemon program to manage an ASICamera2 compatible camera from a directly conected Windows PC.

The service exposes an API to operate and query the camera remotely.

## Requirements

### Compilation

To compile the source code in windows, you need both [golang](https://go.dev/doc/install), [TDM-GCC](https://jmeubank.github.io/tdm-gcc/), [CMake])(https://cmake.org/download/) and [OpenCV](https://gocv.io/getting-started/windows/) installed, and `C:\TDP-GCC\bin` added to your path.

Once all the requirements are in place, just run:

```
cd cmd\driver
go get
go build
```

### Release

To release a new version of the software, tag it and then run [goreleaser](https://goreleaser.com):

```bash
export GITHUB_TOKEN="..."
goreleaser release
```

## Installation

- Create folder `C:\AsiCamera`
- Copy files:
  - `asicamera2.exe`
  - `ASICamera2.dll`
- Create file:
  - `config.toml`
- Run in a privileged shell:
  - `.\driver.exe install`
  - `net start AsiCameraDriver`

## Deinstallation

- Enter directory `C:\AsiCamera` in a privileged shell
- Run:
  - `.\asicamera2.exe uninstall`
