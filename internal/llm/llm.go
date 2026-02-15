package llm

type Client interface {
	Extract(prompt string) (string, error)
}
