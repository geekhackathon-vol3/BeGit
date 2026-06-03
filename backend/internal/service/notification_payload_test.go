package service

import "testing"

// assertAllStringValues は data の全値が文字列であることを検証する（map[string]string なので型は保証されるが、空でないことを確認）
func assertContains(t *testing.T, data map[string]string, key, want string) {
	t.Helper()
	got, ok := data[key]
	if !ok {
		t.Errorf("missing key %q in data: %v", key, data)
		return
	}
	if got != want {
		t.Errorf("data[%q] = %q, want %q", key, got, want)
	}
}

// TestBuildBeGitTime は ① begit_time の data フィールドが ios-guide §2 と一致することを検証する
func TestBuildBeGitTime(t *testing.T) {
	p := BuildBeGitTime(12, 345, 7)
	assertContains(t, p.Data, "type", "begit_time")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "notification_id", "345")
	assertContains(t, p.Data, "sprint_id", "7")
	if p.Notification.Title == "" {
		t.Error("expected non-empty notification title")
	}
}

// TestBuildNiceWork は ② nice_work の data フィールドが ios-guide §2 と一致することを検証する
func TestBuildNiceWork(t *testing.T) {
	p := BuildNiceWork(12, 345, 890, "on_time")
	assertContains(t, p.Data, "type", "nice_work")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "notification_id", "345")
	assertContains(t, p.Data, "draft_post_id", "890")
	assertContains(t, p.Data, "status", "on_time")
}

// TestBuildChallengeEnd は ③ challenge_end の data フィールドを検証する
func TestBuildChallengeEnd(t *testing.T) {
	p := BuildChallengeEnd(12, 345)
	assertContains(t, p.Data, "type", "challenge_end")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "notification_id", "345")
}

// TestBuildSprintReminder は ④ sprint_reminder の data フィールドを検証する
func TestBuildSprintReminder(t *testing.T) {
	p := BuildSprintReminder(12, 7)
	assertContains(t, p.Data, "type", "sprint_reminder")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "sprint_id", "7")
}

// TestBuildSprintEnd は ⑤ sprint_end の data フィールドを検証する
func TestBuildSprintEnd(t *testing.T) {
	p := BuildSprintEnd(12, 7)
	assertContains(t, p.Data, "type", "sprint_end")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "sprint_id", "7")
}

// TestBuildSprintStart は ⑥ sprint_start の data フィールドを検証する
func TestBuildSprintStart(t *testing.T) {
	p := BuildSprintStart(12, 8)
	assertContains(t, p.Data, "type", "sprint_start")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "sprint_id", "8")
}

// TestBuildReaction は ⑦ reaction の data フィールドを検証する
func TestBuildReaction(t *testing.T) {
	p := BuildReaction(12, 890, "octocat")
	assertContains(t, p.Data, "type", "reaction")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "post_id", "890")
	assertContains(t, p.Data, "actor_login", "octocat")
}

// TestBuildComment は ⑦ comment の data フィールドを検証する
func TestBuildComment(t *testing.T) {
	p := BuildComment(12, 890, "octocat")
	assertContains(t, p.Data, "type", "comment")
	assertContains(t, p.Data, "group_id", "12")
	assertContains(t, p.Data, "post_id", "890")
	assertContains(t, p.Data, "actor_login", "octocat")
}
