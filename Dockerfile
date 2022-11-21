FROM golang:1.17-alpine
RUN apk add --no-cache jq
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download && go mod verify

COPY datahandling ./datahandling
COPY web ./web
COPY main.go ./

RUN ls -al
RUN go build

ENV CONTESTANTS={}
ENV PROXY=false
EXPOSE 8080

ENTRYPOINT ./alterna-freshness-league --proxy=${PROXY} --league=${CONTESTANTS}