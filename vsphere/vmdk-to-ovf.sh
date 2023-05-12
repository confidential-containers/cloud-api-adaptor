#!/usr/bin/env bash

RED='\033[0;31m'
NOCOLOR='\033[0m'

[ "$DEBUG" == 'true' ] && set -x

ARGC=$#
if [ $ARGC -lt 7 ] || [ "$7" = "-disableefi" ]; then
    echo "USAGE: $(basename $0) <vCenter Server> <vCenter Username> <vCenter Password> <vCenter Cluster> <vCenter Datastore> <Template Name> <Path to VMDK> <-disableefi>(Optional)"
    exit 1
fi

# VCENTER Credentials
VCENTER_SERVER=$1
VCENTER_USERNAME=$2
VCENTER_PASSWORD=$3
VCENTER_CLUSTER=$4
VCENTER_DATASTORE=$5

# Change this to 0 to enable ssl verification
IGNORE_SSL=1

# Change this to 0 to not overwrite an existing template
FORCE_INSTALL=1

# The name of the uploaded template
TEMPLATE_NAME=$6

# Path to the vmdk to convert to a template
VMDK_PATH=$7

# Legacy bios or EFI
LEGACY_BIOS=${8:--notset}
LEGACYBIOS=0
[ "$LEGACY_BIOS" = "-disableefi" ] && LEGACYBIOS=1


TMPDIR=$(mktemp -d)
IMAGE_NAME="$(basename -- ${VMDK_PATH})"
VMDK_NAME="${IMAGE_NAME%.*}"
FORMAT="${IMAGE_NAME#*.}"

warnmsg  () {
    printf "${RED} $1"
    printf "${NOCOLOR}"
    echo;
}

pre_checks() {
        [[ -x "$(command -v pwsh)" ]] || { warnmsg "Powershell is not installed, Please see: https://learn.microsoft.com/en-us/powershell/scripting/install/installing-powershell-on-linux?view=powershell-7.3" exit 1; }
	(pwsh -Command Get-InstalledModule -Name "VMware.PowerCLI" | grep "No match" 2>&1 >/dev/null) && warnmsg "VMware.PowerCLI module is not installed for Powershell, Please see: https://docs.vmware.com/en/VMware-vSphere/7.0/com.vmware.esxi.install.doc/GUID-F02D0C2D-B226-4908-9E5C-2E783D41FE2D.html" && exit 1;
	[[ "$FORMAT" == "vmdk"  ]] || { warnmsg "image must be \"vmdk\", convert with: \"qemu-img convert -O vmdk -o subformat=streamOptimized ${IMAGE_NAME} ${VMDK_NAME}.vmdk\""; exit 1; }
}

# Copy the vmdk to a folder on datastore root called "podvm"
# Create a new VM using the vmdk as an attached disk
# Convert the VM into a template
create_ps1script() {
    cat > ${TMPDIR}/config.ps1 << EOF
param(\$Server, \$User, \$Password, \$Cluster, \$Datastore, \$VMDKPath, \$TemplateName, \$IgnoreSSL, \$Force, \$LegacyBIOS)

Write-Host "Connecting to \$Server"
if (\$IgnoreSSL -eq 1) {
   Set-PowerCLIConfiguration -InvalidCertificateAction Ignore -Confirm:\$False
}

Connect-VIServer -Server \$Server -User \$User -Password \$Password
\$datastore = Get-Datastore \$Datastore

New-PSDrive -Location \$datastore -Name ds -PSProvider VimDatastore -Root "\"
if (\$Force -eq 1) {
   Write-Host "Attempting to remove \$TemplateName"
   Remove-Template -Template \$TemplateName -Confirm:\$False
   Remove-Item -Path ds:\\\$TemplateName -Recurse -Force
}

if (\$Force -eq 1) {
   New-Item -Path ds:\podvm -Force -ItemType Directory
} else {
     New-Item -Path ds:\podvm -ItemType Directory
     if (-not(\$?)) {
         exit 1
    }
}

Write-Host "Copying vmdk \$VMDKPath to \$Server/\$Datastore"
Copy-DatastoreItem -Item \$VMDKPath -Destination ds:\podvm\podvm-base.vmdk

Write-Host "Creating new VM..."
New-VM -Name \$TemplateName -ResourcePool \$Cluster  -NumCPU 2 -MemoryGB 4 -NetworkName "VM Network" -Datastore \$datastore -GuestID rhel8_64Guest -DiskPath "[\$datastore] podvm/podvm-base.vmdk"

if (\$LegacyBIOS -eq 1) {
   Write-Host "Using legacy BIOS for creating template \$TemplateName"
   \$VM = Get-VM \$TemplateName
   \$spec = New-Object VMware.Vim.VirtualMachineConfigSpec
   \$spec.Firmware = [VMware.Vim.GuestOsDescriptorFirmwareType]::bios
   \$boot = New-Object VMware.Vim.VirtualMachineBootOptions
   \$boot.EfiSecureBootEnabled = \$false
   \$spec.BootOptions = \$boot
   \$VM.ExtensionData.ReconfigVM(\$spec)
}


Write-Host "Converting to template"
Get-VM -Name \$TemplateName | Set-VM -ToTemplate -Confirm:\$false
EOF
}

create_template() {
    pwsh ${TMPDIR}/config.ps1 -Server $VCENTER_SERVER -User $VCENTER_USERNAME -Password $VCENTER_PASSWORD -Cluster $VCENTER_CLUSTER -VMDKPath $VMDK_PATH -TemplateName $TEMPLATE_NAME -IgnoreSSL $IGNORE_SSL -Datastore $VCENTER_DATASTORE  -Force $FORCE_INSTALL -LegacyBIOS $LEGACYBIOS
}

clean_up() {
    rm -rf ${TMPDIR}
}

pre_checks
create_ps1script
create_template
clean_up
