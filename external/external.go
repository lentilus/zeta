package external

import "embed"

//go:embed index.html _vendor/force-graph.js _vendor/d3.v5.min.js
var Assets embed.FS
