package commentread

// Store remembers, per Azure DevOps work item, how many comments the user
// has seen, so cards can flag comments added since.
type Store struct {
	// Seen maps a work item key (see workItemKey) to its seen comment count.
	Seen     map[string]int `yaml:"seen"`
	filePath string
}
