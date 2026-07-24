# Storage account + container used to host VHD page blobs that back podvm
# gallery image versions uploaded by the e2e tooling.
resource "azurerm_storage_account" "podvm_storage" {
  name                     = "${var.podvm_storage_account}${var.ver}"
  resource_group_name      = azurerm_resource_group.ci_rg.name
  location                 = azurerm_resource_group.ci_rg.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  account_kind             = "StorageV2"
}

resource "azurerm_storage_container" "podvm_vhd" {
  name                  = var.podvm_storage_container
  storage_account_id    = azurerm_storage_account.podvm_storage.id
  container_access_type = "private"
}

# The existing `Contributor` role binding on ci_rg covers management-plane
# operations (storage account & container CRUD) but does NOT grant blob data
# plane access. AAD-based blob uploads (via DefaultAzureCredential in the
# Go upload tooling) require `Storage Blob Data Contributor`.
#
# Note: the federated identity credentials themselves do not need to be
# updated -- they only control which GitHub workflow subjects can assume the
# user-assigned identity; write permissions to the container come from the
# role assignment below.
resource "azurerm_role_assignment" "podvm_storage_blob_data_contributor" {
  scope                = azurerm_storage_account.podvm_storage.id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azurerm_user_assigned_identity.gh_action_user_identity.principal_id
}
