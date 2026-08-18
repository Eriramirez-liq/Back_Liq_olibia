package tarifas_sdl_test

import (
	"math"
	"testing"

	"bia-bills/services/tarifas_sdl"
)

// Test dorado del motor de tarifas SDL.
//
// Los números NO son inventados: son los insumos y las tarifas de CENS para
// 2026-01 tal como los produce el proceso del negocio hoy. Se verificaron las
// diez tarifas al centavo antes de escribir este test.
//
// Es la misma disciplina que en Cargos STR: el port a Go no se da por bueno hasta
// que reproduce los números del proceso actual. Un error acá no rompe nada
// visible — devuelve tarifas plausibles y equivocadas, y las tarifas alimentan la
// facturación.

// Insumos de CENS, 2026-01.
//
// CENS es tipo ADD, así que sus NT salen de los DT del ADD de CENTRO. Sus propios
// DT del archivo de uso de la red se guardan pero NO entran en el cálculo — por
// eso acá aparecen los del ADD.
const (
	censDT1ADD = 317.1201141
	censDT2ADD = 198.4777546
	censDT3ADD = 88.97771469

	censCDI  = 53.77485343
	censCDN4 = 34.85239617

	// Fracciones. En el Excel se ven como 13,8011536 % — el "%" es formato.
	censPR1 = 0.138011536
	censPR2 = 0.037078059
	censPR3 = 0.038380691
)

func componentesCENS() tarifas_sdl.Componentes {
	return tarifas_sdl.Componentes{
		NT1: censDT1ADD, NT2: censDT2ADD, NT3: censDT3ADD,
		CDI: censCDI, CDN4: censCDN4,
		PR1: censPR1, PR2: censPR2, PR3: censPR3,
	}
}

func TestCalcular_CENS(t *testing.T) {
	got := tarifas_sdl.Calcular(componentesCENS())

	casos := []struct {
		columna  string
		obtenido float64
		esperado float64
	}{
		{"active_level_1_operator", got.Activa.Nivel1Operador, 276.69},
		{"active_level_1_shared", got.Activa.Nivel1Compartido, 249.80},
		{"active_level_1_user", got.Activa.Nivel1Usuario, 222.91},
		{"active_level_2_user", got.Activa.Nivel2Usuario, 162.28},
		{"active_level_3_user", got.Activa.Nivel3Usuario, 52.73},

		{"reactive_level_1_operator", got.Reactiva.Nivel1Operador, 317.12},
		{"reactive_level_1_shared", got.Reactiva.Nivel1Compartido, 290.23},
		{"reactive_level_1_user", got.Reactiva.Nivel1Usuario, 263.35},
		{"reactive_level_2_user", got.Reactiva.Nivel2Usuario, 198.48},
		{"reactive_level_3_user", got.Reactiva.Nivel3Usuario, 88.98},
	}

	for _, caso := range casos {
		// Se compara a dos decimales porque así están tabuladas las tarifas del
		// negocio; el cálculo interno conserva toda la precisión.
		if math.Abs(redondear(caso.obtenido, 2)-caso.esperado) > 0.0001 {
			t.Errorf("%s = %.6f (redondeado %.2f), se esperaba %.2f",
				caso.columna, caso.obtenido, redondear(caso.obtenido, 2), caso.esperado)
		}
	}
}

// La reactiva de nivel 1 operador es el NT1 sin tocar. Es la comprobación más
// directa de que el NT se tomó del archivo correcto: si un operador tipo ADD
// tomara su NT del archivo de uso de la red, este valor cambiaría.
func TestCalcular_ReactivaNivel1EsElNTSinTocar(t *testing.T) {
	got := tarifas_sdl.Calcular(componentesCENS())

	if got.Reactiva.Nivel1Operador != censDT1ADD {
		t.Errorf("reactiva nivel 1 operador = %.7f, se esperaba el NT1 tal cual (%.7f)",
			got.Reactiva.Nivel1Operador, censDT1ADD)
	}
	if got.Reactiva.Nivel2Usuario != censDT2ADD {
		t.Errorf("reactiva nivel 2 = %.7f, se esperaba %.7f", got.Reactiva.Nivel2Usuario, censDT2ADD)
	}
	if got.Reactiva.Nivel3Usuario != censDT3ADD {
		t.Errorf("reactiva nivel 3 = %.7f, se esperaba %.7f", got.Reactiva.Nivel3Usuario, censDT3ADD)
	}
}

// El CDI descuenta la mitad en compartido y todo en usuario. Se verifica sobre la
// reactiva, donde no interviene el CDN4 y la relación queda aislada.
func TestCalcular_ElCDIDescuentaMitadYTodo(t *testing.T) {
	got := tarifas_sdl.Calcular(componentesCENS())

	mitad := got.Reactiva.Nivel1Operador - got.Reactiva.Nivel1Compartido
	todo := got.Reactiva.Nivel1Operador - got.Reactiva.Nivel1Usuario

	if math.Abs(mitad-censCDI*0.5) > 1e-9 {
		t.Errorf("compartido descuenta %.7f, se esperaba la mitad del CDI (%.7f)", mitad, censCDI*0.5)
	}
	if math.Abs(todo-censCDI) > 1e-9 {
		t.Errorf("usuario descuenta %.7f, se esperaba el CDI completo (%.7f)", todo, censCDI)
	}
}

