provider "aws" {
  profile = "j4y"
  region  = "${var.region}"
}

terraform {
  backend "s3" {
    bucket  = "tf-inari-admin"
    key     = "terraform.tfstate"
    region  = "eu-west-1"
    profile = "j4y"
  }
}

variable "app_name" {}
variable "app_env" {}
variable "app_domain" {}
variable "certificate_arn" {}
variable "region" {}
variable "app_version" {}

resource "aws_api_gateway_rest_api" "main" {
  name = "${var.app_name}"
}

resource "aws_api_gateway_deployment" "main" {
  rest_api_id = "${aws_api_gateway_rest_api.main.id}"
  stage_name  = "${var.app_env}"
}

resource "aws_api_gateway_domain_name" "main" {
  domain_name     = "${var.app_domain}"
  certificate_arn = "${var.certificate_arn}"
}

resource "aws_api_gateway_base_path_mapping" "main" {
  api_id      = "${aws_api_gateway_rest_api.main.id}"
  stage_name  = "${aws_api_gateway_deployment.main.stage_name}"
  domain_name = "${aws_api_gateway_domain_name.main.domain_name}"
}

output "base_url" {
  value = "${aws_api_gateway_deployment.main.invoke_url}"
}

resource "aws_s3_bucket" "admin-data" {
  bucket = "admin.funabashi.co.uk"
  acl    = "private"
}



module "login_lambda" {
  source        = "./lambda-service"
  app_name      = "${var.app_name}-login"
  app_version   = "${var.app_version}"
  app_path      = "login"
  app_method    = "GET"
  deploy_bucket = "dep-inari-admin"
  deploy_key    = "login"
  apigw_id      = "${aws_api_gateway_rest_api.main.id}"
  apigw_root_id = "${aws_api_gateway_rest_api.main.root_resource_id}"
  execution_arn = "${aws_api_gateway_deployment.main.execution_arn}"
}

module "login_init_lambda" {
  source        = "./lambda-service"
  app_name      = "${var.app_name}-login-init"
  app_version   = "${var.app_version}"
  app_path      = "login-init"
  app_method    = "POST"
  deploy_bucket = "dep-inari-admin"
  deploy_key    = "login-init"
  apigw_id      = "${aws_api_gateway_rest_api.main.id}"
  apigw_root_id = "${aws_api_gateway_rest_api.main.root_resource_id}"
  execution_arn = "${aws_api_gateway_deployment.main.execution_arn}"
}

module "login_callback_lambda" {
  source        = "./lambda-service"
  app_name      = "${var.app_name}-login-callback"
  app_version   = "${var.app_version}"
  app_path      = "login-callback"
  app_method    = "GET"
  deploy_bucket = "dep-inari-admin"
  deploy_key    = "login-callback"
  apigw_id      = "${aws_api_gateway_rest_api.main.id}"
  apigw_root_id = "${aws_api_gateway_rest_api.main.root_resource_id}"
  execution_arn = "${aws_api_gateway_deployment.main.execution_arn}"
}
