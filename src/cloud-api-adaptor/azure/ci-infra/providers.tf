terraform {
  required_version = ">=0.12"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "=4.45.0"
    }
  }

  backend "azurerm" {
    resource_group_name  = "caa-azure-state"
    storage_account_name = "caaterraformstate"
    container_name       = "terraform-state"
    key                  = "ci.terraform.tfstate"
  }
}

provider "azurerm" {
  features {}
}
