package orchestrator

// selectLeastLoadedHost returns the SSH host from hosts that has the fewest
// entries in running. If hosts is empty it returns "".
func selectLeastLoadedHost(hosts []string, running map[string]*RunEntry) string {
	if len(hosts) == 0 {
		return ""
	}
	hostCount := make(map[string]int, len(hosts))
	for _, h := range hosts {
		hostCount[h] = 0
	}
	for _, entry := range running {
		if _, ok := hostCount[entry.WorkerHost]; ok {
			hostCount[entry.WorkerHost]++
		}
	}
	minCount := int(^uint(0) >> 1)
	selected := hosts[0]
	for _, h := range hosts {
		if c := hostCount[h]; c < minCount {
			minCount = c
			selected = h
		}
	}
	return selected
}
