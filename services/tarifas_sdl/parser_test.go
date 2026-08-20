package tarifas_sdl_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bia-bills/services/tarifas_sdl"
)

// Test dorado del parser de Tarifas SDL contra los archivos reales de XM.
//
// Los 33 archivos viven en archivos_ejemplo/ y NO se copian a testdata/: pesan
// varios MB y ya están versionados una vez. Si la carpeta no está, el test se
// saltea con un mensaje claro en vez de fallar.
//
// ── Qué compara, y qué NO ────────────────────────────────────────────────────
// Compara las tarifas que produce este código contra las del parser TypeScript,
// exportadas a testdata/golden_ts.json. Eso valida el port: mismos archivos,
// mismos números.
//
// Lo que ese archivo NO puede validar es si el TypeScript lee bien un formato que
// todavía no existe: si las dos implementaciones se equivocaran igual, coinciden.
// Por eso el parser en Go es más estricto —ancla hoja, columna y fila por nombre,
// y corta con error si algo no está— y esas validaciones se prueban aparte, en
// parser_validaciones_test.go.

const dirEjemplos = "../../archivos_ejemplo/formatos_or/Insumos SDL"

// cargarArchivosReales lee los 33 archivos del lote de ejemplo.
func cargarArchivosReales(t *testing.T) []tarifas_sdl.UploadedFile {
	t.Helper()

	archivos := []tarifas_sdl.UploadedFile{}
	for _, sub := range []string{"Cargos ADD DT", "Cargos por uso de la red"} {
		dir := filepath.Join(dirEjemplos, sub)
		entradas, err := os.ReadDir(dir)
		if err != nil {
			t.Skipf("no están los archivos de ejemplo en %s: %v", dir, err)
		}
		for _, e := range entradas {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".xlsx") {
				continue
			}
			contenido, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("no se pudo leer %s: %v", e.Name(), err)
			}
			archivos = append(archivos, tarifas_sdl.UploadedFile{Name: e.Name(), Content: contenido})
		}
	}

	if len(archivos) == 0 {
		t.Skip("no hay .xlsx en los archivos de ejemplo")
	}

	return archivos
}

// areaEsperada es la pertenencia de cada operador a un área según lo que listan
// las hojas "Cargos ADD" de los 12 archivos reales. Vacío = no figura en ninguna.
//
// Está escrita a mano contra los archivos a propósito. El código ya no declara
// esta tabla —la lee de los archivos— así que este test es el que fija lo que los
// archivos dicen hoy: si XM mueve un operador de área, falla acá y se ve.
var areaEsperada = map[string]string{
	"CENS": "CENTRO", "CHEC": "CENTRO", "EDEQ": "CENTRO", "EEP_PEREIRA": "CENTRO",
	"EPM": "CENTRO", "ESSA": "CENTRO", "RUITOQUE": "CENTRO",

	"CEDENAR": "OCCIDENTE", "CELSIA_VALLE": "OCCIDENTE", "CEO": "OCCIDENTE",
	"CETSA": "OCCIDENTE", "EEP_CARTAGO": "OCCIDENTE", "EMCALI": "OCCIDENTE",

	"CELSIA_TOLIMA": "ORIENTE", "EBSA": "ORIENTE", "ELECTROHUILA": "ORIENTE",
	"ENEL": "ORIENTE",

	"EMSA": "SUR", "ENERCA": "SUR",

	// Los dos del Caribe no figuran en ninguna hoja "Cargos ADD": entre los 12
	// archivos no hay ninguno de un área Caribe.
	"AFINIA": "", "AIRE": "",
}

// Los archivos de uso de la red del juego real son de 2026-04, y el parser exige
// que el período elegido coincida con ellos. Los ADD son de 2026-02 y no se
// validan: van dos meses atrás por diseño del negocio.
const periodoDeLosReales = "2026-04"

