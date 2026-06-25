APP_NAME := collector
SVC_NAME := $(APP_NAME).service
CFG_NAME := $(APP_NAME).json 
BUILD_DIR := $(PWD)/bin
CFG_SRC_DIR := $(PWD)/configs
SVC_SRC_DIR := $(PWD)/service
CONFIG_DIR := /etc/$(APP_NAME)/configs
INSTALL_DIR := /etc/$(APP_NAME)
SYSTEMD_DIR := /etc/systemd/system

GREEN := \033[0;32m
RED := \033[0;31m
YELLOW := \033[0;33m
NC := \033[0m

.PHONY: all clean build

all: build check-service stop-service restart-service

build:
	@echo "$(GREEN)Building $(APP_NAME)...$(NC)"
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(APP_NAME) main.go
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(APP_NAME)$(NC)"

clean:
    @echo "$(YELLOW)Cleanning...$(NC)"
	rm -rf $(BUILD_DIR)
	@echo "$(GREEN)Clean complete$(NC)"

check-service:
	@if systemctl list-unit-files | grep -q "^$(SVC_NAME)"; then \
		echo "$(GREEN)Service $(SVC_NAME) found$(NC)"; \
	else \
		echo "$(YELLOW)Service $(SVC_NAME) not found$(NC)"; \
	fi

stop-service:
	@if systemctl is-active --quiet $(SVC_NAME); then \
		echo "$(YELLOW)Stopping service $(SVC_NAME)...$(NC)"; \
		sudo systemctl stop $(SVC_NAME); \
		echo "$(GREEN)Service stopped$(NC)"; \
	else \
		echo "$(YELLOW)Service $(SVC_NAME) is not running$(NC)"; \
	fi

restart-service:
	@echo "$(YELLOW)Restarting service $(SERVICE_NAME)...$(NC)"
	sudo systemctl daemon-reload
	sudo systemctl restart $(SVC_NAME)
	@echo "$(GREEN)Service restarted$(NC)"

install: build stop-service
	@echo "$(YELLOW)Installing $(APP_NAME)...$(NC)"
	
	# Создание необходимых директорий
	sudo mkdir -p $(INSTALL_DIR)
	sudo mkdir -p $(CONFIG_DIR)
	
	# Копирование бинарного файла
	sudo cp $(BUILD_DIR)/$(APP_NAME) $(INSTALL_DIR)
	sudo chmod 755 $(INSTALL_DIR)/$(APP_NAME)
	@echo "$(GREEN)Binary installed to $(INSTALL_DIR)/$(APP_NAME)$(NC)"
	
	# Копирование конфига (если существует)
	@if [ -f $(CFG_SRC_DIR)/$(CFG_NAME) ]; then \
		sudo cp $(CFG_SRC_DIR)/$(CFG_NAME) $(CONFIG_DIR)/; \
		sudo chmod 644 $(CONFIG_DIR)/$(CFG_NAME); \
		echo "$(GREEN)Config installed to $(CONFIG_DIR)/$(CFG_NAME)$(NC)"; \
	else \
		echo "$(YELLOW)Warning: $(CFG_NAME) not found, skipping...$(NC)"; \
	fi
	
	# Копирование systemd сервиса (если существует)
	@if [ -f $(SVC_SRC_DIR)\(SVC_NAME) ]; then \
		sudo cp $(SVC_SRC_DIR)\(SVC_NAME) $(SYSTEMD_DIR)/; \
		sudo chmod 644 $(SYSTEMD_DIR)/$(SVC_NAME); \
		echo "$(GREEN)Service file installed to $(SYSTEMD_DIR)/$(SVC_NAME)$(NC)"; \
	else \
		echo "$(YELLOW)Warning: $(SVC_NAME) not found, skipping...$(NC)"; \
	fi
	
	# Перезагрузка systemd и запуск сервиса
	sudo systemctl daemon-reload
	sudo systemctl enable $(SVC_NAME)
	sudo systemctl start $(SVC_NAME)
	
	@echo "$(GREEN)Installation complete!$(NC)"
	@echo "$(GREEN)Service $(SVC_NAME) is now running$(NC)"