package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrArtifactNotFound cubre tanto "no existe" como "no es tuyo" — nunca
// se distingue cuál de las dos, mismo criterio que
// ErrConversationNotFound (ver Global Constraints).
var ErrArtifactNotFound = errors.New("artefacto no encontrado")

// ErrArtifactNotRerunnable: el artefacto es una instantánea (legacy o
// custom) y no tiene consulta que volver a correr.
var ErrArtifactNotRerunnable = errors.New("este artefacto no se puede actualizar")

// ErrArtifactSourceUnavailable: alguna fuente ya no se puede resolver a un
// monitor (renombrado sin respaldo posible, o borrado). Existe para que el
// handler pueda activar el modo degradado que pide el spec —mostrar la
// caché con un aviso de que la fuente ya no está disponible— sin comparar
// cadenas de error: un fallo de Mongo NO debe mostrar datos viejos como si
// siguieran vigentes, y sin este centinela las dos cosas son la misma.
var ErrArtifactSourceUnavailable = errors.New("la fuente del artefacto ya no está disponible")

// ArtifactRun es el resultado de ejecutar un artefacto: los datos ya
// proyectados y (para un chart multi-fuente) pivoteados, más el momento
// de la corrida.
type ArtifactRun struct {
	Data []map[string]interface{}
	// Series son las claves que hay que graficar en las filas de Data.
	// Existe porque el pivote RENOMBRA las columnas: con varias fuentes
	// las filas quedan {_id: "Caribe", "Bancolombia": 100, ...}, y el
	// ChartSpec.YKeys que escribió el LLM sigue diciendo ["aggValue"] —
	// una clave que ya no está en los datos. Sin este campo, todo gráfico
	// multi-fuente se renderiza vacío.
	//
	// Con una sola fuente vale ChartSpec.YKeys tal cual; con varias, los
	// labels de las fuentes. Quien renderiza usa SIEMPRE Series y nunca
	// YKeys directo.
	Series []string
	RanAt  time.Time
}

type ArtifactService struct {
	artifactRepo *repository.ChatArtifactRepository
	monitorRepo  *repository.MonitorRepository
}

func NewArtifactService(artifactRepo *repository.ChatArtifactRepository, monitorRepo *repository.MonitorRepository) *ArtifactService {
	return &ArtifactService{artifactRepo: artifactRepo, monitorRepo: monitorRepo}
}

// resolveArtifactMonitors traduce el alias de cada source al monitor real.
//
// ESTA ES LA FRONTERA DE AUTORIZACIÓN de un artefacto: `aliases` se
// construye solo a partir de art.MonitorIDs, así que un alias que no está
// ahí no puede convertirse en una consulta. Nunca se acepta un ObjectID
// crudo.
//
// RESPALDO POSICIONAL: el alias se deriva del NOMBRE del monitor —por eso
// se recalcula en cada request y nunca se persiste—, así que renombrar un
// monitor rompería todos los artefactos guardados que lo referencian. Pero
// art.MonitorIDs guarda ObjectIDs ESTABLES, así que con una allowlist de
// un solo monitor y una sola fuente, resolver por posición devuelve EL
// MISMO monitor contra el que se construyó el artefacto: no amplía el
// acceso —sigue siendo el único monitor de la allowlist—, solo recupera el
// artefacto.
//
// Con más de una fuente NO se adivina, y tampoco si la allowlist tiene más
// de un monitor: ahí elegir por posición podría consultar el monitor
// equivocado, y mostrar datos de otro monitor como si fueran los pedidos es
// peor que no mostrar nada.
//
// allowlistSize es len(art.MonitorIDs), y se exige APARTE de len(aliases)
// justamente porque no son lo mismo: `aliases` ya viene sin los monitores
// borrados. Si la allowlist fuera [A, B] con A borrado, `aliases` quedaría
// en 1 y una fuente que nombra a A se ejecutaría contra B en silencio —
// exactamente el "nunca se ejecuta contra un monitor distinto del que se
// resolvió" que el spec prohíbe. Con las dos condiciones, la precondición
// se autoexige y no depende de cómo otra tarea decida poblar MonitorIDs.
func resolveArtifactMonitors(aliases map[string]*models.Monitor, sources []models.ArtifactSource, allowlistSize int) ([]*models.Monitor, error) {
	out := make([]*models.Monitor, 0, len(sources))
	for i, src := range sources {
		monitor, err := resolveQueryMonitor(aliases, src.Monitor)
		if err != nil {
			if allowlistSize == 1 && len(aliases) == 1 && len(sources) == 1 {
				// El `continue` es correcto SOLO por el len(sources) == 1:
				// si esa guarda se relajara, cada fuente irresoluble
				// apilaría el mismo monitor y el chequeo de longitud de
				// RunArtifact no lo vería (las cuentas darían iguales).
				for _, only := range aliases {
					out = append(out, only)
				}
				continue
			}
			return nil, fmt.Errorf("fuente %d: %w: %w", i, err, ErrArtifactSourceUnavailable)
		}
		out = append(out, monitor)
	}
	return out, nil
}

