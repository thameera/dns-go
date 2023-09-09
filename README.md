# dns-go

A DNS client written in Go, similar to `dig`. Attempts to follow the DNS specification at [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035).

## Downloading and building

Requires Go >=1.21.

```sh
git clone git@github.com:thameera/dns-go.git
go build .
```

## Running

```sh
./dns-go [options...] <name> [type]

```

The only available option is `-v` which prints debug output when set.

Supported DNS types are A, AAAA, CNAME, and TXT.

Examples:

```sh
./dns-go example.com
example.com		19959	IN	A	93.184.216.34

./dns-go www.facebook.com
www.facebook.com		783	IN	CNAME	star-mini.c10r.facebook.com
star-mini.c10r.facebook.com		39	IN	A	157.240.8.35

./dns-go example.com txt
example.com		21600	IN	TXT
                                                v=spf1 -all
example.com		21600	IN	TXT	 wgyf8z8cgvm2qmxpnbnldrcltvk4xqfn
```

## Running the tests

```sh
go test
```

Some tests use golden files to keep known good outputs. To update the golden files, run:

```sh
go test -update
```
