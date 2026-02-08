// Package tools provides built-in tool implementations for the Claude Agent SDK.
//
// Use [RegisterAll] to register the core file and system tools:
//
//	tools.RegisterAll(agent.Tools())
//
// For interactive tools (ask, todo, plan mode), use [RegisterConfigurable]:
//
//	tools.RegisterConfigurable(agent.Tools(), tools.BuiltinOptions{
//	    AskCallback: myAskHandler,
//	})
package tools
