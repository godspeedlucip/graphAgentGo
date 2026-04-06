package agentruntime

import "strings"

func FilterToolsByAllowList(all []ToolDef, allowed []string) []ToolDef {
	if len(allowed) == 0 {
		return DeduplicateTools(all)
	}
	set := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		set[key] = struct{}{}
	}
	out := make([]ToolDef, 0, len(all))
	for _, t := range all {
		if _, ok := set[t.Name]; ok {
			out = append(out, t)
		}
	}
	return DeduplicateTools(out)
}

func DeduplicateTools(tools []ToolDef) []ToolDef {
	if len(tools) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tools))
	out := make([]ToolDef, 0, len(tools))
	for _, t := range tools {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, t)
	}
	return out
}
