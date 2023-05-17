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

output "AZURE_PODVM_GALLERY_NAME" {
  value = azurerm_shared_image_gallery.podvm_image_gallery.name
}

output "AZURE_PODVM_IMAGE_DEF_NAME" {
  value = azurerm_shared_image.podvm_image.name
}
