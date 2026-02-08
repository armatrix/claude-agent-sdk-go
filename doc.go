// Package agent provides a pure Go implementation of the Claude Agent SDK.
//
// The SDK directly calls the Anthropic API via anthropic-sdk-go with no
// Claude Code binary dependency. It provides two main entry points:
//
//   - [Agent] is a stateless execution engine that holds config + tools.
//   - [Client] is a stateful session container wrapping an Agent.
//
// # Quick Start
//
//	a := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeSonnet4_5))
//	tools.RegisterAll(a.Tools())
//	stream := a.Run(ctx, "Hello, what files are in this directory?")
//	for stream.Next() {
//	    if e, ok := stream.Current().(*agent.StreamEvent); ok {
//	        fmt.Print(e.Delta)
//	    }
//	}
//
// # Sub-packages
//
//   - [tools] provides built-in tools (Read, Write, Edit, Bash, Glob, Grep).
//   - [session] provides SessionStore implementations (FileStore, MemoryStore).
//   - [hook] provides hook types for intercepting tool execution.
//   - [permission] provides permission types for access control.
package agent
