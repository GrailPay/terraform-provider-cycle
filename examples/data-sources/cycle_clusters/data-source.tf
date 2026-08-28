data "cycle_clusters" "all" {}

output "cluster_identifiers" {
  value = [for cluster in data.cycle_clusters.all.clusters : cluster.identifier]
}
