module fortio.org/terminal

go 1.22.7

require (
	fortio.org/cli v1.9.0
	fortio.org/fortio v1.66.5
	fortio.org/log v1.16.0
	fortio.org/safecast v1.0.0
	fortio.org/term v0.23.0-fortio-6
	github.com/loov/hrtime v1.0.3
	golang.org/x/image v0.20.0
	golang.org/x/sys v0.25.0
)

// replace fortio.org/term => ../term

require (
	fortio.org/struct2env v0.4.1 // indirect
	fortio.org/version v1.0.4 // indirect
	github.com/kortschak/goroutine v1.1.2 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20240916204253-42ee18b96377 // indirect
)
