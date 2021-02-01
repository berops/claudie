terraform {
  backend "gcs" {
    bucket      = "develop_test_bucket"
    prefix      = "/customer/{{ .Metadata.Name }}"
    credentials = "../keys/platform-296509-d6ddeb344e91.json"
  }
}