PROVIDER_NAMESPACE ?= hashicorp
PROVIDER_NAME      ?= scaffolding

default: fmt lint install generate

build:
	go build -v ./...

install:
	go build -v -o "$$(go env GOPATH)/bin/terraform-provider-$(PROVIDER_NAME)" .

DEV_TFRC ?= $(CURDIR)/dev.tfrc

create-dev-overrides:
	@printf 'provider_installation {\n  dev_overrides {\n    "$(PROVIDER_NAMESPACE)/$(PROVIDER_NAME)" = "%s"\n  }\n  direct {}\n}\n' "$$(go env GOPATH)/bin" > $(DEV_TFRC)
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
	@echo "    make install            # build and install provider binary to \$$GOPATH/bin"
	@echo "    export TF_CLI_CONFIG_FILE=$(DEV_TFRC)"
	@echo "    terraform apply         # skips version/checksum checks; no init needed"
	@echo ""
	@echo "  To deactivate:"
	@echo "    bash/zsh : unset TF_CLI_CONFIG_FILE"
	@echo "    fish     : set -e TF_CLI_CONFIG_FILE"
	@echo "  Or run 'terraform init -upgrade' once a new release is published."
	@echo ""

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

generate-client: api/openapi.converted.json
	oapi-codegen -config internal/client/maasclientv3/oapi-codegen.yaml api/openapi.converted.json

api/openapi.converted.json: api/openapi.json
	python3 scripts/fix-openapi-nullable.py api/openapi.json api/openapi.converted.json

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: fmt lint test testacc build install generate generate-client create-dev-overrides
