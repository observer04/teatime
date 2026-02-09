#!/bin/bash

# Configuration
RESOURCE_GROUP="teatime"
VM_NAME="kettle-vm"

echo "================================================================"
echo "Stopping TeaTime Demo Environment"
echo "================================================================"

# 1. Deallocate the VM (Stops Compute Billing due to 'deallocated' state)
echo "Deallocating Virtual Machine '$VM_NAME'..."
echo "(This stops compute charges but keeps Disk and Static IP)"
az vm deallocate --resource-group "$RESOURCE_GROUP" --name "$VM_NAME"

echo "================================================================"
echo "🛑 Demo Environment Stopped"
echo "----------------------------------------------------------------"
echo "VM Status:    Deallocated (Compute billing paused)"
echo "Cost Note:    You are still paying for:"
echo "              - Storage (OS Disk)"
echo "              - Public IP Address (Standard SKU)"
echo "================================================================"
