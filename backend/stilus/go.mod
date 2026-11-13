module github.com/timzifer/figure/backend/stilus

// go-opentype/opentype requires Go 1.27.1, so this module does too. The core
// and the gg backend stay on their own floor; nothing here raises it for them.
go 1.27.1

require (
	github.com/go-opentype/fonts v0.14.0
	github.com/go-opentype/opentype v0.16.0
	github.com/timzifer/figure v0.14.0
	github.com/timzifer/stilus v0.18.0
)

require github.com/andybalholm/brotli v1.2.6 // indirect
