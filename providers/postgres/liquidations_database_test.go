package postgres_test

import (
	"os"
	"testing"

	"bia-bills/providers/postgres"
)

// Conexión real a las bases externas del módulo.
//
// Se saltea si no hay credenciales en el entorno, así que en CI no corre — y no
// debe correr: las bases están en una red privada. Para ejecutarlo local hay que
// exportar liq_db_host, liq_db_port, liq_db_user y liq_db_password, y estar
// dentro de la red de BIA o con VPN.
//
// Las constantes de entities se inicializan al importar el paquete, o sea antes
// de que corra el test: las variables tienen que estar exportadas ANTES de
// arrancar el binario, no desde acá adentro.
func TestLiquidationsDBConecta(t *testing.T) {
	if os.Getenv("liq_db_host") == "" {
		t.Skip("sin credenciales de las bases de BIA (liq_db_host vacío)")
	}

	db := postgres.NewLiquidationsDB()

	bases := []postgres.LiquidationsDatabase{
		postgres.LiqDBFileCompiler,
		postgres.LiqDBCalculatorPrices,
	}

	for _, base := range bases {
		t.Run(string(base), func(t *testing.T) {
			var nombre string
			err := db.Connection(base).Raw("SELECT current_database()").Scan(&nombre).Error
			if err != nil {
				t.Fatalf("no conectó a %s: %v", base, err)
			}
			t.Logf("conectado a %s", nombre)

			// La tabla del módulo tiene que existir en la base que le corresponde.
			tabla := map[postgres.LiquidationsDatabase]string{
				postgres.LiqDBFileCompiler:     "liquidations_str_inputs",
				postgres.LiqDBCalculatorPrices: "liquidations_str_charges",
			}[base]

			var existe bool
			err = db.Connection(base).
				Raw(`SELECT EXISTS (
				       SELECT 1 FROM information_schema.tables
				        WHERE table_schema = 'public' AND table_name = ?)`, tabla).
				Scan(&existe).Error
			if err != nil {
				t.Fatalf("no se pudo consultar el catálogo de %s: %v", base, err)
			}
			if !existe {
				t.Fatalf("falta la tabla public.%s en %s", tabla, base)
			}
			t.Logf("public.%s presente", tabla)
		})
	}
}
