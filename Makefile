BINARY_NAME := capsule
BUILD_DIR := .
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: all build install uninstall clean docker docker-rodin help

all: build

## Build the binary
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/capsule

## Install binary to ~/.local/bin (creates hard link)
install: build
	@mkdir -p $(INSTALL_DIR)
	@ln -f $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo "Make sure $(INSTALL_DIR) is in your PATH"

## Remove installed binary
uninstall:
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Removed $(BINARY_NAME) from $(INSTALL_DIR)"

## Build Docker image
docker: build
	./$(BINARY_NAME) build-image --force

## Build the rodin Docker image (base + agent-relay + vessel-writer)
docker-rodin: build
	./$(BINARY_NAME) build-image --force --target rodin

## Run tests
test:
	go test ./...

## Clean build artifacts
clean:
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Cleaned build artifacts"

## Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build      Build the binary"
	@echo "  install    Build and install to ~/.local/bin"
	@echo "  uninstall  Remove from ~/.local/bin"
	@echo "  docker     Sync Dockerfile and rebuild Docker image"
	@echo "  docker-rodin  Rebuild the rodin image (relay + writer baked in)"
	@echo "  test       Run tests"
	@echo "  clean      Remove build artifacts"
	@echo "  help       Show this help"
