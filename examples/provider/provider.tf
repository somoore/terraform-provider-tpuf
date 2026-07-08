provider "tpuf" {
  # Or set TURBOPUFFER_API_KEY / TURBOPUFFER_REGION environment variables.
  api_key = var.turbopuffer_api_key
  region  = "gcp-us-central1"
}
