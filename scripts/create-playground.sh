#!/bin/bash
# Creates a playground/ directory with a starter main.tf for manual provider testing.
# Prompts before overwriting if the directory already exists.

PLAYGROUND_DIR=$PWD/playground

if [ -d "$PLAYGROUND_DIR" ]; then
  echo ""
  echo "A playground directory already exists at $PLAYGROUND_DIR."
  echo "Would you like to overwrite it? (y/n)"
  read -r OVERWRITE_CHOICE
  echo ""

  if [[ ! "$OVERWRITE_CHOICE" =~ ^[Yy]$ ]]; then
    echo "Exiting."
    echo ""
    exit 0
  fi
fi

if [ -d "$PLAYGROUND_DIR" ]; then
  rm -rf "$PLAYGROUND_DIR"
fi
mkdir -p "$PLAYGROUND_DIR"
cat << EOF > $PLAYGROUND_DIR/main.tf
terraform {
  required_providers {
    # Matches the dev_overrides entry written by 'make create-dev-overrides'.
    maas = {
      source = "canonical/maas-apiv3"
    }
  }
}

provider "maas" {
  # api_url  = "http://<maas-ip>:5240" # Or export TF_MAAS_URL
  # username = "admin"                  # Or export TF_MAAS_USER
  # password = "..."                    # Or export TF_MAAS_PWD
}

# Try me out!
# resource "maas_zone" "example" {
#   name        = "example-zone"
#   description = "Created from the playground"
# }

EOF

echo ""
echo "Playground created at $PLAYGROUND_DIR."
echo "Fill in the provider block (or export TF_MAAS_* env vars), then run 'terraform apply'."
echo ""
echo "Happy Terraforming! 🚀"
echo ""
