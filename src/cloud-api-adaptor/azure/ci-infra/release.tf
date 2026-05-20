# These resources manage the image gallery that holds the PodVM release images.

resource "azurerm_resource_group" "release_rg" {
  name     = var.release_rg
  location = var.location
}

resource "azurerm_shared_image_gallery" "release_podvm_image_gallery" {
  name                = var.release_image_gallery
  resource_group_name = azurerm_resource_group.release_rg.name
  location            = azurerm_resource_group.release_rg.location

  sharing {
    permission = "Community"
    community_gallery {
      prefix          = "cococommunity"
      eula            = "https://raw.githubusercontent.com/confidential-containers/confidential-containers/main/LICENSE"
      publisher_uri   = "https://github.com/confidential-containers/confidential-containers"
      publisher_email = "magnuskulke@microsoft.com"
    }
  }
}

resource "azurerm_shared_image" "release_podvm_image" {
  name                = var.release_image_definition
  gallery_name        = resource.azurerm_shared_image_gallery.release_podvm_image_gallery.name
  resource_group_name = azurerm_resource_group.release_rg.name
  location            = azurerm_resource_group.release_rg.location
  os_type             = "Linux"
  identifier {
    publisher = "coco-caa"
    offer     = "coco-caa"
    sku       = "coco-caa"
  }
  hyper_v_generation        = "V2"
  confidential_vm_supported = true
}

resource "azurerm_shared_image" "release_podvm_image_debug" {
  name                = var.release_image_definition_debug
  gallery_name        = resource.azurerm_shared_image_gallery.release_podvm_image_gallery.name
  resource_group_name = azurerm_resource_group.release_rg.name
  location            = azurerm_resource_group.release_rg.location
  os_type             = "Linux"
  identifier {
    publisher = "coco-caa"
    offer     = "coco-caa"
    sku       = "coco-caa-debug"
  }
  hyper_v_generation        = "V2"
  confidential_vm_supported = true
}

resource "azurerm_role_assignment" "release_gallery_publisher_role_binding" {
  scope                = azurerm_shared_image_gallery.release_podvm_image_gallery.id
  role_definition_name = "Compute Gallery Artifacts Publisher"
  principal_id         = azurerm_user_assigned_identity.gh_action_user_identity.principal_id
}

resource "random_uuid" "release_disk_role_id" {}

# uplosi requires those, apart from the gallery publisher role
resource "azurerm_role_definition" "release_disk_role" {
  name               = "read/write/delete disks"
  role_definition_id = random_uuid.release_disk_role_id.result

  scope       = data.azurerm_subscription.current.id
  description = "Allow read/write/delete on Microsoft.Compute/disks"

  permissions {
    actions = [
      "Microsoft.Compute/disks/read",
      "Microsoft.Compute/disks/write",
      "Microsoft.Compute/disks/delete",
      "Microsoft.Compute/disks/beginGetAccess/action",
      "Microsoft.Compute/disks/endGetAccess/action",
      "Microsoft.Compute/images/read",
      "Microsoft.Compute/images/write",
      "Microsoft.Compute/images/delete",
    ]
    not_actions = []
  }
}

resource "azurerm_role_assignment" "release_disk_role_binding" {
  scope              = azurerm_resource_group.release_rg.id
  role_definition_id = azurerm_role_definition.release_disk_role.role_definition_resource_id
  principal_id       = azurerm_user_assigned_identity.gh_action_user_identity.principal_id
}
