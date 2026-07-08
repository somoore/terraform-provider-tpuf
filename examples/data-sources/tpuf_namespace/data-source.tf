data "tpuf_namespace" "docs" {
  name = "docs"
}

output "docs_schema" {
  value = data.tpuf_namespace.docs.schema
}
