# Janitor bot

This is not the readme you are looking for

## API

Other bots on the same Docker network can call the janitor over gRPC. Set `GRPC_ADDR` (e.g. `:50051`) to turn it on; it's off when unset, and the port isn't published on the host. The service is defined in [`api/janitorv1/janitor.proto`](api/janitorv1/janitor.proto), which also says how to regenerate the Go code. It lives in its own module, so other bots can import `github.com/robert-mccausland/janitor-bot/api/janitorv1` without the janitor's dependencies.

- `Identity`: the janitor's user ID.
- `MoveMembers`: move people from one voice channel to another (or disconnect them), with the usual join and sounds.
- `HoldChannels`: voice channels the office round-up leaves alone until a given time (hr-bot holds meeting rooms in use).
