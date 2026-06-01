output "d1_database_id" {
  description = "D1 Database ID（wrangler.toml の database_id に設定する）"
  value       = cloudflare_d1_database.begit_db.id
}

output "r2_bucket_name" {
  description = "R2 Bucket Name"
  value       = cloudflare_r2_bucket.begit_photos.name
}