// artifactMonitorIDs traduce los alias de las sources a los ObjectID de
// los monitores, para fijar la allowlist del artefacto al crearlo.
// Un alias que no resuelve se omite: no se puede fabricar acceso a un
// monitor que la conversación no tenía.
func artifactMonitorIDs(aliases map[string]*models.Monitor, sources []models.ArtifactSource) []primitive.ObjectID {
	seen := map[primitive.ObjectID]bool{}
	ids := []primitive.ObjectID{}
	for _, src := range sources {
		monitor, err := resolveQueryMonitor(aliases, src.Monitor)
		if err != nil {
			continue
		}
		if !seen[monitor.ID] {
			seen[monitor.ID] = true
			ids = append(ids, monitor.ID)
		}
	}
	return ids
}

// RunArtifact ejecuta las sources del artefacto y devuelve la tabla
// resultante. Es el ÚNICO camino por el que se obtienen datos de un
// artefacto: lo usan el render, la invocación y las exportaciones.
//
// NO persiste nada: solo el endpoint /run escribe la caché. Exportar
// re-ejecuta pero no muta el artefacto, para que bajar un Excel no le
// cambie la fecha a quien lo esté mirando en pantalla.
func (s *ArtifactService) RunArtifact(ctx context.Context, art *models.ChatArtifact, userID primitive.ObjectID) (ArtifactRun, error) {
	if art.UserID != userID {
		return ArtifactRun{}, ErrArtifactNotFound
	}
	if !art.IsRerunnable() {
		return ArtifactRun{}, ErrArtifactNotRerunnable
	}

	monitors := make([]models.Monitor, 0, len(art.MonitorIDs))
	for _, id := range art.MonitorIDs {
		monitor, err := s.monitorRepo.FindByID(ctx, id)
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Borrado (FindByID ya filtra deleted_at): exclusión
			// permanente y correcta de la allowlist.
			continue
		}
		if err != nil {
			// Cualquier otro error (deadline, blip del replica set, fallo
			// de decodificación) es transitorio y NO se traga — mismo
			// criterio que ChatService.Ask: con menos monitores en la
			// allowlist, un artefacto multi-fuente fallaría por "alias
			// desconocido" o —peor— mostraría una comparación incompleta
			// como si estuviera completa.
			return ArtifactRun{}, fmt.Errorf("cargando monitor %s: %w", id.Hex(), err)
		}
		monitors = append(monitors, *monitor)
	}
	aliases := buildMonitorAliases(monitors)

	resolved, err := resolveArtifactMonitors(aliases, art.Sources, len(art.MonitorIDs))
	if err != nil {
		return ArtifactRun{}, err
	}
	if len(resolved) != len(art.Sources) {
		// Invariante del emparejamiento resolved[i] <-> art.Sources[i]:
		// hoy es trivialmente cierto (un append por fuente o un return),
		// pero si alguna vez se agregara un camino que saltea una fuente,
		// el bucle de abajo leería el monitor de OTRA. Se corta antes de
		// tocar Mongo.
		return ArtifactRun{}, fmt.Errorf("resolviendo las fuentes: se resolvieron %d monitores para %d fuentes", len(resolved), len(art.Sources))
	}

	results := make([][]map[string]interface{}, 0, len(art.Sources))
	labels := make([]string, 0, len(art.Sources))
	for i, src := range art.Sources {
		rows, err := s.runSource(ctx, resolved[i], src.Query)
		if err != nil {
			return ArtifactRun{}, fmt.Errorf("ejecutando la fuente %d: %w", i, err)
		}
		results = append(results, rows)
		label := src.Label
		if label == "" {
			label = src.Monitor
		}
		labels = append(labels, label)
	}

	rows, series := s.shape(art, results, labels)
	return ArtifactRun{Data: rows, Series: series, RanAt: time.Now()}, nil
}

