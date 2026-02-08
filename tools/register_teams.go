package tools

import (
	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// RegisterTeamTools registers all team-specific tools for a member.
// This should be called per member, as SendMessage/Broadcast/Shutdown
// need the member's name as the sender identity.
func RegisterTeamTools(registry *agent.ToolRegistry, bus *teams.MessageBus, tasks *teams.SharedTaskList, memberName string) {
	agent.RegisterTool(registry, NewSendMessageTool(bus, memberName))
	agent.RegisterTool(registry, NewBroadcastTool(bus, memberName))
	agent.RegisterTool(registry, NewTaskCreateTool(tasks))
	agent.RegisterTool(registry, NewTaskListTool(tasks))
	agent.RegisterTool(registry, NewTaskUpdateTool(tasks))
	agent.RegisterTool(registry, NewTaskGetTool(tasks))
	agent.RegisterTool(registry, NewShutdownRequestTool(bus, memberName))
}
