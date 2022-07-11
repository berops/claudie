terraform {
  backend "s3" {
    bucket      = "claudie-tf-state-files"
    key      = "{{ .ProjectName }}/{{ .ClusterName }}"

    endpoint = "{{.MinioURL}}"
    region = "main"
    
    access_key="{{.AccessKey}}"
    secret_key="{{.SecretKey}}"

    skip_credentials_validation = true
    skip_metadata_api_check = true
    skip_region_validation = true
    force_path_style = true
  }
}