package services

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/thureos/compliance/internal/models"
)

// aiModelsTimeout acota la consulta del catálogo. Es corta a propósito:
// esto corre mientras un administrador espera a que cargue la pantalla de
// Configuración, y un proveedor lento no puede dejarla colgada. Si no
// contesta a tiempo se cae a la lista estática, que siempre sirve algo.
const aiModelsTimeout = 10 * time.Second

// providerModelList es la forma que devuelven los endpoints /models de las
// APIs compatibles con OpenAI, que es el caso de DeepSeek.
type providerModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListProviderModels devuelve los modelos que el proveedor ofrece AHORA,
// preguntándoselo a su API, y si eso falla los de la lista compilada.
//
// Existe por una caída real: la lista estaba escrita en el código, DeepSeek
// retiró deepseek-chat y deepseek-reasoner, y la aplicación siguió pidiendo
// un modelo inexistente hasta que alguien lo notó desde el chat. Una lista
// que hay que acordarse de actualizar a mano es una lista que va a quedar
// vieja; preguntar es lo único que no caduca.
//
// El segundo valor dice si la lista vino del proveedor. La pantalla lo usa
// para no presentar como catálogo vivo lo que en realidad es el respaldo.
func ListProviderModels(ctx context.Context, ai models.AIConfig) ([]string, bool) {
	fallback := models.AIProviderModels[ai.Provider]

	// Sin clave no hay a quién preguntar: el endpoint de catálogo también
	// pide autenticación. No es un error, es una instalación a medio
	// configurar, y el respaldo es la respuesta correcta.
	if ai.APIKey == "" {
		return fallback, false
	}

	// Sólo DeepSeek por ahora. Anthropic tiene su propio endpoint de
	// catálogo con otra forma de autenticación; mientras no se cablee,
	// conserva su lista estática en vez de quedarse sin opciones.
	if ai.Provider != models.AIProviderDeepSeek {
		return fallback, false
	}

	baseURL := ai.BaseURL
	if baseURL == "" {
		baseURL = models.DefaultAIBaseURLs[ai.Provider]
	}

	ctx, cancel := context.WithTimeout(ctx, aiModelsTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return fallback, false
	}
	req.Header.Set("Authorization", "Bearer "+ai.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fallback, false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fallback, false
	}

	var parsed providerModelList
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fallback, false
	}

	ids := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	// Un catálogo vacío no se distingue de una respuesta rota, y dejar el
	// desplegable sin opciones impediría guardar nada. Mejor el respaldo.
	if len(ids) == 0 {
		return fallback, false
	}

	sort.Strings(ids)
	return ids, true
}

// ModelAllowedForProvider decide si un modelo se puede guardar.
//
// Acepta la unión de tres cosas, y cada una está por un motivo: el catálogo
// vivo, porque es la verdad; la lista compilada, para que un proveedor caído
// no bloquee un cambio legítimo; y el modelo que YA está guardado, para que
// un administrador siempre pueda volver a grabar su propia configuración sin
// quedar encerrado fuera por una lista que hoy no lo incluye.
func ModelAllowedForProvider(live []string, provider models.AIProvider, current, candidate string) bool {
	if candidate == "" {
		return false
	}
	for _, m := range live {
		if m == candidate {
			return true
		}
	}
	for _, m := range models.AIProviderModels[provider] {
		if m == candidate {
			return true
		}
	}
	return candidate == current
}
