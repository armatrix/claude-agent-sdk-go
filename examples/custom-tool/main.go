// Command custom-tool demonstrates a custom Tool[T], hooks, permissions,
// and structured output extraction.
package main

import (
	"context"
	"fmt"
	"os"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/hook"
	"github.com/armatrix/claude-agent-sdk-go/permission"

	"github.com/anthropics/anthropic-sdk-go"
)

// WeatherInput is the typed input for the weather tool.
type WeatherInput struct {
	City string `json:"city" jsonschema:"description=City name to get weather for"`
}

// weatherTool implements agent.Tool[WeatherInput].
type weatherTool struct{}

func (w *weatherTool) Name() string        { return "get_weather" }
func (w *weatherTool) Description() string { return "Get current weather for a city" }

func (w *weatherTool) Execute(_ context.Context, input WeatherInput) (*agent.ToolResult, error) {
	// Stub: return fake weather data
	return agent.TextResult(fmt.Sprintf("Weather in %s: 22°C, sunny", input.City)), nil
}

// WeatherReport is the structured output format.
type WeatherReport struct {
	City        string `json:"city" jsonschema:"description=City name"`
	Temperature int    `json:"temperature" jsonschema:"description=Temperature in Celsius"`
	Condition   string `json:"condition" jsonschema:"description=Weather condition"`
}

func main() {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "set ANTHROPIC_API_KEY to run this example")
		os.Exit(1)
	}

	// Hook: log before and after tool use
	preLog := hook.Matcher{
		Event: hook.PreToolUse,
		Hooks: []hook.Func{
			func(_ context.Context, input *hook.Input) (*hook.Result, error) {
				fmt.Fprintf(os.Stderr, "[hook] pre-tool: %s\n", input.ToolName)
				return nil, nil
			},
		},
	}
	postLog := hook.Matcher{
		Event: hook.PostToolUse,
		Hooks: []hook.Func{
			func(_ context.Context, input *hook.Input) (*hook.Result, error) {
				fmt.Fprintf(os.Stderr, "[hook] post-tool: %s output=%q\n",
					input.ToolName, input.ToolOutput)
				return nil, nil
			},
		},
	}

	a := agent.NewAgent(
		agent.WithModel(anthropic.ModelClaudeSonnet4_5),
		agent.WithMaxTurns(3),
		agent.WithSystemPrompt("You are a weather assistant. Use the get_weather tool, then return a structured report."),
		agent.WithHooks(preLog, postLog),
		agent.WithPermissionMode(permission.ModeAcceptEdits),
		agent.WithOutputFormatType[WeatherReport]("weather_report"),
	)

	// Register custom tool
	agent.RegisterTool(a.Tools(), &weatherTool{})

	stream := a.Run(context.Background(), "What's the weather in Tokyo?")

	var lastMsg anthropic.Message
	for stream.Next() {
		switch ev := stream.Current().(type) {
		case *agent.StreamEvent:
			fmt.Print(ev.Delta)
		case *agent.AssistantEvent:
			lastMsg = ev.Message
		case *agent.ResultEvent:
			if ev.IsError {
				fmt.Fprintf(os.Stderr, "\nerror: %s\n", ev.Result)
				os.Exit(1)
			}
		}
	}
	if err := stream.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stream error: %v\n", err)
		os.Exit(1)
	}

	// Extract structured output from the last message
	report, err := agent.ExtractStructuredOutputTyped[WeatherReport](lastMsg, "weather_report")
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nStructured report: city=%s temp=%d°C condition=%s\n",
		report.City, report.Temperature, report.Condition)
}
