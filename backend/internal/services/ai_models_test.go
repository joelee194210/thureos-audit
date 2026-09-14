package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func deepseekCfg(baseURL, key string) models.AIConfig {
	return models.AIConfig{Provider: models.AIProviderDeepSeek, APIKey: key, BaseURL: baseURL}
}

func TestListProviderModels_DevuelveElCatalogoDelProveedor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("pidió %s, se esperaba /models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"deepseek-v4-pro"},{"id":"deepseek-flash"}]}`))
	}))
	defer srv.Close()

	got, live := ListProviderModels(context.Background(), deepseekCfg(srv.URL, "k"))
	if !live {
		t.Fatal("debería marcarse como catálogo vivo")
	}
	want := []string{"deepseek-flash", "deepseek-v4-pro"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("modelos = %v, se esperaba %v", got, want)
	}
}

func TestListProviderModels_SinClaveNoPregunta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llamar al proveedor sin clave")
	}))
	defer srv.Close()

	got, live := ListProviderModels(context.Background(), deepseekCfg(srv.URL, ""))
	if live {
		t.Error("sin clave no hay catálogo vivo")
	}
	if len(got) == 0 {
		t.Error("debe caer al respaldo, no dejar el desplegable vacío")
	}
}

func TestListProviderModels_ProveedorCaidoCaeAlRespaldo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	got, live := ListProviderModels(context.Background(), deepseekCfg(srv.URL, "k"))
	if live {
		t.Error("un 500 no es un catálogo")
	}
	if len(got) != len(models.AIProviderModels[models.AIProviderDeepSeek]) {
		t.Fatalf("respaldo = %v", got)
	}
}

func TestListProviderModels_CatalogoVacioCaeAlRespaldo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer srv.Close()

	got, live := ListProviderModels(context.Background(), deepseekCfg(srv.URL, "k"))
	if live || len(got) == 0 {
		t.Fatalf("got=%v live=%v; se esperaba respaldo", got, live)
	}
}

func TestListProviderModels_RespuestaNoJSONCaeAlRespaldo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>portal cautivo</html>"))
	}))
	defer srv.Close()

	if _, live := ListProviderModels(context.Background(), deepseekCfg(srv.URL, "k")); live {
		t.Error("HTML no es un catálogo")
	}
}

func TestModelAllowedForProvider_AceptaElCatalogoVivo(t *testing.T) {
	if !ModelAllowedForProvider([]string{"deepseek-v5-nuevo"}, models.AIProviderDeepSeek, "deepseek-flash", "deepseek-v5-nuevo") {
		t.Error("un modelo del catálogo vivo debe aceptarse aunque no esté compilado")
	}
}

func TestModelAllowedForProvider_AceptaElRespaldoSiElProveedorNoContesta(t *testing.T) {
	if !ModelAllowedForProvider(nil, models.AIProviderDeepSeek, "", "deepseek-flash") {
		t.Error("con el proveedor caído debe valer la lista compilada")
	}
}

func TestModelAllowedForProvider_SiempreSePuedeRegrabarElActual(t *testing.T) {
	if !ModelAllowedForProvider([]string{"deepseek-flash"}, models.AIProviderDeepSeek, "modelo-retirado", "modelo-retirado") {
		t.Error("el modelo ya guardado debe poder regrabarse")
	}
}

func TestModelAllowedForProvider_RechazaLoDesconocido(t *testing.T) {
	if ModelAllowedForProvider([]string{"deepseek-flash"}, models.AIProviderDeepSeek, "deepseek-flash", "gpt-inventado") {
		t.Error("un modelo que nadie ofrece no debe aceptarse")
	}
	if ModelAllowedForProvider([]string{"deepseek-flash"}, models.AIProviderDeepSeek, "deepseek-flash", "") {
		t.Error("vacío no es un modelo")
	}
}
