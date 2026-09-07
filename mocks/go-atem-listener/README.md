# go-atem-listener

Minimal ATEM tally reader. Connects to a real ATEM (or mock-atem) over UDP:9910 and prints what's live/preview. No control, read-only.

## Run

```
go build
./go-atem-listener --ip=<switcher ip>
```

Prints one JSON line per tally change to stdout:

```
{"live":[5],"preview":[3]}
```

Connection/reconnect logs go to stderr. Auto-reconnects with backoff if the switcher goes away.
