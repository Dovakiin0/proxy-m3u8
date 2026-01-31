# M3U8 proxy

A simple proxy server for M3U8 file written in go for high performance

### Running the server

Redis is optional but can be used to cache the proxied data.

copy the `.env.example` file to `.env`

| NAME           | DESCRIPTION                       | DEFAULT | Required |
| -------------- | --------------------------------- | ------- | -------- |
| PORT           | Port on which the server will run | 4040    | No       |
| CORS_DOMAIN    | Domains that are allowed for cors | \*      | No       |
| REDIS_URL      | Redis url                         |         | No       |
| REDIS_PASSWORD | Password for redis                |         | No       |

add multiple domain separated by comma (,)

`go run cmd/main.go`

or using docker

`docker run --rm -d -p 4040:4040 -e PORT=4040 -e CORS_DOMAIN=localhost:3000 dovakiin0/proxy-m3u8:latest`

or build yourself using Dockerfile

### Usage

Request the proxy server on `/m3u8-proxy?url=<original_m3u8_url>&referer=<referer_url>`. referer is optional
