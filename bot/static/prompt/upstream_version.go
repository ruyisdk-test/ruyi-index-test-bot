package prompt

import _ "embed"

//go:embed upstream_version_raw.txt
var UpstreamRawPrompt string

//go:embed upstream_version_oldv.txt
var UpstreamOldVersionPrompt string
