# mock-controller

Go port of `poc-controller`. Browser-based proof-of-concept controller for `mock-ue150` (or a real
AW-UE150). Supports multiple cameras as tabs, and per camera: watch the live snapshot and
position, and set a new position by dragging a rectangle on the snapshot.

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

1. **Tabs** — click **+** to add a camera (prompts for host, defaulting to `localhost`, then
   port, defaulting to `8080`). Each tab is an independent camera, labeled `Camera N` by its
   position in the tab bar. Click a tab to switch to it; click its **×** to remove it. The tab
   list persists across reloads (browser `localStorage`). Only the active tab polls, at a fixed
   500ms.
2. **Snapshot** — refetched every 500ms while a tab is active, shows the camera's current cropped
   view.
3. **Current position** — also refetched every 500ms; shows the live pan/tilt/zoom as raw hex
   (the same values you'd get from `#PTV`). If the camera is mid-move, you'll see it update
   toward the target over ~1 second.
4. **Set position** — click **Begin Set Position**. This centers pan/tilt and zooms all the way
   out, and waits for the camera to settle. Once settled, click-and-drag on the snapshot to draw
   a rectangle (forced to the snapshot's 16:9 aspect ratio); on release, the camera zooms in to
   match that rectangle. A too-small drag cancels back to the normal view. The rectangle→position
   math assumes `mock-ue150`'s crop model (a `MIN_CROP_FRACTION` of `0.2`, hardcoded in `app.js`
   to match `index.js`) — it won't correspond 1:1 to real optical zoom on an actual AW-UE150.

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