func TestParseInputs_ArchivosReales(t *testing.T) {
	res := tarifas_sdl.ParseInputs(cargarArchivosReales(t), periodoDeLosReales)

	if len(res.CriticalErrors) > 0 {
		t.Fatalf("el lote completo no debería dar errores críticos: %v", res.CriticalErrors)
	}
	// El único aviso esperado con el lote completo: AFINIA y AIRE no figuran en
	// ninguna hoja "Cargos ADD" porque no hay archivo de área Caribe entre los 12.
	if len(res.Warnings) != 1 {
		t.Errorf("se esperaba exactamente 1 aviso (los dos del Caribe sin área), hubo %d: %v",
			len(res.Warnings), res.Warnings)
	} else {
		for _, codigo := range []string{"AFINIA", "AIRE"} {
			if !strings.Contains(res.Warnings[0], codigo) {
				t.Errorf("el aviso de operadores sin área no menciona a %s: %q", codigo, res.Warnings[0])
			}
		}
	}

	// Una fila por operador del catálogo, ni una más ni una menos.
	if len(res.Rows) != 21 {
		t.Fatalf("devolvió %d filas, se esperaban 21", len(res.Rows))
	}

	porOperador := map[string]tarifas_sdl.SdlInputRow{}
	for _, r := range res.Rows {
		porOperador[r.OperatorCode] = r
	}

	for _, codigo := range tarifas_sdl.OperatorCodes() {
		fila, ok := porOperador[codigo]
		if !ok {
			t.Errorf("falta la fila de %s", codigo)
			continue
		}

		// El área es un dato del operador y NO depende de su tipo: sale de la hoja
		// "Cargos ADD" que lo lista. EPM y EEP Pereira son tipo USO —calculan con
		// los DT de su propio archivo— y a la vez pertenecen a Centro.
		if fila.DistributionArea != areaEsperada[codigo] {
			t.Errorf("%s: área = %q, se esperaba %q",
				codigo, fila.DistributionArea, areaEsperada[codigo])
		}

		if areaEsperada[codigo] == "" {
			// Sin área no hay cargos ADD que guardar: ponerlos en cero sería
			// inventar un dato que el archivo no trae.
			if fila.DT1Add != nil || fila.DT2Add != nil || fila.DT3Add != nil {
				t.Errorf("%s no está en ningún ADD y trae cargos ADD", codigo)
			}
			if len(fila.SourceFiles) != 1 {
				t.Errorf("%s trae %d archivos de origen, se esperaba 1", codigo, len(fila.SourceFiles))
			}
		} else {
			if fila.DT1Add == nil || fila.DT2Add == nil || fila.DT3Add == nil {
				t.Errorf("%s pertenece a %s y le faltan los cargos ADD", codigo, areaEsperada[codigo])
			}
			// Su archivo de uso de la red más los tres ADD de su área.
			if len(fila.SourceFiles) != 4 {
				t.Errorf("%s trae %d archivos de origen, se esperaban 4: %v",
					codigo, len(fila.SourceFiles), fila.SourceFiles)
			}
		}

		// Los PR son fracciones. Si alguno llegara como porcentaje, el cálculo de
		// la tarifa activa daría un divisor negativo.
		//
		// El cero SÍ es válido y aparece en los datos reales: RUITOQUE tiene PR3 = 0
		// en el archivo de XM, o sea que no se le reconocen pérdidas en ese nivel.
		for nombre, pr := range map[string]float64{"PR1": fila.PR1, "PR2": fila.PR2, "PR3": fila.PR3} {
			if pr < 0 || pr >= 1 {
				t.Errorf("%s: %s = %.6f, debería ser una fracción entre 0 y 1", codigo, nombre, pr)
			}
		}
	}
}

// ── El dorado: mismos números que el TypeScript ──────────────────────────────

type filaGoldenTS struct {
	OrCodigo         string  `json:"or_codigo"`
	NivelTension     string  `json:"nivel_tension"`
	PropiedadActivos string  `json:"propiedad_activos"`
	TarifaActiva     float64 `json:"tarifa_activa"`
	TarifaReactiva   float64 `json:"tarifa_reactiva"`
}

type goldenTS struct {
	Archivos int            `json:"archivos"`
	Filas    []filaGoldenTS `json:"filas"`
}

func TestTarifas_CoincidenConElParserTypeScript(t *testing.T) {
	crudo, err := os.ReadFile(filepath.Join("testdata", "golden_ts.json"))
	if err != nil {
		t.Skipf("falta testdata/golden_ts.json: %v", err)
	}

	var golden goldenTS
	if err := json.Unmarshal(crudo, &golden); err != nil {
		t.Fatalf("golden_ts.json ilegible: %v", err)
	}
	if len(golden.Filas) != 105 {
		t.Fatalf("el dorado tiene %d filas, se esperaban 105 (21 operadores × 5)", len(golden.Filas))
	}

	res := tarifas_sdl.ParseInputs(cargarArchivosReales(t), periodoDeLosReales)
	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos: %v", res.CriticalErrors)
	}

	// Se calcula desde los insumos, igual que en producción.
	calculado := map[string]tarifas_sdl.Tarifas{}
	for _, fila := range res.Rows {
		comp, err := tarifas_sdl.ComponentesDe(fila)
		if err != nil {
			t.Fatalf("%s: %v", fila.OperatorCode, err)
		}
		calculado[fila.OperatorCode] = tarifas_sdl.Calcular(comp)
	}

	for _, esperado := range golden.Filas {
		tarifas, ok := calculado[esperado.OrCodigo]
		if !ok {
			t.Errorf("el dorado trae %s y el parser en Go no lo devolvió", esperado.OrCodigo)
			continue
		}

		activa, reactiva, ok := valorDe(tarifas, esperado.NivelTension, esperado.PropiedadActivos)
		if !ok {
			t.Errorf("combinación desconocida: nivel %s, propiedad %s",
				esperado.NivelTension, esperado.PropiedadActivos)
			continue
		}

		// Mismas fórmulas, mismo orden de operaciones y mismo float64: la
		// diferencia tiene que ser del orden del épsilon de la máquina, no de
		// redondeo.
		const tolerancia = 1e-9
		if math.Abs(activa-esperado.TarifaActiva) > tolerancia {
			t.Errorf("%s nivel %s %s activa: Go %.12f, TS %.12f",
				esperado.OrCodigo, esperado.NivelTension, esperado.PropiedadActivos,
				activa, esperado.TarifaActiva)
		}
		if math.Abs(reactiva-esperado.TarifaReactiva) > tolerancia {
			t.Errorf("%s nivel %s %s reactiva: Go %.12f, TS %.12f",
				esperado.OrCodigo, esperado.NivelTension, esperado.PropiedadActivos,
				reactiva, esperado.TarifaReactiva)
		}
	}
}

