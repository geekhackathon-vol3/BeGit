-- リアクションはアプリ内/FCMだけにし、Discord・Slack向けの購読を解除する。
DELETE FROM notification_channel_subscriptions
WHERE event_type = 'reaction';
