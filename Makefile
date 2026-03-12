# Auto-detect host IP for SIP NAT — prefer Tailscale, fall back to local interface
# Override: EXTERNAL_IP=1.2.3.4 make up
export EXTERNAL_IP ?= $(shell \
	ts=$$(tailscale ip -4 2>/dev/null); \
	if [ -n "$$ts" ]; then echo "$$ts"; \
	else ipconfig getifaddr en0 2>/dev/null || hostname -I 2>/dev/null | awk '{print $$1}'; \
	fi)
export EXTERNAL_IP

.PHONY: up down build logs restart asterisk-cli sip-status

up: ## Start xpbx (Asterisk + web UI)
	docker compose up -d --build

down: ## Stop everything
	docker compose down

build: ## Rebuild containers
	docker compose build

logs: ## Follow logs
	docker compose logs -f

restart: ## Restart all services
	docker compose restart

asterisk-cli: ## Open Asterisk console
	docker exec -it xpbx-asterisk asterisk -rvvv

sip-status: ## Show SIP endpoint registrations
	docker exec -it xpbx-asterisk asterisk -rx "pjsip show endpoints"
