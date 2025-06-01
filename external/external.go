package external

import "embed"

//go:embed index.html _vendor/force-graph.js
var Assets embed.FS
