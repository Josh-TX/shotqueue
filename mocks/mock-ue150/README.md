# mock-ue150

Standalone mock of a Panasonic AW-UE150's HTTP/CGI control API, for testing without real hardware.
Serves crops of a single static image as the "camera feed," and tracks a fake pan/tilt/zoom
position that PTZ commands read and write.

Self-contained: its own `package.json`/`node_modules`, no dependency on the rest of this repo.

## Run

```
npm install
node index.js <imagePath> [--auth=on|off] [--port=8080]
```

- `<imagePath>` — required. Any image `jimp` can decode (jpg/png/etc). This is the "sensor" the
  fake camera crops from.
- `--auth` — default `off`. `on` requires HTTP Basic auth on every request.
- `--port` — default `8080`.

## Endpoints

Only what's needed to get PTZ position, set PTZ position, and grab a snapshot — not a full
implementation of the camera's command set.

### `GET /cgi-bin/aw_ptz?cmd=<cmd>&res=1`

`<cmd>` is `#` + a mnemonic (send `%23` for `#` in the URL). Reply is `200 OK` with the answer as
plain-text body — errors are also `200`, encoded as `eR1:<cmd>` in the body, not an HTTP error.

| cmd | meaning | reply |
|---|---|---|
| `#PTV` | get pan+tilt+zoom (+dummy focus/iris) | `pTV` + pan(4hex) + tilt(4hex) + zoom(3hex) + `555555` |
| `#APC` | get pan+tilt | `aPC` + pan(4hex) + tilt(4hex) |
| `#APC<pan4hex><tilt4hex>` | set pan+tilt | echoes the target immediately |
| `#GZ` | get zoom | `gz` + zoom(3hex) |
| `#AXZ<zoom3hex>` | set zoom | echoes the target immediately |
| anything else | unsupported | `eR1:<cmd>` |

Focus/iris aren't modeled — `#PTV` always reports a fixed `555`/`555` for them.

### Gradual moves

Setting pan/tilt (`#APC`) or zoom (`#AXZ`) doesn't jump instantly — like the real camera, the
reply echoes the target position right away, but the actual position interpolates linearly from
wherever it was to the target over 1 second (`MOVE_DURATION_MS` in `index.js`). Any `#PTV`/`#APC`
get, or `view.cgi` snapshot, taken during that second reflects the in-progress position, not the
final target. Pan/tilt and zoom move independently (setting one doesn't affect the other's
timing).

### `GET /cgi-bin/view.cgi?action=<start|stop|snapshot>`

- `start` / `stop` — no-ops, always `200`. (On a real camera, `start` must be called once before
  `snapshot` returns real frames; this mock doesn't require it, since it isn't needed for these 3
  operations.)
- `snapshot` — `200`, `Content-Type: image/jpeg`, body is a 640×360 JPEG crop of `<imagePath>`
  based on the current pan/tilt/zoom (see below).

## PTZ → crop mapping

Pan, tilt, and zoom are stored as raw ints matching the wire format (same hex you'd get/set via
`#PTV`/`#APC`/`#AXZ`), then mapped onto a crop rectangle of the source image each time a snapshot
is rendered:

- **Zoom** controls crop *size*. `0x555` (wide) → crop is the full image. `0xFFF` (full tele) →
  crop shrinks to `MIN_CROP_FRACTION` (20%, in `index.js`) of the image's width/height. Values in
  between scale linearly.
- **Pan** controls crop *center X*. `0x2D09` → center is the image's left edge. `0xD2F5` → center
  is the right edge. Linear in between.
- **Tilt** controls crop *center Y*, same linear mapping, `0x2D09`→top, `0xD2F5`→bottom.
- The crop rect is clamped so it never runs off the edge of the source image (center gets pushed
  inward near the edges rather than the crop going out of bounds).
- The resulting crop is resized to a fixed 640×360 output, regardless of the source image's or
  crop's aspect ratio.

Default position on startup: pan/tilt `0x8000` (center), zoom `0x555` (full wide, i.e. the whole
image).

### Why pan and tilt share one range

The real camera's documented ranges are pan `0x2D09`–`0xD2F5` and tilt `0x8E38`–`0x1C71`, both
centered on `0x8000`. Pan's range is numerically sane (`0x2D09 < 0x8000 < 0xD2F5`), but tilt's
isn't — `0x8000` doesn't fall inside `[0x8E38, 0x1C71]` under any consistent interpretation
(plain range or wraparound), so treating it literally would put the "center" position outside its
own valid range. Since this mock only needs *some* value range that a real controller's captured
preset data will fall within — not physically-accurate tilt degrees — tilt just reuses pan's
range (`PAN_MIN`/`PAN_MAX` in `index.js`) so both axes are centered and consistent.

Zoom's range (`0x555`–`0xFFF`) is used as documented — no ambiguity there.

## Auth

When `--auth=on`, every request needs HTTP Basic auth for a hardcoded account:

- username: `admin`
- password: `12345`

Missing/wrong credentials get `401` with `WWW-Authenticate: Basic realm="Control"`. No Digest
support — the real camera can do Basic too, and that's enough to exercise auth-on code paths.

## What's deliberately not implemented

- Digest auth
- Resolution selection (`#RZL`) — output is always 640×360
- Focus/iris control
- Presets, speed-based PTZ moves, any command outside the table above
- Real error codes (`eR2`, `eR3`) — out-of-range PTZ values are clamped visually rather than
  rejected
