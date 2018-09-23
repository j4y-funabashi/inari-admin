variable "apigw_id" {}
variable "apigw_root_id" {}
variable "execution_arn" {}
variable "app_name" {}
variable "app_version" {}
variable "app_path" {}
variable "app_method" {}
variable "deploy_bucket" {}
variable "deploy_key" {}

resource "aws_iam_role" "lambda" {
  name = "${var.app_name}-role"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "sts:AssumeRole"
        ],
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "app" {
  name = "${var.app_name}-policy"
  role = "${aws_iam_role.lambda.id}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "s3:PutObject",
        "s3:PutObjectAcl",
        "s3:GetObject"
      ],
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_lambda_permission" "apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.main.arn}"
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${var.execution_arn}/*/*"
}

resource "aws_lambda_permission" "apigw-alias" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.main.arn}"
  qualifier     = "${aws_lambda_alias.main.name}"
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${var.execution_arn}/*/*"
}

resource "aws_lambda_function" "main" {
  function_name = "lambda-${var.app_name}"
  handler       = "function"
  runtime       = "go1.x"
  role          = "${aws_iam_role.lambda.arn}"
  s3_bucket     = "${var.deploy_bucket}"
  s3_key        = "${var.deploy_key}/${var.app_version}/function.zip"
  publish       = true
}

resource "aws_lambda_alias" "main" {
  name             = "lambda-alias-${var.app_name}"
  function_name    = "${aws_lambda_function.main.arn}"
  function_version = "$LATEST"
}

resource "aws_api_gateway_resource" "main" {
  rest_api_id = "${var.apigw_id}"
  parent_id   = "${var.apigw_root_id}"
  path_part   = "${var.app_path}"
}

resource "aws_api_gateway_method" "proxy" {
  rest_api_id   = "${var.apigw_id}"
  resource_id   = "${aws_api_gateway_resource.main.id}"
  http_method   = "${var.app_method}"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "lambda" {
  rest_api_id = "${var.apigw_id}"
  resource_id = "${aws_api_gateway_method.proxy.resource_id}"
  http_method = "${aws_api_gateway_method.proxy.http_method}"

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = "${replace(aws_lambda_function.main.invoke_arn, aws_lambda_function.main.function_name, "${aws_lambda_function.main.function_name}:${aws_lambda_alias.main.name}")}"
}
