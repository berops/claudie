terraform {
  backend "s3" {
    key            = "{{ .ProjectName }}/{{ .ClusterName }}"
    region         = "{{or .Region "main" }}"
    bucket         = "{{ .BucketName }}"
    dynamodb_table = "{{ .DynamoTable }}"
    
    access_key = "{{ .AccessKey }}"
    secret_key = "{{ .SecretKey }}"

    endpoints = {
    {{if .BucketURL }}s3 = "{{ .BucketURL }}"{{ end }}
    {{if .DynamoURL }}dynamodb = "{{ .DynamoURL }}"{{end}}
    }

    skip_credentials_validation = true
    skip_requesting_account_id  = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    use_path_style            = true
  }
}