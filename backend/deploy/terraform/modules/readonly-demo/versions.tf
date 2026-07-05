# Reusable module: a read-only lcatd demo on Lambda (Function URL) behind
# CloudFront. No DynamoDB/S3 -- the in-memory document store plus grains bundled
# in the Lambda zip. CloudFront caches the SPA's hashed /assets/* at the edge and
# passes /config + /v1/* through to the function, so cold starts are only felt on
# API calls. A caller (e.g. the demo site) supplies the built zip and, optionally,
# a custom domain + ACM cert.
#
# NOTE: CloudFront is a global service; an ACM certificate for a custom alias
# MUST live in us-east-1. Pass its ARN via acm_certificate_arn.

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.60"
    }
  }
}
