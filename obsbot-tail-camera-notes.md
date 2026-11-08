# Tail 2: position, and thumbnail

I based this on this repo's code (`src/api.ts`, `src/actions.ts`) and web searches. Anything marked **unverified** I haven't tested on a real camera.

## 1. Get current position

**Transport.** OSC 1.0 packets over UDP (default) or TCP. TCP frames are length-prefixed (4-byte length, then the OSC packet; `tcp.ts:54`). Send to `<camera-ip>:57110` (the module's `port` setting). Replies arrive on the UDP listen port, default 57120, and the module only accepts packets whose source IP matches the camera's IP (`api.ts:210`).

**Request (Tail 2, direct connection).**
- Address: `/OBSBOT/WebCam/General/Connected`
- Args: one int32, value `0`.
- No device-index prefix. This address is in the module's `AddressesWithoutDeviceId` list (`api.ts:494`).
- The module sends it every poll interval (`api.ts:120`).

**Response.**
- The Tail 2 answers `Connected` with `/OBSBOT/WebCam/General/ConnectedResp`, then its full state. That includes `/OBSBOT/WebCam/General/GetGimbalPosInfoResp`. The "full state" claim comes from a code comment (`api.ts:118`). I haven't seen a raw packet capture.
- `GetGimbalPosInfoResp` args for the Tail 2: two numbers, `[pitch, yaw]` (`api.ts:484`). The module reads them with a lenient number parser, so I don't know if they're int32 or float32.
- Units are probably degrees. The module's limits for setting the Tail 2 are pan -159 to 159 and pitch -62 to 62 (`actions.ts:340`). I'm assuming the reported values use the same scale. **Unverified.**
- Order is assumed from the module's parser: pitch first, yaw second. If pan and tilt look swapped on a real camera, flip this. **Unverified.**
- Zoom can't be read back. `ZoomInfo` is always `[0, -1]` on Tail 2 hardware (`api.ts:309`).

**Other connection modes (for reference).**
- Through OBSBOT Center, the request is `/OBSBOT/WebCam/General/GetGimbalPosInfo` with args `[int32 device index]`. The device index is 0-based and the module prepends it.
- The reply is `GetGimbalPosInfoResp`. With 3 args it's `[device index, pitch, yaw]`. With 2 args on non-Tail-2 models the module reads `[yaw, pitch]`, the opposite order from the Tail 2 (`api.ts:479`). That inconsistency is in the module's parser, not something I've confirmed against the spec.
- On a direct Tail 2 connection, `Get*` queries are ignored.

**Possible alternative (unverified).** The unofficial HTTP API under `/camera/sdk/` and its WebSocket status push probably expose position. Request and response shapes are unknown to me. `lxman/obsbot-mcp`'s `TAIL2-PROTOCOL.md` is the place to check. I couldn't find that file in my searches.

## 2. Set current position

All of these are OSC messages to `<camera-ip>:57110`. On a direct Tail 2 connection no device-index prefix is added. Through Center, the module prepends a 0-based device-index int32 to the args below. No reply is documented for any of them. To confirm a move, re-poll with `Connected`.

**Absolute position.**
- Address: `/OBSBOT/WebCam/General/SetGimMotorDegree`
- Args: three int32, in this order: `speed`, `pan`, `pitch` (`actions.ts:390`).
- `speed`: 0 to 90. What `0` means (fastest, default speed or no move) is unknown. **Unverified.**
- `pan`: Tail 2 range -159 to 159. Other models -129 to 129.
- `pitch`: Tail 2 range -62 to 62. Other models -59 to 59.
- Values are whole degrees. A fractional "Precise" variant exists but the module only offers it through Center.
- Example: pan 30, pitch -10, speed 45 means int32 args `[45, 30, -10]`.

**Continuous movement.**
- Addresses: `/OBSBOT/WebCam/General/SetGimbalUp`, `SetGimbalDown`, `SetGimbalLeft`, `SetGimbalRight`.
- Args: one int32 speed, 1 to 100. The camera keeps moving until stopped.
- Stop: the same address with int32 `0` (`actions.ts:209`).

**Recenter.**
- Address: `/OBSBOT/WebCam/General/ResetGimbal`
- Args: one int32, value `0`.

**Zoom.**
- Address: `/OBSBOT/WebCam/General/SetZoom`
- Args: one int32, 0 to 100.
- Related: `SetZoomSpeed` (Tail 2 max 10), `SetZoomMin`, `SetZoomMax`.

**Saved positions.**
- Address: `/OBSBOT/Camera/Tail2/TriggerPreset`
- Args: one int32, **0-based** preset index, 0 to 2 (the module's UI shows 1 to 3 and subtracts 1; `actions.ts:1344`).

**Unverified.**
- Whether `SetGimMotorDegree` behaves the same on Tail 2 hardware as through Center.
- Whether AI tracking overrides or fights manual moves. The module has AI-mode and "Only Me" toggles (`/OBSBOT/Camera/Tail2/SetAiEnable`), so you may need to disable tracking first.
- Whether out-of-range values are clamped or rejected.
- The HTTP API probably has set-position endpoints, but I have nothing confirmed.

## 3. Get a thumbnail

**There is no official snapshot-to-image API.**
- The OSC "Take Snapshot" command (`/OBSBOT/Camera/Tail2/Snapshot`) only saves a photo on the camera or in Center. It returns no image.
- So you have to pull a frame out of a video output. Which one depends on your show's setup.

**Option A: NDI (my recommendation if the show already uses NDI).**
- Subscribe to the camera's NDI source with the NDI SDK or an ffmpeg build with NDI support, and save one frame.
- It doesn't change the camera's output mode. NDI supports multiple receivers.
- **Unverified:** whether the Tail 2 limits how many receivers it allows, and what NDI flavor it sends (I believe NDI|HX).
- The Tail 2 needs an NDI license key.

**Option B: SRT.**
- Turn on SRT in OBSBOT Center in listener mode, port 5000 by default. Then run `ffmpeg -i srt://<ip>:5000 -frames:v 1 thumb.jpg`. Your ffmpeg build needs libsrt.
- The `obsbot-mcp` project does this and can enable SRT over the HTTP API.
- **Drawbacks:** SRT is exclusive with NDI, UVC, RTSP and Live Stream, and a reboot resets it. It would knock out the show's NDI feed, so I wouldn't use it on live cameras.

**Option C: RTSP.**
- It's officially supported, but the reverse-engineering author couldn't get it to serve on firmware 7.2.13.1. Treat it as unreliable until you test it.

**Option D: USB (UVC).**
- It works as a normal webcam, but the camera must be plugged into the capture machine, and this is incompatible with the one-cable plan.

**Option E: take the frame from the production side.**
- Grab it from the switcher, a multiviewer, or an existing NDI receiver. This puts no extra load on the camera, and it's often the most practical in a big show.

**Unverified overall:** whether there's any simple HTTP snapshot URL on the camera. I found no evidence of one. `/camera/sdk/` endpoints could include something, so test them when you have the camera.

## 4. Risks and what to test first

- **Protocol maturity:** OSC is official but thin. The HTTP API is undocumented and was found by reverse engineering on one firmware. Pin your firmware version and re-test after updates.
- **Coexisting with Companion:** if your project and this module both control the camera, commands can conflict and the UDP reply port can clash. A shared listener, or the HTTP API for your side, may avoid this. Test it.
- **First tests when you have the camera:**
  1. Send `Connected` and log the reply to confirm the pitch and yaw units.
  2. Send `SetGimMotorDegree` and compare it with the reported position.
  3. Confirm NDI is enabled, and grab a frame with ffmpeg or the NDI SDK.
  4. Probe `http://<ip>/camera/sdk/...` for position and any image endpoints.

## Sources
- [Reverse-engineering the OBSBOT Tail 2 (DEV)](https://dev.to/michael_jordan_87eaf96f24/reverse-engineering-the-obsbot-tail-2-http-usb-and-a-linux-kernel-fix-12em)
- [lxman/obsbot-mcp](https://github.com/lxman/obsbot-mcp)
- [obsbot_tail2_snapshot](https://glama.ai/mcp/servers/lxman/obsbot-mcp/tools/obsbot_tail2_snapshot)
- [obsbot_tail2_stream](https://glama.ai/mcp/servers/lxman/obsbot-mcp/tools/obsbot_tail2_stream)
- [OBSBOT SRT setup for Tail 2](https://www.obsbot.com/explore/obsbot-tail-2/srt-protocol)
- [OBSBOT NDI guide](https://www.obsbot.com/solutions/ndi-guide)
- [OBSBOT RTSP guide](https://www.obsbot.com/blog/live-streaming/rtsp-streaming)
