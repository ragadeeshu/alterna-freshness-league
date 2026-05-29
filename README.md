# alterna-freshness-league
This is a little tool that compares your Splatoon 3 single-player times with those of your friends. It uses a [`nxapi`](https://github.com/samuelthomas2774/nxapi) sidecar for Nintendo Switch Online auth and for dumping SplatNet 3 hero/history records.

Special thanks to [@frozenpandaman](https://github.com/frozenpandaman) for showing how to navigate the login flow over in [s3s](https://github.com/frozenpandaman/s3s), [@JoneWang](https://github.com/JoneWang) for the [imink f-api](https://github.com/imink-app/f-API) this project used previously, and [Ellie](https://gitlab.fancy.org.uk/samuel) for [nxapi](https://github.com/samuelthomas2774/nxapi) which this project now uses.

## Configuration

Create a `contestants.json` with the following structure:
```json
{
    "league": "leagueName",
    "contestants": [
        {
            "name": "tomte",
            "session_token": "session_token"
        },
        {
            "name": "tomte2",
            "session_token": "session_token"
        }
    ],
    "proxies": [
        "http://localhost:8081",
        "http://localhost:8082"
    ]
}
```
Which contains all of your contestants. You need either their session tokens, or you need them to run a proxy (see below). How to get a session token? If you use s3s you can grab it from your `config.txt`.

## Running with Docker Compose (recommended)

```sh
docker compose up --build
```

This starts:
- `nxapi` sidecar on `:2727`, running the `nxapi-bridge` binary built from `./nxapi-bridge`. The bridge shells out to the `nxapi` CLI on every request.
- `freshness-league` on `:8080`, which talks to the sidecar over HTTP only.

The `freshness-league` container talks to the sidecar at `http://nxapi:2727`. All `nxapi` state (`/data/persist`) lives in the `nxapi-data` volume on the sidecar, so multiple `freshness-league` instances pointed at the same sidecar share one token cache.

### Sidecar HTTP API

The bridge intentionally exposes a tiny surface (the rest of `nxapi` is not reachable from outside the sidecar):

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/user?token=<session_token>` | Runs `nxapi nso user` and extracts the NSO/Coral display name + Mii avatar URL, returned as `{nsoName, nsoImage}` |
| `GET` | `/api/splatnet3/records?token=<session_token>` | Runs `nxapi splatnet3 dump-records --hero --history` and returns the raw GraphQL payloads as `{heroResult, historyPlayer}` |



## Running the Go app directly

The app talks to the sidecar over HTTP and has no other runtime dependencies. Make sure the bridge is reachable (either run `docker compose up nxapi` or run the bridge yourself with `go run ./nxapi-bridge` plus a local `nxapi` install).

```
Usage of ./alterna-freshness-league:
  -league value
        Contestants json string, tries to read 'contestants.json' file if not set
  -port string
        Port to bind to (default "8080")
  -proxy
        Start in proxy mode
 ```

What does it look like? Something like this:
![web example](alterna%20freshness%20league.png)
  -sidecar-url string
        Base URL for the nxapi-bridge sidecar (default "http://127.0.0.1:2727")
```

Note: Nintendo authentication tokens are sent through the `nxapi` sidecar, which forwards relevant data to `nxapi-znca-api.fancy.org.uk` (Coral `f` generation). Review the upstream terms in the [nxapi-znca-api docs](https://github.com/samuelthomas2774/nxapi-znca-api/blob/docs/docs/public-api-terms.md).
