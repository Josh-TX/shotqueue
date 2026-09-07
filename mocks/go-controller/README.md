# go-controller

Go port of `poc-controller`. Browser-based proof-of-concept controller for `mock-ue150` (or a real
AW-UE150). Lets you type in an IP+port, watch the live snapshot and position, and set a new
position.

Self-contained: standard library only, no dependency on the rest of this repo.

## Run

```
go run . [--port=8080]
```

Then open `http://localhost:<port>`. Default port is `8080`; override with `--port=` or the
`PORT` env var (`--port=` wins if both are given).

`mock-ue150` also defaults to `8080` — if running both on the same machine, give one of them
`--port=`.

The browser only ever talks to this server — this server proxies every request on to whatever
host/port you typed in, so there's no CORS setup needed and the camera doesn't need to be
reachable from the browser directly (just from wherever this server runs).

## Using the page

1. **Connection** — enter the camera's host and port (defaults to `localhost` / `8080`, i.e. a
   local `mock-ue150`) and a poll rate in ms (defaults to `1000`), click **Connect**. This starts
   polling immediately. Changing the poll rate takes effect the next time you click **Connect**.
2. **Snapshot** — refetched at the poll rate while connected, shows the camera's current cropped
   view.
3. **Current position** — also refetched at the poll rate; shows the live pan/tilt/zoom as raw
   hex (the same values you'd get from `#PTV`). If the camera is mid-move, you'll see it update
   toward the target over ~1 second.
4. **Set position** — the three sliders (Pan/Tilt/Zoom, 0–100%) do *not* send anything while
   you're dragging them; they're just staged values. Click **Set Position** to send them all at
   once. 0%/100% map to that axis's documented hex range (see `main.go`'s `panMin/Max`,
   `tiltMin/Max`, `zoomMin/Max`, which must match `mock-ue150`'s).

If a request fails (bad host/port, camera unreachable, etc.), the status line shows the error
instead of silently stalling.

## Auth

Ported from `poc-controller/auth.js`: `internal/auth` answers a camera's Basic or Digest
challenge (RFC 7616), whichever it offers, using the built-in `mock-ue150` account
(`admin`/`12345`). Harmless against a camera with auth off — nothing is sent until the camera
actually challenges for it.

## Test

```
go test ./...
```
