package repositories_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bia-bills/providers/postgres"
	"bia-bills/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	postgresGorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Tests del catálogo de nombres con sqlmock, SIN base de datos.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// El mismo motivo que en liquidations_str_unit_test.go: `public.agents` vive en
// file-compiler, un RDS externo que el CI de bia-bills no alcanza.

// mocksAgentes reusa el armado de conexiones falsas del test del repositorio de
// cargos. Solo se usa file-compiler: el catálogo de agentes vive ahí.
func mocksAgentes(t *testing.T) (repositories.LiquidationsAgentsRepository, sqlmock.Sqlmock, func() []string) {
	t.Helper()

	ejecutado := []string{}

	espia := sqlmock.QueryMatcherFunc(func(esperado, actual string) error {
		ejecutado = append(ejecutado, actual)
		return sqlmock.QueryMatcherRegexp.Match(esperado, actual)
	})

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(espia))
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

	db := postgres.NewLiquidationsDB(map[postgres.LiquidationsDatabase]*gorm.DB{
		postgres.LiqDBFileCompiler: gormDB,
	})

	return repositories.NewLiquidationsAgentsRepository(db), mock, func() []string { return ejecutado }
}

func TestUnit_NamesByOperator_MapeaCodigoAOperador(t *testing.T) {
	repo, mock, _ := mocksAgentes(t)

	// Dos códigos que apuntan al mismo operador, como AIRE en los archivos reales.
	homologacion := map[string]string{
		"CSID": "AIRE",
		"CSSD": "AIRE",
		"CHCD": "CHEC",
	}

	mock.ExpectQuery(`FROM public.agents`).
		WillReturnRows(sqlmock.NewRows([]string{"code", "name"}).
			AddRow("CSID", "AIR-E S.A.S. E.S.P.").
			AddRow("CHCD", "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P."))

	nombres, err := repo.NamesByOperator(context.Background(), homologacion)
	if err != nil {
		t.Fatalf("NamesByOperator: %v", err)
	}

	if nombres["AIRE"] != "AIR-E S.A.S. E.S.P." {
		t.Errorf("AIRE = %q", nombres["AIRE"])
	}
	if nombres["CHEC"] != "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P." {
		t.Errorf("CHEC = %q", nombres["CHEC"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("file-compiler: %v", err)
	}
}

// La consulta normaliza con upper(trim(...)) en el SQL, pero el mapeo de vuelta
// también tiene que normalizar: si la base devolviera " chcd ", el código no
// coincidiría con la llave de la homologación y el nombre se perdería en silencio
// —que es exactamente el síntoma que tuvo el bug del ANY—.
func TestUnit_NamesByOperator_NormalizaElCodigoQueVuelve(t *testing.T) {
	repo, mock, _ := mocksAgentes(t)

	mock.ExpectQuery(`FROM public.agents`).
		WillReturnRows(sqlmock.NewRows([]string{"code", "name"}).
			AddRow("  chcd  ", "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P."))

	nombres, err := repo.NamesByOperator(context.Background(), map[string]string{"CHCD": "CHEC"})
	if err != nil {
		t.Fatalf("NamesByOperator: %v", err)
	}

	if nombres["CHEC"] == "" {
		t.Errorf("se perdió el nombre por espacios/minúsculas en el código: %+v", nombres)
	}
}

// Un código que no está en la homologación se descarta, y un nombre vacío no se
// guarda: el servicio distingue "sin nombre" para avisar, y un "" pasaría por
// nombre resuelto.
func TestUnit_NamesByOperator_DescartaLoQueNoSirve(t *testing.T) {
	repo, mock, _ := mocksAgentes(t)

	mock.ExpectQuery(`FROM public.agents`).
		WillReturnRows(sqlmock.NewRows([]string{"code", "name"}).
			AddRow("CHCD", "").     // vino sin nombre
			AddRow("XXXX", "Otro")) // código que no pedimos

	nombres, err := repo.NamesByOperator(context.Background(), map[string]string{"CHCD": "CHEC"})
	if err != nil {
		t.Fatalf("NamesByOperator: %v", err)
	}

	if len(nombres) != 0 {
		t.Errorf("guardó nombres que debía descartar: %+v", nombres)
	}
}

// Filtra por OPERADOR DE RED y por deleted_at, y usa IN (?) —no ANY(?)—. Esta
// consulta fue una de las cuatro que rompió con ANY, y su falla era invisible: el
// preview respondía 200 con los montos bien y los nombres en blanco.
func TestUnit_NamesByOperator_SQLFiltraYUsaIn(t *testing.T) {
	repo, mock, sqlEjecutado := mocksAgentes(t)

	mock.ExpectQuery(`.*`).WillReturnRows(sqlmock.NewRows([]string{"code", "name"}))

	_, err := repo.NamesByOperator(context.Background(), map[string]string{
		"CHCD": "CHEC", "CSID": "AIRE", "EPMD": "EPM",
	})
	if err != nil {
		t.Fatalf("NamesByOperator: %v", err)
	}

	query := sqlEjecutado()[0]

	for _, fragmento := range []string{
		"upper(trim(code)) IN (",
		"activity = ",
		"deleted_at IS NULL",
	} {
		if !strings.Contains(query, fragmento) {
			t.Errorf("al SQL le falta %q:\n%s", fragmento, query)
		}
	}
	if strings.Contains(query, "ANY(") || strings.Contains(query, "ANY (") {
		t.Errorf("volvió el ANY(?) que GORM no expande:\n%s", query)
	}

	// 3 códigos expandidos + la actividad = 4 placeholders.
	if n := strings.Count(query, "$"); n != 4 {
		t.Errorf("hay %d placeholders, se esperaban 4 (3 códigos + actividad):\n%s", n, query)
	}
}

func TestUnit_NamesByOperator_PropagaElError(t *testing.T) {
	repo, mock, _ := mocksAgentes(t)

	mock.ExpectQuery(`FROM public.agents`).WillReturnError(errors.New("catálogo caído"))

	nombres, err := repo.NamesByOperator(context.Background(), map[string]string{"CHCD": "CHEC"})
	if err == nil {
		t.Fatal("no propagó el error del catálogo")
	}
	// Importa que devuelva nil y no un mapa vacío: el servicio degrada con aviso
	// cuando el catálogo falla, y un mapa vacío se vería como "no hay nombres".
	if nombres != nil {
		t.Errorf("devolvió %+v en vez de nil", nombres)
	}
}
