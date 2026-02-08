// Command multi-turn demonstrates a stateful Client with conversation history.
package main

import (
	"context"
	"fmt"
	"os"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/session"

	"github.com/anthropics/anthropic-sdk-go"
)

func main() {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "set ANTHROPIC_API_KEY to run this example")
		os.Exit(1)
	}

	store := session.NewMemoryStore()
	client := agent.NewClient(
		agent.WithModel(anthropic.ModelClaudeSonnet4_5),
		agent.WithMaxTurns(1),
		agent.WithSystemPrompt("You are a helpful assistant. Be concise."),
		agent.WithSessionStore(store),
	)
	defer client.Close()

	// First query
	fmt.Println("=== Turn 1 ===")
	drain(client.Query(context.Background(), "My name is Alice. Remember it."))

	// Second query — the agent should remember the name
	fmt.Println("\n=== Turn 2 ===")
	drain(client.Query(context.Background(), "What is my name?"))

	fmt.Printf("\nsession %s has %d messages\n",
		client.Session().ID, len(client.Session().Messages))
}

func drain(stream *agent.AgentStream) {
	for stream.Next() {
		switch ev := stream.Current().(type) {
		case *agent.StreamEvent:
			fmt.Print(ev.Delta)
		case *agent.ResultEvent:
			if ev.IsError {
				fmt.Fprintf(os.Stderr, "\nerror: %s\n", ev.Result)
			}
		}
	}
	if err := stream.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stream error: %v\n", err)
	}
}
