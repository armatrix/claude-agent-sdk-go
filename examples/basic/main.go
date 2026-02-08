// Command basic demonstrates a minimal single-shot agent that streams a response.
package main

import (
	"context"
	"fmt"
	"os"

	agent "github.com/armatrix/claude-agent-sdk-go"

	"github.com/anthropics/anthropic-sdk-go"
)

func main() {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "set ANTHROPIC_API_KEY to run this example")
		os.Exit(1)
	}

	a := agent.NewAgent(
		agent.WithModel(anthropic.ModelClaudeSonnet4_5),
		agent.WithMaxTurns(1),
		agent.WithSystemPrompt("You are a helpful assistant. Be concise."),
	)

	stream := a.Run(context.Background(), "What is the capital of France?")
	for stream.Next() {
		switch ev := stream.Current().(type) {
		case *agent.StreamEvent:
			fmt.Print(ev.Delta)
		case *agent.ResultEvent:
			if ev.IsError {
				fmt.Fprintf(os.Stderr, "\nerror: %s\n", ev.Result)
				os.Exit(1)
			}
			fmt.Printf("\n\n[done] turns=%d input=%d output=%d\n",
				ev.NumTurns, ev.Usage.InputTokens, ev.Usage.OutputTokens)
		}
	}
	if err := stream.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stream error: %v\n", err)
		os.Exit(1)
	}
}
