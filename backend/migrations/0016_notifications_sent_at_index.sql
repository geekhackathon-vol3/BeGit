-- The every-minute bonus check only looks at challenges sent in the last 25 hours.
CREATE INDEX IF NOT EXISTS idx_notifications_sent_at ON notifications(sent_at);
