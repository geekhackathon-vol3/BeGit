resource "cloudflare_d1_database" "begit_db" {
  account_id = var.cloudflare_account_id
  name       = "begit-db"
}
