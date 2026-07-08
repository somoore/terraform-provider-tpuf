data "tpuf_namespaces" "prod" {
  prefix = "prod-"
}

output "prod_namespaces" {
  value = data.tpuf_namespaces.prod.names
}
