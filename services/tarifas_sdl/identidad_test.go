package tarifas_sdl_test

import (
	"testing"

	"bia-bills/services/tarifas_sdl"
)

// Identidad de cada archivo: operador, código de agente y mercado.
//
// El agente es lo que resuelve el nombre legal contra public.agents, así que si
// se lee mal, Tarifas SDL muestra un nombre distinto al de Cargos STR para el
// mismo operador. El mercado es lo que distingue a los dos pares que comparten
// razón social; sin él parecerían filas duplicadas.

func TestIdentidad_DeLos21Archivos(t *testing.T) {
	res := tarifas_sdl.ParseInputs(cargarArchivosReales(t))
	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos: %v", res.CriticalErrors)
	}

	esperado := map[string]struct{ agente, mercado string }{
		"AFINIA":        {"CMMD", "CARIBE MAR"},
		"AIRE":          {"CSID", "CARIBE SOL"},
		"CEDENAR":       {"CDND", "NARIÑO"},
		"CELSIA_TOLIMA": {"EPSD", "TOLIMA"},
		"CELSIA_VALLE":  {"EPSD", "VALLE DEL CAUCA"},
		"CENS":          {"CNSD", "NORTE DE SANTANDER"},
		"CEO":           {"CEOD", "CAUCA"},
		"CETSA":         {"CETD", "TULUA"},
		"CHEC":          {"CHCD", "CALDAS"},
		"EBSA":          {"EBSD", "BOYACA"},
		"EDEQ":          {"EDQD", "QUINDIO"},
		"EEP_CARTAGO":   {"EEPD", "CARTAGO"},
		"EEP_PEREIRA":   {"EEPD", "PEREIRA"},
		"ELECTROHUILA":  {"HLAD", "HUILA"},
		"EMCALI":        {"EMID", "CALI - YUMBO - PUERTO TEJADA"},
		"EMSA":          {"EMSD", "META"},
		"ENEL":          {"ENDD", "BOGOTA - CUNDINAMARCA"},
		"ENERCA":        {"CASD", "CASANARE"},
		"EPM":           {"EPMD", "ANTIOQUIA"},
		"ESSA":          {"ESSD", "SANTANDER"},
		"RUITOQUE":      {"RTQD", "RUITOQUE"},
	}

	for _, fila := range res.Rows {
		esp, ok := esperado[fila.OperatorCode]
		if !ok {
			t.Errorf("operador inesperado: %s", fila.OperatorCode)
			continue
		}
		if fila.AgentCode != esp.agente {
			t.Errorf("%s: agente = %q, se esperaba %q", fila.OperatorCode, fila.AgentCode, esp.agente)
		}
		// Los tres casos que rompieron una implementación anterior: un nombre con
		// guion (AIR-E), un mercado con guiones adentro (CALI - YUMBO - …) y una
		// vocal acentuada (NARIÑO), que desalineaba el corte por bytes.
		if fila.Market != esp.mercado {
			t.Errorf("%s: mercado = %q, se esperaba %q", fila.OperatorCode, fila.Market, esp.mercado)
		}
	}
}

// Dos pares comparten razón social y el negocio los quiere separados. Si el
// parser los uniera, se perderían dos filas sin que nada falle.
func TestIdentidad_LosParesQueComparteRazonSocialQuedanSeparados(t *testing.T) {
	res := tarifas_sdl.ParseInputs(cargarArchivosReales(t))
	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos: %v", res.CriticalErrors)
	}

	porAgente := map[string][]string{}
	for _, fila := range res.Rows {
		porAgente[fila.AgentCode] = append(porAgente[fila.AgentCode], fila.OperatorCode)
	}

	for agente, esperados := range map[string][]string{
		"EEPD": {"EEP_CARTAGO", "EEP_PEREIRA"},
		"EPSD": {"CELSIA_TOLIMA", "CELSIA_VALLE"},
	} {
		if len(porAgente[agente]) != 2 {
			t.Errorf("el agente %s debería tener 2 operadores (%v), tiene %v",
				agente, esperados, porAgente[agente])
		}
	}
}

// El alias existe por un solo caso, y conviene que se note si cambia: el archivo
// de Air-e trae CSID, pero en public.agents ese código está como DISTRIBUCIÓN y
// con el nombre "…- INTERVENIDO". El operador de red vigente es CSSD.
func TestIdentidad_AliasDeAgente(t *testing.T) {
	if got := tarifas_sdl.AgentCodeFor("CSID"); got != "CSSD" {
		t.Errorf("CSID debería resolverse como CSSD para el nombre, dio %q", got)
	}
	// Los demás pasan sin tocar.
	for _, codigo := range []string{"CMMD", "EPSD", "EEPD", "RTQD"} {
		if got := tarifas_sdl.AgentCodeFor(codigo); got != codigo {
			t.Errorf("%s no debería tener alias, dio %q", codigo, got)
		}
	}
}