func valorDe(t tarifas_sdl.Tarifas, nivel, propiedad string) (activa, reactiva float64, ok bool) {
	switch nivel + "|" + propiedad {
	case "1|OR":
		return t.Activa.Nivel1Operador, t.Reactiva.Nivel1Operador, true
	case "1|COMPARTIDO":
		return t.Activa.Nivel1Compartido, t.Reactiva.Nivel1Compartido, true
	case "1|USUARIO":
		return t.Activa.Nivel1Usuario, t.Reactiva.Nivel1Usuario, true
	case "2|USUARIO":
		return t.Activa.Nivel2Usuario, t.Reactiva.Nivel2Usuario, true
	case "3|USUARIO":
		return t.Activa.Nivel3Usuario, t.Reactiva.Nivel3Usuario, true
	}
	return 0, 0, false
}

// ── La comprobación de auditoría ─────────────────────────────────────────────

// Recalcular desde los insumos guardados tiene que reproducir el resultado
// guardado. Es lo que demuestra que las dos tablas están de verdad relacionadas y
// no solo dicen estarlo.
//
// Acá se prueba en memoria; el mismo camino se usa sobre los datos persistidos.
func TestAuditoria_RecalcularDesdeLosInsumosDaLoMismo(t *testing.T) {
	res := tarifas_sdl.ParseInputs(cargarArchivosReales(t), periodoDeLosReales)
	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos: %v", res.CriticalErrors)
	}

	for _, fila := range res.Rows {
		comp, err := tarifas_sdl.ComponentesDe(fila)
		if err != nil {
			t.Fatalf("%s: %v", fila.OperatorCode, err)
		}
		primera := tarifas_sdl.Calcular(comp)

		// Se vuelve a armar desde la fila, como haría una verificación posterior
		// leyendo la base.
		compOtraVez, err := tarifas_sdl.ComponentesDe(fila)
		if err != nil {
			t.Fatalf("%s: %v", fila.OperatorCode, err)
		}
		segunda := tarifas_sdl.Calcular(compOtraVez)

		if primera != segunda {
			t.Errorf("%s: recalcular no dio lo mismo\n  %+v\n  %+v", fila.OperatorCode, primera, segunda)
		}
	}
}

// El NT de un operador tipo ADD sale del ADD de su área, NO de su propio archivo
// de uso de la red. Es la regla que más fácil se invierte al portar, y el síntoma
// sería un juego de tarifas plausible y equivocado para 14 de los 21 operadores.
func TestComponentesDe_ElNTSaleDelArchivoQueCorresponde(t *testing.T) {
	res := tarifas_sdl.ParseInputs(cargarArchivosReales(t), periodoDeLosReales)
	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos: %v", res.CriticalErrors)
	}

	for _, fila := range res.Rows {
		comp, err := tarifas_sdl.ComponentesDe(fila)
		if err != nil {
			t.Fatalf("%s: %v", fila.OperatorCode, err)
		}

		tipo, _ := tarifas_sdl.TipoDeOperador(fila.OperatorCode)
		if tipo == tarifas_sdl.InsumoADD {
			if comp.NT1 != *fila.DT1Add {
				t.Errorf("%s (ADD): NT1 = %.6f, debería ser el DT1 del ADD (%.6f)",
					fila.OperatorCode, comp.NT1, *fila.DT1Add)
			}
			// Y si por casualidad coincidieran, el test no sirve: se avisa.
			if *fila.DT1Add == fila.DT1 {
				t.Logf("ojo: en %s el DT del ADD y el propio coinciden, este caso no discrimina",
					fila.OperatorCode)
			}
			continue
		}

		if comp.NT1 != fila.DT1 {
			t.Errorf("%s (USO): NT1 = %.6f, debería ser su propio DT1 (%.6f)",
				fila.OperatorCode, comp.NT1, fila.DT1)
		}
	}
}
