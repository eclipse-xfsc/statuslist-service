GOA ?= goa
DESIGN_PKG := github.com/eclipse-xfsc/statuslist-service/design
COMPOSE ?= docker compose

.PHONY: \
	goa-gen goa-example goa-clean goa-regenerate \
	fmt test build run \
	compose-up compose-down compose-restart compose-logs compose-ps compose-clean \
	db-shell nats-shell \
	mock-create-bitstring mock-create-bitstring-list mock-create-statuslist2021 mock-create-statuslist2021-credential \
	mock-create-many-bitstring mock-create-many-statuslist2021

goa-gen:
	$(GOA) gen $(DESIGN_PKG)

goa-example:
	$(GOA) example $(DESIGN_PKG)

goa-clean:
	rm -rf gen cmd/statuslist

goa-regenerate: goa-clean goa-gen

fmt:
	gofmt -w main.go internal design

test:
	go test ./...

build:
	go build ./...

run:
	go run .

compose-up:
	$(COMPOSE) up --build

compose-down:
	$(COMPOSE) down

compose-restart:
	$(COMPOSE) down
	$(COMPOSE) up --build

compose-logs:
	$(COMPOSE) logs -f

compose-ps:
	$(COMPOSE) ps

compose-clean:
	$(COMPOSE) down -v --remove-orphans

db-shell:
	$(COMPOSE) exec postgres psql -U statuslist -d statuslist

nats-shell:
	$(COMPOSE) exec nats sh

mock-create-bitstring:
	TYPE=BitstringStatusListCredential ./mock/client/create-status.sh

mock-create-bitstring-list:
	TYPE=BitstringStatusList ./mock/client/create-status.sh

mock-create-statuslist2021:
	TYPE=StatusList2021 ./mock/client/create-status.sh

mock-create-statuslist2021-credential:
	TYPE=StatusList2021Credential ./mock/client/create-status.sh

mock-create-many-bitstring:
	TYPE=BitstringStatusListCredential COUNT=10 ./mock/client/create-status.sh

mock-create-many-statuslist2021:
	TYPE=StatusList2021 COUNT=10 ./mock/client/create-status.sh

SERVICE_URL ?= http://localhost:8080
TENANT_ID ?= tenant-a
LIST_ID ?= 1

get-status-json:
	curl -s \
		-H "X-Tenant-Id: $(TENANT_ID)" \
		-H "Accept: application/json" \
		"$(SERVICE_URL)/status/$(LIST_ID)" | jq

get-status-vcld:
	curl -s \
		-H "X-Tenant-Id: $(TENANT_ID)" \
		-H "Accept: application/vc+ld+json" \
		"$(SERVICE_URL)/status/$(LIST_ID)" | jq

get-status-jwt:
	curl -s \
		-H "X-Tenant-Id: $(TENANT_ID)" \
		-H "Accept: application/statuslist+jwt" \
		"$(SERVICE_URL)/status/$(LIST_ID)"