# alterna-freshness-league
This is little tool that compares your splatoon 3 single player times with those of your friends. Uses the amazing imink f-api by @JoneWang https://github.com/imink-app/f-API

To use first create a `contestants.json` with the following structure:
```
{
    "league": "leagueName",
    "contestants": [
        {
            "name":"tomte",
            "session_token":"session_token"
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


To run a server:

 ```
 docker run -d --restart=always --env CONTESTANTS=`jq -c . contestants.json` -p <port>:8080 ghcr.io/ragadeeshu/alterna-freshness-league:latest
 ```

Or, to run a proxy:

 ```
 docker run -d --restart=always --env CONTESTANTS=`jq -c . contestants.json` --env PROXY=true -p <port>:8080 ghcr.io/ragadeeshu/alterna-freshness-league:latest
 ```

 You could of course build/run the go app yourself, if you feel like it. Arguments are
 ```
 Usage of ./alterna-freshness-league:
  -league value
        Contestants json string, tries to read 'contestants.json' file if not set
  -port string
        Port to bind to (default "8080")
  -proxy
        Start in proxy mode
 ```