terraform {
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4"
    }
  }

  backend "local" {}
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
