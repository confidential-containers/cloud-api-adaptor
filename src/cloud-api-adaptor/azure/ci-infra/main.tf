resource "azurerm_resource_group" "ci_rg" {
  name     = "${var.ci_rg}${var.ver}"
  location = var.location
}

# Create a container registry that will host the images.
resource "azurerm_container_registry" "acr" {
  name                   = "${var.container_registry}${var.ver}"
  resource_group_name    = azurerm_resource_group.ci_rg.name
  location               = azurerm_resource_group.ci_rg.location
  sku                    = "Standard"
  anonymous_pull_enabled = true

  admin_enabled = true
}

# Create a user assigned identity
resource "azurerm_user_assigned_identity" "gh_action_user_identity" {
  name                = "${var.gh_action_user_identity}${var.ver}"
  location            = azurerm_resource_group.ci_rg.location
  resource_group_name = azurerm_resource_group.ci_rg.name
}

resource "azurerm_federated_identity_credential" "gh_action_federated_credential" {
  name                = "${var.gh_action_federated_credential}${var.ver}"
  resource_group_name = azurerm_resource_group.ci_rg.name
  audience            = ["api://AzureADTokenExchange"]
  issuer              = "https://token.actions.githubusercontent.com"
  parent_id           = azurerm_user_assigned_identity.gh_action_user_identity.id

  subject = "repo:${var.gh_repo}:ref:refs/heads/main"
}

resource "azurerm_role_assignment" "ci_rg_role_binding" {
  scope                = azurerm_resource_group.ci_rg.id
  role_definition_name = "Contributor"
  principal_id         = azurerm_user_assigned_identity.gh_action_user_identity.principal_id
}

data "azurerm_subscription" "current" {
}

# CAA application needs permissions when creating the podvm to be able to join the worker node subnet.
resource "azurerm_role_definition" "caa_ci_provisioner" {
  name        = "Azure CAA CI Provisioner v${var.ver}"
  scope       = data.azurerm_subscription.current.id
  description = "Permissions needed for the GH actions to run test for CAA Azure provider"
  permissions {
    actions = [
      "Microsoft.Network/virtualNetworks/read",
      "Microsoft.Network/virtualNetworks/subnets/read",
      "Microsoft.Network/virtualNetworks/subnets/join/action"
    ]
  }
  assignable_scopes = [
    data.azurerm_subscription.current.id
  ]

}

resource "azurerm_role_assignment" "ci_custom_role_binding" {
  scope                = data.azurerm_subscription.current.id
  role_definition_name = azurerm_role_definition.caa_ci_provisioner.name
  principal_id         = azurerm_user_assigned_identity.gh_action_user_identity.principal_id
}

# This is needed in case of storing the podvm images.
resource "azurerm_shared_image_gallery" "podvm_image_gallery" {
  name                = "${var.image_gallery}${var.ver}"
  resource_group_name = azurerm_resource_group.ci_rg.name
  location            = azurerm_resource_group.ci_rg.location

  sharing {
    permission = "Community"
    community_gallery {
      prefix          = "cocopodvm"
      eula            = "https://raw.githubusercontent.com/confidential-containers/confidential-containers/main/LICENSE"
      publisher_uri   = "https://github.com/confidential-containers/confidential-containers"
      publisher_email = "cocoatmsft@outlook.com"
    }
  }
}

resource "azurerm_shared_image" "podvm_image" {
  name                = "${var.image_definition}${var.ver}"
  gallery_name        = resource.azurerm_shared_image_gallery.podvm_image_gallery.name
  resource_group_name = azurerm_resource_group.ci_rg.name
  location            = azurerm_resource_group.ci_rg.location
  os_type             = "Linux"
  identifier {
    publisher = "coco-caa"
    offer     = "coco-caa"
    sku       = "coco-caa"
  }
  hyper_v_generation                = "V2"
  confidential_vm_supported         = true
  disk_controller_type_nvme_enabled = true
}
