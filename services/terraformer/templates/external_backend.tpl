terraform {
  backend "s3" {
    bucket  = "{{ .S3Name }}"
    key     = "{{ .ProjectName }}/{{ .ClusterName }}"

    region   = "{{ .Region }}"
    dynamodb_table    = "{{ .DynamoTable }}"
    
    access_key = "{{ .AwsAccessKey }}"
    secret_key = "{{ .AwsSecretKey }}"

    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    force_path_style            = true
  }
}