terraform {
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5"
    }
  }

  backend "local" {}
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