// Si los PR llegaran como porcentaje (13.8) en vez de fracción (0.138), el
// divisor (1-PR) queda NEGATIVO y la tarifa activa sale por encima de la
// reactiva, que es imposible: la activa siempre descuenta el CDN4.
//
// Este test no prueba el motor, prueba que el error se pueda detectar. Es la
// confusión más fácil de cometer con estos datos.
func TestCalcular_PRComoPorcentajeDaUnResultadoImposible(t *testing.T) {
	bien := tarifas_sdl.Calcular(componentesCENS())

	mal := componentesCENS()
	mal.PR1, mal.PR2, mal.PR3 = 13.8011536, 3.7078059, 3.8380691 // el mismo dato, mal escalado
	roto := tarifas_sdl.Calcular(mal)

	if !(bien.Activa.Nivel1Operador < bien.Reactiva.Nivel1Operador) {
		t.Fatal("con PR como fracción, la activa debería ser menor que la reactiva")
	}
	if roto.Activa.Nivel1Operador <= roto.Reactiva.Nivel1Operador {
		t.Error("con PR como porcentaje el resultado debería ser imposible (activa > reactiva) y no lo es")
	}
}

// ── Catálogo de operadores ───────────────────────────────────────────────────

func TestCatalogo_TieneLos21Operadores(t *testing.T) {
	codigos := tarifas_sdl.OperatorCodes()

	if len(codigos) != 21 {
		t.Errorf("el catálogo tiene %d operadores, se esperaban 21: %v", len(codigos), codigos)
	}

	// Ordenados: el orden de un mapa en Go es aleatorio y en STR eso ya causó un
	// bug que ningún test detectaba.
	for i := 1; i < len(codigos); i++ {
		if codigos[i-1] > codigos[i] {
			t.Fatalf("el catálogo no está ordenado: %q antes de %q", codigos[i-1], codigos[i])
		}
	}
}

func TestCatalogo_TipoYAreaDeCadaOperador(t *testing.T) {
	casos := []struct {
		codigo string
		tipo   tarifas_sdl.TipoInsumo
		area   tarifas_sdl.AreaDistribucion // vacío = no tiene área
	}{
		{"CENS", tarifas_sdl.InsumoADD, tarifas_sdl.AreaCentro},
		{"CHEC", tarifas_sdl.InsumoADD, tarifas_sdl.AreaCentro},
		{"CEDENAR", tarifas_sdl.InsumoADD, tarifas_sdl.AreaOccidente},
		{"EBSA", tarifas_sdl.InsumoADD, tarifas_sdl.AreaOriente},
		{"EMSA", tarifas_sdl.InsumoADD, tarifas_sdl.AreaSur},
		// Los tipo USO no tienen área: su NT sale de su propio archivo.
		{"EPM", tarifas_sdl.InsumoUSO, ""},
		{"AIRE", tarifas_sdl.InsumoUSO, ""},
		{"AFINIA", tarifas_sdl.InsumoUSO, ""},
	}

	for _, caso := range casos {
		t.Run(caso.codigo, func(t *testing.T) {
			tipo, ok := tarifas_sdl.TipoDeOperador(caso.codigo)
			if !ok {
				t.Fatalf("%s no está en el catálogo", caso.codigo)
			}
			if tipo != caso.tipo {
				t.Errorf("tipo = %q, se esperaba %q", tipo, caso.tipo)
			}

			area, tieneArea := tarifas_sdl.AreaDeOperador(caso.codigo)
			if caso.area == "" {
				if tieneArea {
					t.Errorf("un operador tipo USO no debería tener área, tiene %q", area)
				}
				return
			}
			if !tieneArea {
				t.Fatalf("%s debería tener área", caso.codigo)
			}
			if area != caso.area {
				t.Errorf("área = %q, se esperaba %q", area, caso.area)
			}
		})
	}
}

// Todo operador tipo ADD necesita un área, o su NT no se puede resolver.
func TestCatalogo_TodoOperadorADDTieneArea(t *testing.T) {
	for _, codigo := range tarifas_sdl.OperatorCodes() {
		tipo, _ := tarifas_sdl.TipoDeOperador(codigo)
		if tipo != tarifas_sdl.InsumoADD {
			continue
		}
		if _, ok := tarifas_sdl.AreaDeOperador(codigo); !ok {
			t.Errorf("%s es tipo ADD y no tiene área asignada", codigo)
		}
	}
}

func redondear(v float64, decimales int) float64 {
	factor := math.Pow(10, float64(decimales))
	return math.Round(v*factor) / factor
}
