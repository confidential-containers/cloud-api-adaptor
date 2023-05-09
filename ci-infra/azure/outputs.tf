output "AZURE_SUBSCRIPTION_ID" {
  value = data.azurerm_subscription.current.subscription_id
}

output "AZURE_TENANT_ID" {
  value = data.azurerm_subscription.current.tenant_id
}

output "AZURE_CLIENT_ID" {
  value = azurerm_user_assigned_identity.gh_action_user_identity.client_id
}

output "AZURE_RESOURCE_GROUP" {
  value = azurerm_resource_group.ci_rg.name
}

output "AZURE_REGION" {
  value = var.location
}

output "ACR_URL" {
  value = azurerm_container_registry.acr.login_server
}
