module fortio.org/terminal

go 1.22.6

require (
	fortio.org/cli v1.8.0
	fortio.org/log v1.16.0
	golang.org/x/term v0.23.0
)

//nolint:gomoddirectives // nolint should work even in go.mod imnsho
replace golang.org/x/term => github.com/ldemailly/term v0.0.0-20240809182630-68dd89eaaf9e

require (
	fortio.org/struct2env v0.4.1 // indirect
	fortio.org/version v1.0.4 // indirect
	github.com/kortschak/goroutine v1.1.2 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20240806160748-b2d3a6a4b4d3 // indirect
	golang.org/x/sys v0.24.0 // indirect
)
