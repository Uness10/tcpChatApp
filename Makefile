# Application Name
APP_NAME=tcp-chat-app

# Go commands
GO=go

# Database Configuration
DB_CONTAINER_NAME=chat_db
DB_USER=root
DB_PASSWORD=root
DB_NAME=chat_db
DB_PORT=5432

# Default target
.DEFAULT_GOAL := help

# -------------------------------------------
# HELP: List available commands
# -------------------------------------------
help:
	@echo "Usage: make [target]"
	@echo "Available targets:"
	@echo "  build     - Compile the chat server"
	@echo "  run       - Start the chat server"
	@echo "  db-up     - Start PostgreSQL with Docker"
	@echo "  db-down   - Stop and remove PostgreSQL container"

# -------------------------------------------
# BUILD: Compile the Go binary
# -------------------------------------------
build:
	@echo "🔨 Building the chat server..."
	@$(GO) build -o $(APP_NAME) ./cmd/server
	@echo "✅ Build complete!"

# -------------------------------------------
# RUN: Start the chat server
# -------------------------------------------
run: build
	@echo "🚀 Running the chat server..."
	@./$(APP_NAME)

# -------------------------------------------
# DATABASE: Start PostgreSQL
# -------------------------------------------
db-up:
	@echo "🚀 Starting PostgreSQL..."
	@docker run --name $(DB_CONTAINER_NAME) -e POSTGRES_USER=$(DB_USER) -e POSTGRES_PASSWORD=$(DB_PASSWORD) -e POSTGRES_DB=$(DB_NAME) -p $(DB_PORT):5432 -d postgres
	@echo "✅ Database running on port $(DB_PORT)"

# -------------------------------------------
# DATABASE: Stop PostgreSQL
# -------------------------------------------
db-down:
	@echo "🛑 Stopping PostgreSQL..."
	@docker stop $(DB_CONTAINER_NAME) && docker rm $(DB_CONTAINER_NAME)
	@echo "✅ Database stopped and removed!"
