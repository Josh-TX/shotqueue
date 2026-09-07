# mock-atem

Fake ATEM switcher for testing PTZ/tally clients. Emits real ATEM UDP protocol packets (handshake, session, tally) on port 9910. Not a real video switcher — no video, just protocol + state.

## Run

```
npm install
npm start [webPort]   # default 8080
```

Open `http://localhost:<webPort>` for the control UI (8 cams, Live/Preview per cam, CUT, FADE).

## Notes

- Single M/E model: exactly 1 live + 1 preview input at all times.
- Live/Preview buttons set that bus directly and instantly.
- CUT swaps live/preview instantly.
- FADE swaps instantly but holds both old+new live cams tallied for 2s (hardcoded), matching real ATEM tally-during-transition behavior.
- Supports multiple simultaneous UDP clients (e.g. several go-atem-listener instances).
