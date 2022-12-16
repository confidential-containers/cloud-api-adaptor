# Uncomment the awk statement if you want to use mac as 
# dhcp-identifier in Ubuntu
#awk '/^[[:space:]-]*dhcp4/{ split($0,arr,/dhcp4.*/)
#                           gsub(/-/," ", arr[1])
#                           rep=arr[1]
#                           print $0}
#                      rep{ printf "%s%s\n", rep, "dhcp-identifier: mac"
#                      rep=""
#                      next} 1' /etc/netplan/50-cloud-init.yaml | sudo tee /etc/netplan/50-cloud-init.yaml

# This ensures machine-id is generated during first boot and a unique
# dhcp IP is assigned to the VM
sudo echo -n > /etc/machine-id

#Lock password for the ssh user (peerpod) to disallow logins
sudo passwd -l peerpod
