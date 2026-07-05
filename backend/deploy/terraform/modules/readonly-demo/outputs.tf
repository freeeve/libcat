output "cloudfront_domain" {
  description = "The distribution's domain (point your CNAME/alias here, or use it directly)."
  value       = aws_cloudfront_distribution.cdn.domain_name
}

output "distribution_id" {
  description = "CloudFront distribution id (for cache invalidations on redeploy)."
  value       = aws_cloudfront_distribution.cdn.id
}

output "function_url" {
  description = "The raw Lambda Function URL (origin; usually reached via CloudFront)."
  value       = aws_lambda_function_url.api.function_url
}

output "function_name" {
  description = "The Lambda function name (for updating code / reading logs)."
  value       = aws_lambda_function.api.function_name
}
