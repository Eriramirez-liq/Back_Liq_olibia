package repositories

import (
	"context"
	"strings"

	"bia-bills/providers/postgres"
)

// Nombres de los operadores de red, desde el catálogo de agentes de XM.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// No hay catálogo propio: los nombres salen de `public.agents` en la base
// file-compiler, que ya contiene las 24 abreviaturas que aparecen como
// encabezados en los archivos BalanceSTR — incluidos DISPAC, ENERGUAVIARE y
// PUTUMAYO, que NO están en `public.operators` de calculator-prices.
//
// Cuando un operador llega en varias columnas (AIRE = CSID + CSSD), el nombre se
// toma del agente con actividad OPERADOR DE RED. Verificado contra la base: hay
// exactamente uno por operador en los 23, y para AIRE elige el registro vigente
// en vez del que figura como intervenido.

const actividadOperadorDeRed = "OPERADOR DE RED"

type LiquidationsAgentsRepository interface {
	// NamesByOperator devuelve operador → nombre legal. Los operadores sin agente
	// principal en el catálogo no aparecen; el llamador decide qué hacer.
	NamesByOperator(ctx context.Context, homologation map[string]string) (map[string]string, error)

	// NamesByAgentCode devuelve código de agente → nombre legal.
	//
	// Es la misma consulta que NamesByOperator con el mismo filtro de actividad,
	// pero sin homologar: la usa Tarifas SDL, donde el código de agente llega en el
	// propio archivo y no hay nada que traducir.
	//
	// Que las dos pasen por el MISMO filtro es lo que garantiza que un operador se
	// llame igual en las tablas de los dos módulos. Si SDL resolviera el nombre por
	// su cuenta, Air-e aparecería como "AIR- E S.A.S. E.S.P. - INTERVENIDO" en un
	// lado y "AIR- E S.A.S. E.S.P." en el otro.
	NamesByAgentCode(ctx context.Context, codes []string) (map[string]string, error)
}

type liquidationsAgentsRepository struct {
	db postgres.LiquidationsDB
}

func NewLiquidationsAgentsRepository(db postgres.LiquidationsDB) LiquidationsAgentsRepository {
	return liquidationsAgentsRepository{db: db}
}

func (repository liquidationsAgentsRepository) NamesByOperator(
	ctx context.Context,
	homologation map[string]string,
) (map[string]string, error) {
	codigos := make([]string, 0, len(homologation))
	for code := range homologation {
		codigos = append(codigos, code)
	}

	filas := []struct {
		Code string
		Name string
	}{}

	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(`SELECT upper(trim(code)) AS code, name
		       FROM public.agents
		      WHERE upper(trim(code)) IN (?)
		        AND activity = ?
		        AND deleted_at IS NULL`, codigos, actividadOperadorDeRed).
		Scan(&filas).Error
	if err != nil {
		return nil, err
	}

	nombres := make(map[string]string, len(filas))
	for _, f := range filas {
		operador, ok := homologation[strings.ToUpper(strings.TrimSpace(f.Code))]
		if !ok || f.Name == "" {
			continue
		}
		nombres[operador] = f.Name
	}

	return nombres, nil
}

func (repository liquidationsAgentsRepository) NamesByAgentCode(
	ctx context.Context,
	codes []string,
) (map[string]string, error) {
	nombres := map[string]string{}
	if len(codes) == 0 {
		return nombres, nil
	}

	filas := []struct {
		Code string
		Name string
	}{}

	// IN (?) y no = ANY(?): GORM no expande slices en ANY.
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(`SELECT upper(trim(code)) AS code, name
		       FROM public.agents
		      WHERE upper(trim(code)) IN (?)
		        AND activity = ?
		        AND deleted_at IS NULL`, codes, actividadOperadorDeRed).
		Scan(&filas).Error
	if err != nil {
		return nil, err
	}

	for _, f := range filas {
		codigo := strings.ToUpper(strings.TrimSpace(f.Code))
		if codigo == "" || f.Name == "" {
			continue
		}
		nombres[codigo] = f.Name
	}

	return nombres, nil
}
