# AWS-managed policies (stable, account-agnostic): cache the immutable assets,
# never cache the API, and forward everything-except-Host to the Function URL
# origin (forwarding the viewer Host would break the Lambda Function URL).
data "aws_cloudfront_cache_policy" "optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_cache_policy" "disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_viewer_except_host" {
  name = "Managed-AllViewerExceptHostHeader"
}

# --- Lambda (the API + embedded SPA) --------------------------------------------

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.name}-demo"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

# Read-only demo needs nothing but CloudWatch Logs -- no DynamoDB, no S3.
resource "aws_iam_role_policy_attachment" "logs" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "api" {
  function_name    = "${var.name}-demo"
  role             = aws_iam_role.lambda.arn
  filename         = var.lambda_zip
  source_code_hash = filebase64sha256(var.lambda_zip)
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = var.memory_size
  timeout          = var.timeout

  environment {
    variables = var.environment
  }
}

# Public Function URL: the origin CloudFront fronts. Harmless to hit directly on
# a read-only demo; hardening with Origin Access Control (auth_type = AWS_IAM +
# a SigV4-signing CloudFront OAC) is a later option.
resource "aws_lambda_function_url" "api" {
  function_name      = aws_lambda_function.api.function_name
  authorization_type = "NONE"
}

locals {
  origin_domain = replace(replace(aws_lambda_function_url.api.function_url, "https://", ""), "/", "")
}

# --- CloudFront -----------------------------------------------------------------

resource "aws_cloudfront_distribution" "cdn" {
  enabled         = true
  is_ipv6_enabled = true
  comment         = "${var.name} read-only lcatd demo"
  price_class     = var.price_class
  aliases         = var.aliases

  origin {
    domain_name = local.origin_domain
    origin_id   = "lambda"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  # HTML + SPA client routes: always fresh from the origin so a redeploy shows
  # immediately (and the history-API fallback returns index.html).
  default_cache_behavior {
    target_origin_id         = "lambda"
    viewer_protocol_policy   = "redirect-to-https"
    allowed_methods          = ["GET", "HEAD", "OPTIONS"]
    cached_methods           = ["GET", "HEAD"]
    compress                 = true
    cache_policy_id          = data.aws_cloudfront_cache_policy.disabled.id
    origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
  }

  # Hashed, immutable build assets: cache hard at the edge so page loads do not
  # wake Lambda. A new deploy ships new hashes, so nothing goes stale.
  ordered_cache_behavior {
    path_pattern           = "/assets/*"
    target_origin_id       = "lambda"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true
    cache_policy_id        = data.aws_cloudfront_cache_policy.optimized.id
  }

  # The JSON API: never cache; forward auth headers, cookies, and query (but not
  # Host) to the function. All methods, since edits POST/PUT/DELETE (the backend
  # rejects writes in read-only mode).
  ordered_cache_behavior {
    path_pattern             = "/v1/*"
    target_origin_id         = "lambda"
    viewer_protocol_policy   = "redirect-to-https"
    allowed_methods          = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods           = ["GET", "HEAD"]
    compress                 = true
    cache_policy_id          = data.aws_cloudfront_cache_policy.disabled.id
    origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
  }

  # Boot config: small and may change on redeploy, so never cache it.
  ordered_cache_behavior {
    path_pattern             = "/config"
    target_origin_id         = "lambda"
    viewer_protocol_policy   = "redirect-to-https"
    allowed_methods          = ["GET", "HEAD", "OPTIONS"]
    cached_methods           = ["GET", "HEAD"]
    compress                 = true
    cache_policy_id          = data.aws_cloudfront_cache_policy.disabled.id
    origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = var.acm_certificate_arn == ""
    acm_certificate_arn            = var.acm_certificate_arn == "" ? null : var.acm_certificate_arn
    ssl_support_method             = var.acm_certificate_arn == "" ? null : "sni-only"
    minimum_protocol_version       = var.acm_certificate_arn == "" ? "TLSv1" : "TLSv1.2_2021"
  }
}
