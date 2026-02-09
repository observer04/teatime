#!/bin/bash

# Configuration
RESOURCE_GROUP="teatime"
VM_NAME="kettle-vm"

echo "================================================================"
echo "Starting TeaTime Demo Environment"
echo "================================================================"

# 1. Start the VM
echo "Starting Virtual Machine '$VM_NAME' in group '$RESOURCE_GROUP'..."
az vm start --resource-group "$RESOURCE_GROUP" --name "$VM_NAME"

# 2. Get Public IP
echo "Retrieving Public IP..."
IP_ADDRESS=$(az vm show --resource-group "$RESOURCE_GROUP" --name "$VM_NAME" --show-details --query publicIps --output tsv)

echo "================================================================"
echo "✅ Demo Environment Started!"
echo "----------------------------------------------------------------"
echo "VM Status:    Running (Billing Active)"
echo "Public IP:    $IP_ADDRESS"
echo "Application:  https://$IP_ADDRESS (or configured domain)"
echo "SSH Access:   ssh azureuser@$IP_ADDRESS"
echo "================================================================"
echo "Make sure to run ./scripts/turn-off-demo.sh when finished!"
