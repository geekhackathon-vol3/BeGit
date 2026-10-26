-- Repository単位の増分同期カーソル。
-- sequenceはD1内で単調増加し、FCMを取りこぼしても次回のpollで追いつける。
CREATE TABLE repository_changes (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL REFERENCES groups(id),
  entity_type TEXT NOT NULL CHECK (entity_type IN ('post', 'photo', 'reaction', 'notification', 'notification_member', 'notification_result', 'member')),
  entity_id INTEGER NOT NULL,
  post_id INTEGER,
  operation TEXT NOT NULL CHECK (operation IN ('upsert', 'delete')),
  changed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX repository_changes_group_sequence
  ON repository_changes(group_id, sequence);

CREATE TRIGGER repository_change_post_insert AFTER INSERT ON posts BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  VALUES (NEW.group_id, 'post', NEW.id, NEW.id, 'upsert');
END;
CREATE TRIGGER repository_change_post_update AFTER UPDATE ON posts BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  VALUES (NEW.group_id, 'post', NEW.id, NEW.id, 'upsert');
END;
CREATE TRIGGER repository_change_post_delete AFTER DELETE ON posts BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  VALUES (OLD.group_id, 'post', OLD.id, OLD.id, 'delete');
END;

CREATE TRIGGER repository_change_photo_insert AFTER INSERT ON photos BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  SELECT p.group_id, 'photo', NEW.id, NEW.post_id, 'upsert' FROM posts p WHERE p.id = NEW.post_id;
END;
CREATE TRIGGER repository_change_photo_update AFTER UPDATE ON photos BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  SELECT p.group_id, 'photo', NEW.id, NEW.post_id, 'upsert' FROM posts p WHERE p.id = NEW.post_id;
END;
CREATE TRIGGER repository_change_photo_delete BEFORE DELETE ON photos BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  SELECT p.group_id, 'photo', OLD.id, OLD.post_id, 'upsert' FROM posts p WHERE p.id = OLD.post_id;
END;

CREATE TRIGGER repository_change_reaction_insert AFTER INSERT ON reactions BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  SELECT p.group_id, 'reaction', NEW.id, NEW.post_id, 'upsert' FROM posts p WHERE p.id = NEW.post_id;
END;
CREATE TRIGGER repository_change_reaction_delete BEFORE DELETE ON reactions BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, post_id, operation)
  SELECT p.group_id, 'reaction', OLD.id, OLD.post_id, 'upsert' FROM posts p WHERE p.id = OLD.post_id;
END;

CREATE TRIGGER repository_change_notification_insert AFTER INSERT ON notifications BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  SELECT s.group_id, 'notification', NEW.id, 'upsert' FROM sprints s WHERE s.id = NEW.sprint_id;
END;
CREATE TRIGGER repository_change_notification_update AFTER UPDATE ON notifications BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  SELECT s.group_id, 'notification', NEW.id, 'upsert' FROM sprints s WHERE s.id = NEW.sprint_id;
END;

CREATE TRIGGER repository_change_notification_member_insert AFTER INSERT ON notification_members BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  SELECT s.group_id, 'notification_member', NEW.user_id, 'upsert'
  FROM notifications n JOIN sprints s ON s.id = n.sprint_id WHERE n.id = NEW.notification_id;
END;
CREATE TRIGGER repository_change_notification_member_delete BEFORE DELETE ON notification_members BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  SELECT s.group_id, 'notification_member', OLD.user_id, 'delete'
  FROM notifications n JOIN sprints s ON s.id = n.sprint_id WHERE n.id = OLD.notification_id;
END;

CREATE TRIGGER repository_change_notification_result_insert AFTER INSERT ON notification_game_results BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  SELECT s.group_id, 'notification_result', NEW.notification_id, 'upsert'
  FROM notifications n JOIN sprints s ON s.id = n.sprint_id WHERE n.id = NEW.notification_id;
END;

CREATE TRIGGER repository_change_member_insert AFTER INSERT ON group_members BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  VALUES (NEW.group_id, 'member', NEW.user_id, 'upsert');
END;
CREATE TRIGGER repository_change_member_update AFTER UPDATE ON group_members BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  VALUES (NEW.group_id, 'member', NEW.user_id, 'upsert');
END;
CREATE TRIGGER repository_change_member_delete BEFORE DELETE ON group_members BEGIN
  INSERT INTO repository_changes(group_id, entity_type, entity_id, operation)
  VALUES (OLD.group_id, 'member', OLD.user_id, 'delete');
END;
