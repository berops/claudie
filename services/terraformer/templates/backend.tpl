terraform {
  backend "gcs" {
    bucket      = "develop_test_bucket"
    prefix      = "/customer/{{ .ProjectName }}/{{ .ClusterName }}"
    credentials = "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json"
  }
}