PROVIDER_HOSTNAME  ?= registry.terraform.io
PROVIDER_NAMESPACE ?= canonical
PROVIDER_NAME      ?= maas-apiv3
PROVIDER_VERSION   ?= 0.0.1

OS   ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

# Built artifact (Terraform expects a name starting with terraform-provider-<type>)
BIN    ?= $(CURDIR)/bin
BINARY ?= terraform-provider-$(PROVIDER_NAME)

default: help

help: ## Show all available make targets
	@echo 'All Makefile commands:'
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Compile the provider binary to $(BIN)/$(BINARY)
	@mkdir -p $(BIN)
	go build -v -o $(BIN)/$(BINARY) .

PLUGIN_DIR ?= ~/.terraform.d/plugins/$(PROVIDER_HOSTNAME)/$(PROVIDER_NAMESPACE)/$(PROVIDER_NAME)/$(PROVIDER_VERSION)/$(OS)_$(ARCH)

install: build ## Install provider into the local filesystem mirror (~/.terraform.d/plugins/...)
	@mkdir -p $(PLUGIN_DIR)
	mv $(BIN)/$(BINARY) $(PLUGIN_DIR)/$(BINARY)

DEV_TFRC ?= $(CURDIR)/dev.tfrc

create-dev-overrides: install ## Install provider + write dev.tfrc for local testing (then export TF_CLI_CONFIG_FILE)
	@printf 'provider_installation {\n  dev_overrides {\n    "$(PROVIDER_NAMESPACE)/$(PROVIDER_NAME)" = "%s"\n  }\n  direct {}\n}\n' "$(BIN)" > $(DEV_TFRC)
	@echo ""
	@echo "  dev_overrides written to: $(DEV_TFRC)"
	@echo ""
	@echo "  Activate for this shell session:"
	@echo "    bash/zsh : export TF_CLI_CONFIG_FILE=$(DEV_TFRC)"
	@echo "    fish     : set -x TF_CLI_CONFIG_FILE $(DEV_TFRC)"
	@echo ""
	@echo "  NOTE: This export is TEMPORARY — it applies only to the current shell"
	@echo "  session. To persist it, add the relevant line above to your shell"
	@echo "  profile (~/.bashrc, ~/.zshrc, ~/.config/fish/config.fish, etc.)."
	@echo ""
	@echo "  Typical workflow:"
	@echo "    make create-dev-overrides   # builds $(BIN)/$(BINARY), installs to \$$GOPATH/bin, writes $(DEV_TFRC)"
	@echo "    export TF_CLI_CONFIG_FILE=$(DEV_TFRC)"
	@echo "    terraform apply         # skips version/checksum checks; no init needed"
	@echo ""
	@echo "  To deactivate:"
	@echo "    bash/zsh : unset TF_CLI_CONFIG_FILE"
	@echo "    fish     : set -e TF_CLI_CONFIG_FILE"
	@echo "  Or run 'terraform init -upgrade' once a new release is published."
	@echo ""

lint: ## Run golangci-lint
	golangci-lint run

generate: ## Run go generate (docs, etc.)
	cd tools; go generate ./...

generate-client: api/generated/openapi.converted.json ## Regenerate the MAAS API client from api/generated/openapi.json
	oapi-codegen -config internal/client/maasclientv3/oapi-codegen.yaml api/generated/openapi.converted.json

api/generated/openapi.converted.json: api/generated/openapi.json
	python3 scripts/fix-openapi-nullable.py api/generated/openapi.json api/generated/openapi.converted.json

fmt: ## Run gofmt across the codebase
	gofmt -s -w -e .

test: ## Run unit tests
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc: ## Run acceptance tests (requires live MAAS and TF_ACC=1)
	TF_ACC=1 go test -v -cover -timeout 120m ./...

scaffold-resource: ## Scaffold a new resource: make scaffold-resource NAME=<name>
	@test -n "$(NAME)" || (echo "Usage: make scaffold-resource NAME=<name>"; exit 1)
	tfplugingen-framework scaffold resource    -force -name $(NAME) -output-dir internal/provider/ -package provider

scaffold-datasource: ## Scaffold a new data source: make scaffold-datasource NAME=<name>
	@test -n "$(NAME)" || (echo "Usage: make scaffold-datasource NAME=<name>"; exit 1)
	tfplugingen-framework scaffold data-source -force -name $(NAME) -output-dir internal/provider/ -package provider

scaffold-function: ## Scaffold a new function: make scaffold-function NAME=<name>
	@test -n "$(NAME)" || (echo "Usage: make scaffold-function NAME=<name>"; exit 1)
	tfplugingen-framework scaffold function    -force -name $(NAME) -output-dir internal/provider/ -package provider

generate-resources: ## Regenerate provider schemas from api/generated/openapi.json + api/generator_config.yaml
	python3 scripts/fix-openapi-nullable.py api/generated/openapi.json api/generated/openapi.converted.json
	tfplugingen-openapi generate \
		--config api/generator_config.yaml \
		--output api/generated/provider-code-spec.json \
		api/generated/openapi.converted.json
	tfplugingen-framework generate resources \
		--input api/generated/provider-code-spec.json \
		--output internal/provider

.PHONY: help fmt lint test testacc build install generate generate-client generate-resources create-dev-overrides scaffold-resource scaffold-datasource scaffold-function
