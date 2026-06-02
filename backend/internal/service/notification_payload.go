package service

import (
	"strconv"

	"github.com/irj0927/begit/pkg/fcm"
)

// Payload は FCM 送信用の通知ペイロード。
// Notification は表示用（title/body）、Data は ios-guide §2 準拠の data メッセージ（全値文字列）。
type Payload struct {
	Notification fcm.Notification
	Data         map[string]string
}

// s は int64 を文字列化する（FCM data は全値文字列のため）
func s(v int64) string {
	return strconv.FormatInt(v, 10)
}

// BuildBeGitTime は ① begit_time のペイロードを構築する。
// data: type, group_id, notification_id, sprint_id
func BuildBeGitTime(groupID, notifID, sprintID int64) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "BeGit Time!",
			Body:  "今なに作ってる？チームへの通知が届きました",
		},
		Data: map[string]string{
			"type":            "begit_time",
			"group_id":        s(groupID),
			"notification_id": s(notifID),
			"sprint_id":       s(sprintID),
		},
	}
}

// BuildNiceWork は ② nice_work のペイロードを構築する。
// data: type, group_id, notification_id(anchor), draft_post_id, status(on_time|late)
func BuildNiceWork(groupID, notifID, draftPostID int64, status string) Payload {
	body := "いい仕事！写真を撮って投稿しよう"
	return Payload{
		Notification: fcm.Notification{
			Title: "Nice Work!",
			Body:  body,
		},
		Data: map[string]string{
			"type":            "nice_work",
			"group_id":        s(groupID),
			"notification_id": s(notifID),
			"draft_post_id":   s(draftPostID),
			"status":          status,
		},
	}
}

// BuildChallengeEnd は ③ challenge_end のペイロードを構築する。
// data: type, group_id, notification_id
func BuildChallengeEnd(groupID, notifID int64) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "チャレンジ終了",
			Body:  "結果が出ました。みんなの様子を見てみよう",
		},
		Data: map[string]string{
			"type":            "challenge_end",
			"group_id":        s(groupID),
			"notification_id": s(notifID),
		},
	}
}

// BuildSprintReminder は ④ sprint_reminder のペイロードを構築する。
// data: type, group_id, sprint_id
func BuildSprintReminder(groupID, sprintID int64) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "スプリント終了3日前",
			Body:  "ラストスパート！残り3日です",
		},
		Data: map[string]string{
			"type":      "sprint_reminder",
			"group_id":  s(groupID),
			"sprint_id": s(sprintID),
		},
	}
}

// BuildSprintEnd は ⑤ sprint_end のペイロードを構築する。
// data: type, group_id, sprint_id
func BuildSprintEnd(groupID, sprintID int64) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "スプリント終了",
			Body:  "今回の結果をチェックしよう",
		},
		Data: map[string]string{
			"type":      "sprint_end",
			"group_id":  s(groupID),
			"sprint_id": s(sprintID),
		},
	}
}

// BuildSprintStart は ⑥ sprint_start のペイロードを構築する。
// data: type, group_id, sprint_id(新スプリント)
func BuildSprintStart(groupID, sprintID int64) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "新スプリント開始",
			Body:  "新しいスプリントが始まりました",
		},
		Data: map[string]string{
			"type":      "sprint_start",
			"group_id":  s(groupID),
			"sprint_id": s(sprintID),
		},
	}
}

// BuildReaction は ⑦ reaction のペイロードを構築する。
// data: type, group_id, post_id, actor_login
func BuildReaction(groupID, postID int64, actorLogin string) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "リアクションが届きました",
			Body:  actorLogin + " があなたの投稿に反応しました",
		},
		Data: map[string]string{
			"type":        "reaction",
			"group_id":    s(groupID),
			"post_id":     s(postID),
			"actor_login": actorLogin,
		},
	}
}

// BuildComment は ⑦ comment のペイロードを構築する。
// data: type, group_id, post_id, actor_login
func BuildComment(groupID, postID int64, actorLogin string) Payload {
	return Payload{
		Notification: fcm.Notification{
			Title: "コメントが届きました",
			Body:  actorLogin + " があなたの投稿にコメントしました",
		},
		Data: map[string]string{
			"type":        "comment",
			"group_id":    s(groupID),
			"post_id":     s(postID),
			"actor_login": actorLogin,
		},
	}
}
