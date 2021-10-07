terraform {
  backend "gcs" {
    bucket      = "develop_test_bucket"
    prefix      = "/customer/{{ .ProjectName }}/{{ .ClusterName }}"
  }
}