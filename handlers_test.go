package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// nuevoServidor levanta la API sobre una base SQLite en memoria,
// así cada test arranca con la base vacía y no deja archivos.
func nuevoServidor(t *testing.T) *httptest.Server {
	t.Helper()
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	srv := httptest.NewServer((&API{store: store}).Rutas())
	t.Cleanup(func() {
		srv.Close()
		store.Close()
	})
	return srv
}

// pedir hace una petición HTTP y, si se pasa destino, decodifica el JSON de respuesta.
func pedir(t *testing.T, metodo, url, cuerpo string, destino any) int {
	t.Helper()
	req, err := http.NewRequest(metodo, url, bytes.NewBufferString(cuerpo))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if destino != nil {
		if err := json.NewDecoder(res.Body).Decode(destino); err != nil {
			t.Fatalf("decodificar respuesta: %v", err)
		}
	}
	return res.StatusCode
}

func TestCRUDCompleto(t *testing.T) {
	srv := nuevoServidor(t)
	base := srv.URL + "/productos"

	// Crear
	var creado Producto
	if st := pedir(t, "POST", base, `{"nombre":"Mate","precio":1500,"stock":10}`, &creado); st != http.StatusCreated {
		t.Fatalf("crear: status %d", st)
	}
	if creado.ID != 1 || creado.Nombre != "Mate" || creado.Precio != 1500 || creado.Stock != 10 {
		t.Fatalf("crear: producto inesperado %+v", creado)
	}

	// Listar
	var lista []Producto
	if st := pedir(t, "GET", base, "", &lista); st != http.StatusOK || len(lista) != 1 {
		t.Fatalf("listar: status %d, %d productos", st, len(lista))
	}

	// Obtener
	var obtenido Producto
	if st := pedir(t, "GET", base+"/1", "", &obtenido); st != http.StatusOK || obtenido.Nombre != "Mate" {
		t.Fatalf("obtener: status %d, %+v", st, obtenido)
	}

	// Actualizar
	var actualizado Producto
	if st := pedir(t, "PUT", base+"/1", `{"nombre":"Mate imperial","precio":2500,"stock":5}`, &actualizado); st != http.StatusOK {
		t.Fatalf("actualizar: status %d", st)
	}
	if actualizado.Nombre != "Mate imperial" || actualizado.Precio != 2500 || actualizado.Stock != 5 {
		t.Fatalf("actualizar: producto inesperado %+v", actualizado)
	}
	if !actualizado.CreadoEn.Equal(creado.CreadoEn) {
		t.Fatalf("actualizar no debe cambiar creado_en")
	}

	// Eliminar
	if st := pedir(t, "DELETE", base+"/1", "", nil); st != http.StatusNoContent {
		t.Fatalf("eliminar: status %d", st)
	}
	if st := pedir(t, "GET", base+"/1", "", nil); st != http.StatusNotFound {
		t.Fatalf("obtener tras eliminar: status %d, se esperaba 404", st)
	}
}

func TestNoEncontrado(t *testing.T) {
	srv := nuevoServidor(t)
	base := srv.URL + "/productos"

	casos := []struct{ metodo, cuerpo string }{
		{"GET", ""},
		{"PUT", `{"nombre":"X","precio":1,"stock":1}`},
		{"DELETE", ""},
	}
	for _, c := range casos {
		if st := pedir(t, c.metodo, base+"/99", c.cuerpo, nil); st != http.StatusNotFound {
			t.Errorf("%s /productos/99: status %d, se esperaba 404", c.metodo, st)
		}
	}
}

func TestValidaciones(t *testing.T) {
	srv := nuevoServidor(t)
	base := srv.URL + "/productos"

	casos := map[string]string{
		"nombre vacío":    `{"nombre":"  ","precio":1,"stock":1}`,
		"precio negativo": `{"nombre":"X","precio":-1,"stock":1}`,
		"stock negativo":  `{"nombre":"X","precio":1,"stock":-1}`,
		"JSON roto":       `{"nombre":`,
		"campo extra":     `{"nombre":"X","precio":1,"stock":1,"color":"rojo"}`,
	}
	for nombre, cuerpo := range casos {
		if st := pedir(t, "POST", base, cuerpo, nil); st != http.StatusBadRequest {
			t.Errorf("%s: status %d, se esperaba 400", nombre, st)
		}
	}

	if st := pedir(t, "GET", base+"/abc", "", nil); st != http.StatusBadRequest {
		t.Errorf("id no numérico: status %d, se esperaba 400", st)
	}
}
