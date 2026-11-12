package ui

import (
	"cmdban/internal/commentread"
	"cmdban/internal/task"
)

const unreadCommentsMarker = "💬 "

func loadCommentReads() (*commentread.Store, error) {
	path := commentread.DefaultStorePath()
	store, err := commentread.Load(path)
	if err != nil {
		return commentread.New(path), err
	}
	return store, nil
}

func (m Model) adoOrg() string {
	return m.config.ActiveBoard().AzureDevOps.Org
}

func (m Model) hasUnreadComments(t *task.Task) bool {
	if t.ADOItemID == 0 || !m.config.IsADOBoard() {
		return false
	}
	return m.commentReads.HasUnread(m.adoOrg(), t.ADOItemID, t.ADOCommentCount)
}

func (m Model) unreadCommentsTaskCount() int {
	count := 0
	for _, lane := range m.board.Lanes {
		for _, t := range m.board.Tasks[lane] {
			if m.hasUnreadComments(t) {
				count++
			}
		}
	}
	return count
}

// observeCommentCounts records the synced comment counts so comments added
// after the first sync of a work item show up as unread.
func (m Model) observeCommentCounts(tasks []*task.Task) {
	org := m.adoOrg()
	changed := false
	for _, t := range tasks {
		if t.ADOItemID != 0 && m.commentReads.Observe(org, t.ADOItemID, t.ADOCommentCount) {
			changed = true
		}
	}
	if changed {
		m.saveCommentReads()
	}
}

func (m Model) markCommentsRead(workItemID, commentCount int) {
	if m.commentReads.MarkRead(m.adoOrg(), workItemID, commentCount) {
		m.saveCommentReads()
	}
}

// saveCommentReads only logs failures: losing read state is not worth
// interrupting the user over.
func (m Model) saveCommentReads() {
	if err := m.commentReads.Save(); err != nil {
		m.logs.Error("Error saving comment read state: " + err.Error())
	}
}
