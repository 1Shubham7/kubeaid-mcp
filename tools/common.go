package tools

// contextInput is embedded in every tool's input to let a caller target a
// specific kubeconfig context. Omitting it uses the server's default context.
type contextInput struct {
	Context string `json:"context,omitempty" jsonschema:"kubeconfig context (cluster) to target; omit to use the default context"`
}
