#!/bin/bash
# Stop the Azure VM to save credits
az vm deallocate -g "${AZURE_RESOURCE_GROUP:-teatime}" -n "${AZURE_VM_NAME:-kettle-vm}"
