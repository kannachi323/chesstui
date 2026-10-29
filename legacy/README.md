# Legacy C CLI

This is the original `chess-cli`, a C11 program built with CMake and a vendored
libcurl submodule. It is kept for history and is not part of the ChessTUI build
or release. The maintained application is the Go module in [`../chesstui`](../chesstui).

```sh
git submodule update --init legacy/vendor/curl
cmake -S legacy -B build/legacy && cmake --build build/legacy
```

`todo.txt` is the C CLI's live-play checklist. The Go rewrite covers the same
Board API features.
