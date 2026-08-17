package postgres_test

import (
	"testing"

	"bia-bills/entities"
	"bia-bills/providers/postgres"

	"github.com/DATA-DOG/go-sqlmock"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Tests del provider que corren SIN base de datos.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// liquidations_database_test.go conecta de verdad y se saltea en CI. Acá va lo que
// se puede verificar sin red: el ruteo entre las dos bases y el armado de la
// cadena de conexión.

func conexionFalsa(t *testing.T) *gorm.DB {
	t.Helper()

	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gormDB, err := gorm.Open(
		postgresGorm.New(postgresGorm.Config{Conn: sqlDB}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return gormDB
}

// Connection tiene que devolver la instancia de la base pedida y no la otra.
// Cruzarlas mandaría los insumos a calculator-prices y el resultado a
// file-compiler, sin que nada falle.
func TestUnit_ConnectionDevuelveLaBasePedida(t *testing.T) {
	fileCompiler := conexionFalsa(t)
	calculatorPrices := conexionFalsa(t)

	db := postgres.NewLiquidationsDB(map[postgres.LiquidationsDatabase]*gorm.DB{
		postgres.LiqDBFileCompiler:     fileCompiler,
		postgres.LiqDBCalculatorPrices: calculatorPrices,
	})

	if db.Connection(postgres.LiqDBFileCompiler) != fileCompiler {
		t.Error("file-compiler devolvió otra instancia")
	}
	if db.Connection(postgres.LiqDBCalculatorPrices) != calculatorPrices {
		t.Error("calculator-prices devolvió otra instancia")
	}
}

// Pedir una base que no está registrada es un error de programación, no de
// configuración: tiene que reventar de una y no devolver nil, porque un nil se
// manifestaría mucho más lejos y como otra cosa.
func TestUnit_ConnectionDeBaseDesconocidaPanica(t *testing.T) {
	db := postgres.NewLiquidationsDB(map[postgres.LiquidationsDatabase]*gorm.DB{
		postgres.LiqDBFileCompiler: conexionFalsa(t),
	})

	defer func() {
		if recover() == nil {
			t.Error("no panicó al pedir una base sin registrar")
		}
	}()

	db.Connection(postgres.LiquidationsDatabase("base-que-no-existe"))
}

// Con LiqSQLDebug activo devuelve una sesión en modo debug, que es otra instancia
// —no la registrada—. Sin él, la misma. Importa porque el modo debug loguea cada
// consulta y en producción no debe estar.
func TestUnit_ConnectionRespetaElModoDebug(t *testing.T) {
	original := entities.LiqSQLDebug
	t.Cleanup(func() { entities.LiqSQLDebug = original })

	registrada := conexionFalsa(t)
	db := postgres.NewLiquidationsDB(map[postgres.LiquidationsDatabase]*gorm.DB{
		postgres.LiqDBFileCompiler: registrada,
	})

	entities.LiqSQLDebug = false
	if db.Connection(postgres.LiqDBFileCompiler) != registrada {
		t.Error("sin debug debería devolver la conexión tal cual")
	}

	entities.LiqSQLDebug = true
	if db.Connection(postgres.LiqDBFileCompiler) == registrada {
		t.Error("con debug debería devolver una sesión aparte")
	}
}
