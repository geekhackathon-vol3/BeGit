resource "cloudflare_r2_bucket" "begit_photos" {
  account_id = var.cloudflare_account_id
  name       = "begit-photos"
}
