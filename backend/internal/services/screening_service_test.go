package services

import (
	"reflect"
	"testing"
)

func TestExtractScreeningValues_TomaSoloLosCamposConfigurados(t *testing.T) {
	data := map[string]interface{}{
		"account":     "ACC-1",
		"counterpart": "Juan Perez",
		"amount":      500.0,
	}
	got := extractScreeningValues(data, []string{"counterpart"})
	want := map[string]string{"counterpart": "Juan Perez"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractScreeningValues_CampoAusenteOVacioSeOmite(t *testing.T) {
	data := map[string]interface{}{"counterpart": "", "other": "x"}
	got := extractScreeningValues(data, []string{"counterpart", "no_existe"})
	if len(got) != 0 {
		t.Errorf("got %v, want vacío (campo ausente o vacío no cuenta)", got)
	}
}

func TestExtractScreeningValues_ValorNoStringSeOmite(t *testing.T) {
	data := map[string]interface{}{"amount": 500.0}
	got := extractScreeningValues(data, []string{"amount"})
	if len(got) != 0 {
		t.Errorf("got %v, want vacío (un monto no es un nombre para screenear)", got)
	}
}

func TestExtractScreeningValues_VariosCampos(t *testing.T) {
	data := map[string]interface{}{
		"ordenante":   "Ana Gomez",
		"beneficiario": "Juan Perez",
	}
	got := extractScreeningValues(data, []string{"ordenante", "beneficiario"})
	want := map[string]string{"ordenante": "Ana Gomez", "beneficiario": "Juan Perez"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
