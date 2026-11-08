# Tail 2: updates to tail-cameras.md

Based on captures in `logs/` from a real Tail 2 (direct connection, `192.168.103.220`), taken with `tail-camera-debugger` at different zoom levels. Each capture has two files: `http <zoom> zoom.txt` and `info <zoom> zoom.txt`. Only zoom was varied. Nothing in the logs is focus-related.

## Corrections

**1. Zoom CAN be read back over OSC.** `tail-cameras.md` (section 1, and `api.ts:309`) says `ZoomInfo` is always `[0, -1]`. That is wrong. The first arg changes with zoom:

| HTTP `ratio` | OSC `ZoomInfo[0]` |
|---|---|
| 1.0 | 0 |
| 1.2 | 1 |
| 2.8 | 16 |
| 3.0 | 18 |
| 10 | 88 |

- Both args are int32.
- The value is roughly `(ratio - 1) * 10 - 2`, but it is not exactly linear. I have 5 points.
- **Unverified:** that it's a 0-100 scale matching `SetZoom`'s 0-100 arg. **Unverified:** the max zoom. 10x gives 88, so the max may be above 10x.
- `ZoomInfo[1]` was -1 in every capture. Its meaning is unknown.
- I only have the HTTP ratio at 1 and 3, so the 1.2, 2.8 and 10 ratios are the zoom levels in the filenames, not confirmed readings.

**2. `Get*` queries are NOT ignored on a direct connection.** The doc says they are ignored. `GetGimbalPosInfo` (int32 `0`) sent 300ms after `Connected` got a `GetGimbalPosInfoResp`.
- The reply has 2 int32 args and no device-index prefix.
- The reply to `Connected` also included a `GetGimbalPosInfoResp`, so both give the same data.
- I didn't test other `Get*` queries.

## Confirmed

- **Gimbal order is `[pitch, yaw]`.** The captured pair was `[9, -20]`. The reported order agrees with the doc's assumption, but the logs can't show which axis is which, because I don't know the camera's physical position. **Still unverified** against a known pan or tilt move.
- **Gimbal values are int32.** The doc didn't know the type. They looked like whole degrees.
- **Reply to `Connected`.** The camera sends these, in order: `ConnectedResp` (no args), `DeviceInfo`, `ZoomInfo`, `GetGimbalPosInfoResp`, `PresetPositionInfo`. All arrived within about 3ms.
- **`DeviceInfo`** args: `[1, "Tail2", 0, "", 0, "", 0, "", 0, 1, 5]`.
- **`PresetPositionInfo`** args: `[1, "UHJlc2V0czE=", 1, "UHJlc2V0czI=", 0, ""]`. The strings are base64 of "Presets1" and "Presets2". The pattern looks like `[enabled, name]` x 3 slots. Slot 3 is unset. This reading is **unverified**.
- **Ports.** The send peer was `192.168.103.220:57110`. The `recv` peer is shown as the same address and port in the debugger's log. I did not check the actual UDP source port.

## Gimbal noise

Pitch and yaw moved by 1 degree between captures with no commanded move (pitch 9 or 8, yaw -20 or -19). There's no correlation with zoom. Don't treat a 1-degree change as real movement when polling.

## HTTP API (new, verified)

Base: `http://<ip>/camera/sdk/`. All requests were GET, returned 200 and took under 10ms.

| Path | Response |
|---|---|
| `device_info` | `{device_name, wired_ip, wireless_ip, mac}` |
| `ptz/zoom` | `{"ratio": 1.0}` (float, 1.0 = no zoom) |
| `networkconfig` | NDI, network, multicast and stream params. `ndi_enable`, `ip_type`, `ip`, `mask`, `gateway`, `multicase_param`, `stream_param`: `encoder_format`, `fps`, `resolution`, `bitrate` |
| `ndi-rtsp-srt/control` | `{"control": "off"}` |
| `ndi-rtsp-srt/rtspurl` | Main and sub RTSP URLs, wired and wireless |

- **RTSP:** `rtsp://<ip>/stream1` (main) and `rtsp://<ip>/stream2` (sub). Streaming was off (`control: off`) during the captures, so I don't know whether the URLs work in that state. This could give the thumbnail source for section 3 (`ffmpeg -i rtsp://<ip>/stream1 -frames:v 1 thumb.jpg`). **Untested.**
- **`ptz/zoom` reports the zoom ratio directly**, so it's the better zoom source than OSC, which needs the curve above.
- **Position over HTTP:** I didn't call a gimbal or position endpoint, so section 1's "possible alternative" is still unresolved.

## Still unknown

- Anything about focus (no endpoint or OSC message was captured).
- `SetZoom` mapping to ratio.
- Max zoom ratio.
- Gimbal units and axis meaning.
- Whether the RTSP URLs work, and whether they work with streaming "off".
- HTTP endpoints for position.
