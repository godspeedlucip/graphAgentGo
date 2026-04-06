package memory

func CacheKey(conversationID string) string {
	return "chat:memory:" + conversationID
}

func IsValidRole(role Role) bool {
	switch role {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	default:
		return false
	}
}

func HasToolCalls(metadata *Metadata) bool {
	return metadata != nil && len(metadata.ToolCalls) > 0
}

func HasToolResponse(metadata *Metadata) bool {
	return metadata != nil && metadata.ToolResponse != nil
}
