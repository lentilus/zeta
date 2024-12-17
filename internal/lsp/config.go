package lsp

type Config struct {
	ReferenceQuery     string `json:"reference_query"     required:"true"`
	TargetRegex        string `json:"target_regex"        required:"true"`
	PathSeparator      string `json:"path_separator"      required:"true"`
	CanonicalExtension string `json:"canonical_extension" required:"true"`
}
