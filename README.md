# shotqueue

Automates PTZ cameras between saved shot presets, cued by which camera is live on an ATEM switcher — for live-production/streaming setups (church, small studio, etc).

## Glossary

- **ATEM** — Blackmagic video switcher. shotqueue connects read-only to its tally feed (UDP 9910) to know what's live/preview.
- **Tally** — live/preview status broadcast by the ATEM per input source.
- **PTZ camera** — a pan-tilt-zoom camera. shotqueue drives Panasonic AW-UE150s (or the `mock-ue150` stand-in) over their HTTP CGI interface.
- **Position** — a camera's pan/tilt/zoom values.
- **Preset** — a named, saved position + thumbnail snapshot for one camera.
- **Group** — a named pool of a camera's presets used for auto-queueing.
- **Queued preset** — the preset that will fire next time the camera goes off-live. Set manually, or auto-set from the camera's selected group.
- **Triggering** — transient state while a preset's position is being applied to the camera.
- **Camera status** — `none` / `preview` / `live`, derived from tally + the camera's configured tally source number.
- **TallySource** — the ATEM input number a camera is wired to.
- **Version** - A saved configuration encompassing the camera, preset, and group settings
## How it works

1. Backend listens to the ATEM's tally feed and tracks each camera's status.
2. While a camera is live, you can't trigger it. While it's off-live, you (or the auto-queue) pick a preset to queue.
3. When the camera goes off-live, its queued preset fires: backend sends the saved position to the camera, polls until it settles, grabs a fresh snapshot.
4. Frontend gets structural state (cameras, presets, groups, queue) over a websocket, and polls position over REST (1s normally, 100ms while triggering) to catch when a trigger has settled.

Only the ATEM host and camera roster are persisted (`config.json`); presets/groups are runtime state — see `todo.txt`.

## Layout

- `backend/` — Go server: ATEM tally listener, PTZ control, REST + websocket API, serves the built frontend.
- `frontend/` — Vue + Vite UI.
- `mocks/` — throwaway PoCs used to prototype the backend:
  - `mock-controller` — proved out getting/setting a camera's position and snapshot; `backend/internal/ptz` is ported from this.
  - `mock-atem` — fake ATEM tally server, for dev without real switcher hardware.
  - `mock-ue150` — fake AW-UE150 camera, for dev without real camera hardware.

## Testing / debugging with mocks

VSCode compound **"shotqueue+mocks"** (`.vscode/launch.json`) launches everything at once:

| Process              | Port(s)               |
|----------------------|------------------------|
| Backend (Delve)      | 8080 (`--port`)        |
| Frontend (Vite)      | 5173                    |
| Mock Controller      | 5000                    |
| Mock ATEM            | UDP 9910 (fixed) + web UI 9000 |
| Mock UE150 (x3)      | 5001, 5002, 5003        |

Or run mocks standalone via `mocks/run-*.sh` (same default ports as above).

To wire shotqueue to the mocks: in Settings, set ATEM host to `localhost` (port is fixed at
9910, not configurable). Add cameras with host `localhost` and port `5001`/`5002`/`5003`, one per
`mock-ue150` instance.

Mock ATEM's web UI (port 9000) lets you set live/preview/cut/fade to drive tally state. Mock
Controller (port 5000) is a standalone page for poking a `mock-ue150` camera directly, independent
of shotqueue.

Note: all of backend, mock-controller, and mock-ue150 default to port 8080 if run standalone
without an explicit port — only run one at a time on that port, or override with `--port=`.

## Known limitations

See `todo.txt`.
