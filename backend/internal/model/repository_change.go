package model

import "time"

// RepositoryChange はRepository内の増分同期イベント。
type RepositoryChange struct {
	Sequence   int64
	GroupID    int64
	EntityType string
	EntityID   int64
	PostID     *int64
	Operation  string
	ChangedAt  time.Time
}
