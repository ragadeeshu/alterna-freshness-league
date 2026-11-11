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
Which contains all of your contestants. You need either their session tokens, or you need them to run a proxy (see below). A proxy is an instance started with `-proxy` (or `PROXY=true`); it serves its own contestants' data as JSON on `/` (Alterna) and `/order` (Side Order), and the main instance merges in every URL listed under `proxies`. How to get a session token? If you use s3s you can grab it from your `config.txt`.

## Running with Docker Compose (recommended)

```sh
docker compose up --build
```

This starts:
- `nxapi` sidecar on `:2727`, running the `nxapi-bridge` binary built from `./nxapi-bridge`. The bridge shells out to the `nxapi` CLI on every request.
- `freshness-league` on `:8080`, which talks to the sidecar over HTTP only. It serves two pages:
  - `/`: the Alterna (hero mode) league.
  - `/order`: the Side Order league, styled after SplatNet 3's Side Order pages, with each player's fastest run per palette (chip board, hacks, date) side by side. Each palette is a comparison won by the fastest clear (N points for 1st, N−1 for 2nd, … where N is the number of players with Side Order data; Eight's Palette counts double), plus one comparison for the highest score. Contestants without the Side Order DLC are left out of this page.

The `freshness-league` container talks to the sidecar at `http://nxapi:2727`. All `nxapi` state (`/data/persist`) lives in the `nxapi-data` volume on the sidecar, so multiple `freshness-league` instances pointed at the same sidecar share one token cache.

### Sidecar HTTP API

The bridge intentionally exposes a tiny surface (the rest of `nxapi` is not reachable from outside the sidecar):

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/user?token=<session_token>` | Runs `nxapi nso user` and extracts the NSO/Coral display name + Mii avatar URL, returned as `{nsoName, nsoImage}` |
| `GET` | `/api/splatnet3/records?token=<session_token>` | Runs `nxapi splatnet3 dump-records --hero --history` and returns the raw GraphQL payloads as `{heroResult, historyPlayer}` |
| `GET` | `/api/splatnet3/side-order?token=<session_token>` | Gets a SplatNet 3 bullet token from `nxapi splatnet3 token --json`, then runs SplatNet 3's `HistoryRecordQuery` and Side Order queries itself (nxapi doesn't know the Side Order queries): palettes, results/high score, color chips, field guide, and the run details (chip board, hacks, date) of each palette's fastest run and the high-score run. Run details never change, so the sidecar caches them for up to a year, on disk in the `nxapi-data` volume (`/data/side-order-runs`); a run is re-fetched a week before the signed image links inside it expire. Returns `{historyPlayer, sideOrderPalettes, sideOrderChallenge, sideOrderColorChips, sideOrderEnemies, tryDetails}`; the Side Order fields are `null` for players without the DLC |

## Running the Go app directly

The app talks to the sidecar over HTTP and has no other runtime dependencies. Make sure the bridge is reachable (either run `docker compose up nxapi` or run the bridge yourself with `go run ./nxapi-bridge` plus a local `nxapi` install).

```
Usage of ./alterna-freshness-league:
  -league value
        Contestants json string; falls back to CONTESTANTS env var, then 'contestants.json'
  -port string
        Port to bind to (default "8080")
  -proxy
        Start in proxy mode
  -sidecar-url string
        Base URL for the nxapi-bridge sidecar (default "http://127.0.0.1:2727")
```

Note: Nintendo authentication tokens are sent through the `nxapi` sidecar, which forwards relevant data to `nxapi-znca-api.fancy.org.uk` (Coral `f` generation). Review the upstream terms in the [nxapi-znca-api docs](https://github.com/samuelthomas2774/nxapi-znca-api/blob/docs/docs/public-api-terms.md).

## What does it look like?

The Alterna league (`/`):

![Alterna league](alterna%20freshness%20league.png)

The Side Order league (`/order`):

![Side Order league](side%20order%20league.png)
