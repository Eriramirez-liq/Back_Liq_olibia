package router_test

import (
	"testing"

	"bia-bills/entities"
	"bia-bills/router"

	"github.com/gin-gonic/gin"
)

// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Este test fija el CONTRATO con el front. Los paths de acá son los que están en
// el endpoints.ts de olibia-web; cambiar uno rompe la pantalla de Cargos STR sin
// que ningún test de backend se queje. Al trasvasar a bia-bills, el prefijo del
// servicio se suma adelante y quedan como /ms-bill/liquidations/...
//
// No necesita base de datos: NewLiquidationsDB abre con DisableAutomaticPing, así
// que registrar las rutas no conecta a ningún lado.
func TestRegisterLiquidations_RutasDelContrato(t *testing.T) {
	// Credenciales de mentira para que la cadena de conexión sea parseable. No se
	// usa ninguna conexión: solo se registran handlers.
	for variable, valor := range map[*string]string{
		&entities.LiqDbHost:    "localhost",
		&entities.LiqDbPort:    "5432",
		&entities.LiqDbUser:    "test",
		&entities.LiqDbPass:    "test",
		&entities.LiqDbSSLMode: "disable",
	} {
		original := *variable
		*variable = valor
		defer func(p *string, v string) { *p = v }(variable, original)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router.RegisterLiquidations(engine.Group("/"))

	registradas := map[string]bool{}
	for _, ruta := range engine.Routes() {
		registradas[ruta.Method+" "+ruta.Path] = true
	}

	esperadas := []string{
		"GET /liquidations/health",
		"POST /liquidations/cargos-str/preview",
		"POST /liquidations/cargos-str/confirm",
		"GET /liquidations/cargos-str",
		"GET /liquidations/cargos-str/periods",
		"GET /liquidations/cargos-str/loads",

		"POST /liquidations/tarifas-sdl/preview",
		"POST /liquidations/tarifas-sdl/confirm",
		"GET /liquidations/tarifas-sdl",
		"GET /liquidations/tarifas-sdl/periods",
		"GET /liquidations/tarifas-sdl/loads",
		"GET /liquidations/tarifas-sdl/audit",

		// TC1 no tiene preview: el archivo se parsea en el navegador y llegan las
		// filas ya normalizadas al confirm.
		"POST /liquidations/tc1/confirm",
		"GET /liquidations/tc1",
		"GET /liquidations/tc1/periods",
		"GET /liquidations/tc1/loads",
		"GET /liquidations/tc1/status",
		"GET /liquidations/tc1/operators",

		"POST /liquidations/sdl-operador/confirm",
		"GET /liquidations/sdl-operador",
		"GET /liquidations/sdl-operador/periods",
		"GET /liquidations/sdl-operador/loads",
		"GET /liquidations/sdl-operador/status",
		"GET /liquidations/sdl-operador/operators",

		"GET /liquidations/proyeccion/prices",
	}

	for _, ruta := range esperadas {
		if !registradas[ruta] {
			t.Errorf("falta la ruta %q; registradas: %v", ruta, claves(registradas))
		}
	}

	// Ni una ruta de más: la fase 2 (NetSuite) no se despliega, y una ruta suelta
	// que quedara del port se expondría en producción sin que nadie la revise.
	if len(registradas) != len(esperadas) {
		t.Errorf("hay %d rutas registradas y se esperaban %d: %v",
			len(registradas), len(esperadas), claves(registradas))
	}
}

func claves(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
