# Targets del port a Go. Mismos nombres y herramientas que bia-bills, para que
# lo que pase acá pase igual allá cuando se trasvase el código.
#
# Ojo: los targets de docker-compose de bia-bills (start, ci-start, mockserver)
# no están porque este repo no tiene esos compose. Se corren allá.

# Paquetes propios, excluyendo node_modules: la dependencia npm `flatted` trae una
# implementación en Go y `./...` la levanta. Acá es solo ruido —aparece en la
# cobertura y alarga el build— pero si algún día no compilara, rompería los
# comandos. En bia-bills no existe el problema porque no hay node_modules.
PKGS := $(shell go list ./... | grep -v node_modules)

install:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/vektra/mockery/v2@v2.43.2
	go install github.com/swaggo/swag/cmd/swag@latest

lint:
	staticcheck -checks all $(PKGS)
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v

test:
	go test $(PKGS)

coverage:
	go test $(PKGS) -coverprofile=cover.out
	go tool cover -html=cover.out
	go test $(PKGS) -coverprofile cover.out -covermode count
	go tool cover -func cover.out

mock:
	mockery --all --keeptree --disable-version-string

swagger:
	swag init --parseDependency

# El TypeScript que todavía no se portó. Se va borrando por módulo a medida que
# su versión en Go entra en producción.
test-ts:
	npm run test

typecheck-ts:
	npx tsc --noEmit -p tsconfig.json
