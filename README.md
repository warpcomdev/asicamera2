# ASICamera2 management driver

This folder contains a windows daemon program to manage an ASICamera2 compatible camera from a directly conected Windows PC.

The service exposes an API to operate and query the camera remotely.

## Requirements

To compile the source code in windows, you need both [golang](https://go.dev/doc/install), [TDM-GCC](https://jmeubank.github.io/tdm-gcc/), [CMake])(https://cmake.org/download/) and [OpenCV](https://gocv.io/getting-started/windows/) installed, and `C:\TDP-GCC\bin` added to your path.

Assuming you have uncompressed and installed the opencv libraries to `C:\opencv`, you will have to set the following environment variable 