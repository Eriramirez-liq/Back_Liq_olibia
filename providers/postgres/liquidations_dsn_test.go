package postgres

// Test interno (package postgres, no postgres_test) porque cadenaDeConexion no
// está exportada.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.

import (
	"strings"
	"testing"

	"bia-bills/entities"

	"github.com/jackc/pgx/v5"
)

// El DSN se valida con el MISMO parser que lo va a consumir en producción (pgx),
// no comparando strings: lo que importa no es cómo se ve, es que pgx saque de ahí
// las credenciales correctas.
func TestCadenaDeConexion(t *testing.T) {
	casos := []struct {
		nombre string
		pass   string
	}{
		{"contraseña común", "alfanumerica123"},
		// Los cuatro que en el formato con Sprintf daban problema. `/` y `#`
		// directamente no parseaban: el servicio no conectaba y el error hablaba de
		// un puerto inválido, que no ayuda a encontrar la causa.
		{"con arroba", "con@arroba"},
		{"con slash", "con/slash"},
		{"con numeral", "con#gato"},
		{"con dos puntos", "con:dospuntos"},
		{"con espacio", "con espacio"},
		{"con signo de pregunta", "con?pregunta"},
	}

	original := struct{ host, port, user, pass, ssl string }{
		entities.LiqDbHost, entities.LiqDbPort, entities.LiqDbUser,
		entities.LiqDbPass, entities.LiqDbSSLMode,
	}
	t.Cleanup(func() {
		entities.LiqDbHost, entities.LiqDbPort = original.host, original.port
		entities.LiqDbUser, entities.LiqDbPass = original.user, original.pass
		entities.LiqDbSSLMode = original.ssl
	})

	entities.LiqDbHost = "c4-rds-bia-dev.dev.bia.app"
	entities.LiqDbPort = "5432"
	entities.LiqDbUser = "usuario"
	entities.LiqDbSSLMode = "require"

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			entities.LiqDbPass = caso.pass

			cfg, err := pgx.ParseConfig(cadenaDeConexion("file-compiler"))
			if err != nil {
				t.Fatalf("pgx no pudo parsear el DSN: %v", err)
			}

			if cfg.User != "usuario" {
				t.Errorf("user = %q", cfg.User)
			}
			// Lo importante: la contraseña llega ENTERA, sin el escapado adentro.
			if cfg.Password != caso.pass {
				t.Errorf("password = %q, se esperaba %q", cfg.Password, caso.pass)
			}
			if cfg.Host != "c4-rds-bia-dev.dev.bia.app" {
				t.Errorf("host = %q", cfg.Host)
			}
			if cfg.Port != 5432 {
				t.Errorf("port = %d", cfg.Port)
			}
			if cfg.Database != "file-compiler" {
				t.Errorf("database = %q", cfg.Database)
			}
		})
	}
}

// Cada base tiene que quedar en su propio DSN. Si `abrir` recibiera el nombre y lo
// ignorara, las dos conexiones irían a la misma base y el módulo escribiría todo
// en una sola sin que nada falle.
func TestCadenaDeConexionUsaElNombreQueRecibe(t *testing.T) {
	passOriginal := entities.LiqDbPass
	hostOriginal := entities.LiqDbHost
	t.Cleanup(func() { entities.LiqDbPass, entities.LiqDbHost = passOriginal, hostOriginal })
	entities.LiqDbPass = "x"
	entities.LiqDbHost = "host.example"

	for _, base := range []string{"file-compiler", "calculator-prices"} {
		cfg, err := pgx.ParseConfig(cadenaDeConexion(base))
		if err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		if cfg.Database != base {
			t.Errorf("para %q el DSN apunta a %q", base, cfg.Database)
		}
	}
}

// El sslmode viaja como query param. En dev va "require"; si se perdiera, el RDS
// rechaza la conexión y el error no dice que faltaba TLS.
func TestCadenaDeConexionLlevaElSslmode(t *testing.T) {
	sslOriginal := entities.LiqDbSSLMode
	passOriginal := entities.LiqDbPass
	t.Cleanup(func() { entities.LiqDbSSLMode, entities.LiqDbPass = sslOriginal, passOriginal })
	entities.LiqDbPass = "x"

	entities.LiqDbSSLMode = "require"
	if dsn := cadenaDeConexion("file-compiler"); !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("falta el sslmode: %s", dsn)
	}

	entities.LiqDbSSLMode = "disable"
	if dsn := cadenaDeConexion("file-compiler"); !strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("no respetó el sslmode configurado: %s", dsn)
	}
}
