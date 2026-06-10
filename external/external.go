package external

import "embed"

//go:embed index.html force-graph.js
var Assets embed.FS
