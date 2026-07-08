resource "tpuf_namespace" "docs" {
  name = "docs"

  schema = {
    text = {
      type             = "string"
      full_text_search = true
    }
    embedding = {
      type = "[1536]f32"
      ann  = true
    }
    source = {
      type       = "string"
      filterable = true
    }
  }

  pinning_replicas = 1
}
