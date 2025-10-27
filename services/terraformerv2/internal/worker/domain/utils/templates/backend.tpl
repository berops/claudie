terraform {
  backend "s3" {
    key            = "{{ .ProjectName }}/{{ .ClusterName }}"
    region         = "{{or .Region "main" }}"
    bucket         = "{{ .BucketName }}"
    dynamodb_table = "{{ .DynamoTable }}"
    
    access_key = "{{ .AccessKey }}"
    secret_key = "{{ .SecretKey }}"

    {{if .BucketURL }}endpoint = "{{ .BucketURL }}"{{ end }}
    {{if .DynamoURL }}dynamodb_endpoint = "{{ .DynamoURL }}"{{end}}

    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    force_path_style            = true
  }
}