// shape aplica la presentación y devuelve, junto a las filas, las claves
// que hay que graficar.
//
// Con una sola fuente los datos van tal cual y las series son las YKeys
// que escribió el LLM. Con varias, un chart PIVOTEA —una serie por
// fuente, porque concatenar daría una sola serie con las x repetidas— y
// entonces las series pasan a ser los labels de las fuentes: el pivote
// renombró las columnas y YKeys ya no apunta a nada.
//
// Una tabla con varias fuentes CONCATENA, con su columna discriminadora.
func (s *ArtifactService) shape(
	art *models.ChatArtifact,
	results [][]map[string]interface{},
	labels []string,
) ([]map[string]interface{}, []string) {
	var yKeys []string
	if art.ChartSpec != nil {
		yKeys = art.ChartSpec.YKeys
	}

	if len(results) == 1 {
		rows := results[0]
		if art.ChartSpec != nil {
			rows = projectRows(rows, art.ChartSpec.Columns)
		}
		return rows, yKeys
	}

	if art.Type == models.ChatArtifactChart && art.ChartSpec != nil {
		valueKey := ""
		if len(yKeys) > 0 {
			valueKey = yKeys[0]
		}
		// Las series NO son los labels crudos: pivotSources desambigua los
		// repetidos (dos ventanas de tiempo del mismo monitor comparten
		// label) y el que colisiona con xKey, así que la columna que crea
		// puede llamarse "Bancolombia (2)". Se calcula el mismo nombre con
		// la misma función y los mismos argumentos — determinista — para
		// que Series apunte siempre a una columna que existe.
		series := pivotSourceLabels(labels, art.ChartSpec.XKey, len(results))
		// NO se proyecta después de pivotear: las columnas que el pivote
		// acaba de crear se llaman como los labels de las fuentes, y una
		// proyección escrita para las columnas originales las borraría
		// justo después de crearlas. Columns es solo para tablas.
		return pivotSources(results, labels, art.ChartSpec.XKey, valueKey), series
	}

	rows := concatSources(results, labels)
	if art.ChartSpec != nil {
		rows = projectRows(rows, art.ChartSpec.Columns)
	}
	return rows, yKeys
}

// runSource ejecuta una consulta contra el monitor YA RESUELTO. No vuelve
// a mirar src.Query.Monitor: la autorización pasó en
// resolveArtifactMonitors y este método no debe poder saltearla.
func (s *ArtifactService) runSource(ctx context.Context, monitor *models.Monitor, query models.ChatQueryInput) ([]map[string]interface{}, error) {
	if query.Aggregate != nil {
		pipeline := buildDataAggregatePipeline(*query.Aggregate)
		results, err := s.monitorRepo.AggregateData(ctx, monitor.CollectionID, pipeline)
		if err != nil {
			return nil, fmt.Errorf("ejecutando agregación: %w", err)
		}
		return bsonToMaps(results), nil
	}

	filter := bson.M{}
	if query.ConditionGroup != nil {
		filter = BuildMongoFilter(*query.ConditionGroup)
	}
	limit := query.Limit
	if limit <= 0 || limit > chatQueryMaxLimit {
		limit = chatQueryMaxLimit
	}
	results, err := s.monitorRepo.QueryData(ctx, monitor.CollectionID, filter, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("ejecutando consulta: %w", err)
	}
	return bsonToMaps(results), nil
}

func bsonToMaps(docs []bson.M) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		out = append(out, map[string]interface{}(doc))
	}
	return out
}
