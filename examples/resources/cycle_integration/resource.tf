# An AWS integration. Auth and extra are write-sensitive: later reads
# preserve state when the Cycle API omits secrets.
resource "cycle_integration" "aws" {
  name   = "AWS Production"
  vendor = "aws"

  # Optional; auto-generated from the name if omitted.
  identifier = "aws-production"

  auth = {
    api_key = "example-access-key"
    secret  = "example-secret-key"
    region  = "us-east-1"
  }

  extra = {
    account_id = "123456789012"
  }
}
