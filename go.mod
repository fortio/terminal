module fortio.org/terminal

go 1.23.8

retract v0.28.0 // accidental tag, not sure where from

require (
	fortio.org/cli v1.10.0
	fortio.org/log v1.17.2
	fortio.org/safecast v1.0.0
	github.com/rivo/uniseg v0.4.7
	golang.org/x/image v0.26.0
	golang.org/x/sys v0.32.0
	golang.org/x/term v0.32.0
)

// Remove once 0.32.0 is released / https://github.com/golang/term/pull/20 merged
replace golang.org/x/term => github.com/ldemailly/term v0.0.0-20250321061617-b9f8be7d8922

require (
	fortio.org/struct2env v0.4.2 // indirect
	fortio.org/version v1.0.4 // indirect
	github.com/kortschak/goroutine v1.1.2 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20250406160420-959f8f3db0fb // indirect
)
