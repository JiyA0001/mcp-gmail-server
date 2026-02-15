package mcp

func mergeResults(target, source map[string]interface{}) {
	for key, newVal := range source {

		// If key does not exist yet → just set it
		if _, exists := target[key]; !exists {
			target[key] = newVal
			continue
		}

		oldVal := target[key]

		// CASE 1: Both are arrays → APPEND
		oldArr, oldOk := oldVal.([]interface{})
		newArr, newOk := newVal.([]interface{})
		if oldOk && newOk {
			target[key] = append(oldArr, newArr...)
			continue
		}

		// CASE 2: Anything else → keep old (do NOT overwrite)
	}
}
