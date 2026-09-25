.PHONY: build install dev clean run test

BIN := aip
INSTALL_DIR ?= $(HOME)/.local/bin

build:
	@echo "==> Building frontend assets..."
	@if [ ! -d "web/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		npm --prefix web install; \
	fi
	npm --prefix web run build
	@echo "==> Compiling standalone Go binary with embedded assets..."
	go build -o $(BIN) ./cmd/aip
	@echo "==> Build complete: ./$(BIN)"

install: build
	@echo "==> Installing binary to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	rm -f $(INSTALL_DIR)/$(BIN)
	cp $(BIN) $(INSTALL_DIR)/$(BIN)
	@chmod +x $(INSTALL_DIR)/$(BIN)
	@echo "==> Installed successfully to $(INSTALL_DIR)/$(BIN)"

dev:
	@echo "==> Starting AIP development environment..."
	@trap 'kill 0' EXIT; \
	go run ./cmd/aip __run_daemon & \
	npm --prefix web run dev & \
	wait

clean:
	@echo "==> Cleaning build artifacts..."
	rm -f $(BIN)
	rm -rf web/dist
	@echo "==> Clean complete."

test:
	@echo "==> Running Go tests..."
	go test -v ./...
