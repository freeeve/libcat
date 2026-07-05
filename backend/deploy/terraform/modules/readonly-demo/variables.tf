variable "name" {
  description = "Name prefix for the Lambda, role, and CloudFront comment."
  type        = string
}

variable "lambda_zip" {
  description = <<-EOT
    Path to the built Lambda deployment zip. Build it with the SPA embedded and
    the read-only grains bundled, e.g.:

      cd backend
      (cd ui && npm ci && npm run build)
      GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap ./cmd/lcatd-lambda
      zip -r lcatd-demo.zip bootstrap grains/

    LCATD_BLOB_DIR (in `environment`) points at the bundled grains dir.
  EOT
  type        = string
}

variable "environment" {
  description = <<-EOT
    LCATD_* environment for the read-only demo. Typically:
      LCATD_READ_ONLY    = "1"
      LCATD_BLOB_DIR     = "/var/task/grains"
      LCATD_LOCAL_AUTH   = "1"
      LCATD_BOOTSTRAP_ADMIN     = "demo@example.org:<password>"
      LCATD_LOCAL_SIGNING_KEY   = "<base64 ed25519 seed>"  # stable so a warm session survives
      LCATD_ABUSE_SECRET        = "<>=16 chars>"
      LCATD_PROVIDER     = "marc"
  EOT
  type        = map(string)
  default     = {}
}

variable "memory_size" {
  description = "Lambda memory (MB). More memory = more CPU = a faster cold-start vocab load."
  type        = number
  default     = 1024
}

variable "timeout" {
  description = "Lambda timeout (seconds)."
  type        = number
  default     = 30
}

variable "aliases" {
  description = "Optional custom domain names for the CloudFront distribution (e.g. [\"try.example.org\"]). Requires acm_certificate_arn."
  type        = list(string)
  default     = []
}

variable "acm_certificate_arn" {
  description = "ARN of an ACM certificate in us-east-1 covering `aliases`. Empty uses the default CloudFront certificate."
  type        = string
  default     = ""
}

variable "price_class" {
  description = "CloudFront price class (edge coverage vs cost). PriceClass_100 is the cheapest."
  type        = string
  default     = "PriceClass_100"
}
