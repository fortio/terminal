module fortio.org/terminal

go 1.23.0

require (
	fortio.org/cli v1.10.0
	fortio.org/log v1.17.2
	fortio.org/safecast v1.0.0
	github.com/rivo/uniseg v0.4.7
	golang.org/x/image v0.29.0
	golang.org/x/sys v0.34.0
	golang.org/x/term v0.33.0
)

replace golang.org/x/term => github.com/ldemailly/term v0.0.0-20250712005731-bc5cb00b388e

require (
	fortio.org/struct2env v0.4.2 // indirect
	fortio.org/version v1.0.4 // indirect
	github.com/kortschak/goroutine v1.1.2 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20250406160420-959f8f3db0fb // indirect
)
