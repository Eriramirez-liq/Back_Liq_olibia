package postgres

import (
	"fmt"
	"log"

	"bia-bills/entities"

	"github.com/jackc/pgx/v5/stdlib"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Conexiones a las bases externas del módulo de Liquidaciones.
//
// ⚠️ TRASVASE: archivo `liquidations_database.go`, aparte de `database.go`, para
// no pisar el provider del servicio. Ver docs/backend/migracion-a-go.md.
//
// ── Por qué no dblink ────────────────────────────────────────────────────────
// bia-bills ya consulta otras bases con `dblink` (ver UserDblinkConnstr y
// ConsumptionsDblinkConnstr en entities/const.go), y para LEER es un patrón
// razonable. Acá no alcanza: Cargos STR **escribe** en las dos bases, y por
// dblink eso significa `dblink_exec` con SQL armado como string — se pierde el
// parametrizado y no hay transacción. Con 23 filas por carga es tolerable, pero
// es peor por donde se lo mire.
//
// Por eso este provider abre conexiones propias. Es una desviación consciente
// del patrón del servicio y conviene mencionarla en el PR del trasvase.

// LiquidationsDatabase identifica cada base externa del módulo.
type LiquidationsDatabase string

const (
	LiqDBFileCompiler     LiquidationsDatabase = "file-compiler"
	LiqDBCalculatorPrices LiquidationsDatabase = "calculator-prices"
)

// LiquidationsDB entrega la conexión de cada base externa.
type LiquidationsDB interface {
	Connection(db LiquidationsDatabase) *gorm.DB
}

type liquidationsDB struct {
	connections map[LiquidationsDatabase]*gorm.DB
}

// NewLiquidationsDB abre las conexiones a las bases externas.
//
// Con `DisableAutomaticPing` el abrir NO conecta: el pool conecta con la primera
// consulta. Es deliberado — sin eso, GORM hace un ping al abrir y el servicio se
// negaría a arrancar si una de las dos bases está caída un momento. Con esto
// arranca igual y el health check reporta el problema, que es lo correcto: una
// base externa intermitente no debe impedir que el resto del servicio atienda.
//
// Acepta instancias ya armadas para poder inyectar mocks en los tests, igual que
// NewPostgresDB del servicio.
func NewLiquidationsDB(instances ...map[LiquidationsDatabase]*gorm.DB) LiquidationsDB {
	if len(instances) > 0 {
		return liquidationsDB{connections: instances[0]}
	}

	log.Println("Init liquidations dbs")

	nombres := map[LiquidationsDatabase]string{
		LiqDBFileCompiler:     entities.LiqDbNameFileCompiler,
		LiqDBCalculatorPrices: entities.LiqDbNameCalculatorPrices,
	}

	conexiones := make(map[LiquidationsDatabase]*gorm.DB, len(nombres))
	for id, nombre := range nombres {
		conexiones[id] = abrir(nombre)
	}

	return liquidationsDB{connections: conexiones}
}

func abrir(nombreBase string) *gorm.DB {
	// El driver se registra una sola vez por proceso; si el servicio ya lo hizo
	// en su propio provider, este Register es un no-op.
	sqltrace.Register("pgx", &stdlib.Driver{}, sqltrace.WithServiceName("gorm.db"))

	sqlDb, err := sqltrace.Open("pgx", cadenaDeConexion(nombreBase))
	if err != nil {
		log.Fatalf("liquidations: no se pudo abrir la base %s: %v", nombreBase, err)
	}

	instancia, err := gormtrace.Open(
		postgresGorm.New(postgresGorm.Config{Conn: sqlDb}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		log.Fatalf("liquidations: no se pudo instanciar gorm para %s: %v", nombreBase, err)
	}

	return instancia
}

func cadenaDeConexion(nombreBase string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		entities.LiqDbUser,
		entities.LiqDbPass,
		entities.LiqDbHost,
		entities.LiqDbPort,
		nombreBase,
		entities.LiqDbSSLMode,
	)
}

func (db liquidationsDB) Connection(id LiquidationsDatabase) *gorm.DB {
	conexion := db.connections[id]
	if conexion == nil {
		// Programación, no configuración: se pidió una base que no está en el mapa.
		log.Panicf("liquidations: no hay conexión registrada para %q", id)
	}

	if entities.LiqSQLDebug {
		return conexion.Debug()
	}

	return conexion
}
