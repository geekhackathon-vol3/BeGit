variable "cloudflare_account_id" {
  description = "Cloudflare Account ID"
  type        = string
}

variable "cloudflare_api_token" {
  description = "Cloudflare API Token (TF_VAR_cloudflare_api_token 環境変数で渡す)"
  type        = string
  sensitive   = true
}
