-- ============================================================
-- GitHub App installations
--
-- GitHub Appは個人アカウントまたは組織ごとにInstallation IDを持つ。
-- リポジトリ追加時に、どのInstallation Tokenを使うかを特定するために保存する。
-- ============================================================

CREATE TABLE IF NOT EXISTS github_app_installations (
  id                    INTEGER PRIMARY KEY AUTOINCREMENT,
  installation_id       INTEGER NOT NULL UNIQUE,
  account_id            INTEGER NOT NULL,
  account_login         TEXT    NOT NULL,
  account_type          TEXT    NOT NULL, -- 'User' | 'Organization'
  repository_selection  TEXT    NOT NULL DEFAULT 'selected', -- 'all' | 'selected'
  installed_by_user_id  INTEGER REFERENCES users(id),
  created_at            TEXT    NOT NULL DEFAULT (datetime('now')),
  updated_at            TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_github_app_installations_account_login
  ON github_app_installations(account_login);

CREATE INDEX IF NOT EXISTS idx_github_app_installations_installed_by_user
  ON github_app_installations(installed_by_user_id);
