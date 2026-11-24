package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type API struct {
	store *Store
}

func (a *API) Rutas() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /productos", a.listar)
	mux.HandleFunc("POST /productos", a.crear)
	mux.HandleFunc("GET /productos/{id}", a.obtener)
	mux.HandleFunc("PUT /productos/{id}", a.actualizar)
	mux.HandleFunc("DELETE /productos/{id}", a.eliminar)
	return mux
}

func (a *API) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := a.store.Listar(r.Context())
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, lista)
}

func (a *API) crear(w http.ResponseWriter, r *http.Request) {
	datos, ok := leerProducto(w, r)
	if !ok {
		return
	}
	p, err := a.store.Crear(r.Context(), datos)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, p)
}

func (a *API) obtener(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}
	p, err := a.store.Obtener(r.Context(), id)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, p)
}

func (a *API) actualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}
	datos, ok := leerProducto(w, r)
	if !ok {
		return
	}
	p, err := a.store.Actualizar(r.Context(), id, datos)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, p)
}

func (a *API) eliminar(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}
	if err := a.store.Eliminar(r.Context(), id); err != nil {
		manejarError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func leerID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		responderError(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func leerProducto(w http.ResponseWriter, r *http.Request) (Producto, bool) {
	var p Producto
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return p, false
	}
	p.Nombre = strings.TrimSpace(p.Nombre)
	switch {
	case p.Nombre == "":
		responderError(w, http.StatusBadRequest, "el nombre es obligatorio")
		return p, false
	case p.Precio < 0:
		responderError(w, http.StatusBadRequest, "el precio no puede ser negativo")
		return p, false
	case p.Stock < 0:
		responderError(w, http.StatusBadRequest, "el stock no puede ser negativo")
		return p, false
	}
	return p, true
}

func manejarError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNoEncontrado) {
		responderError(w, http.StatusNotFound, err.Error())
		return
	}
	log.Printf("error interno: %v", err)
	responderError(w, http.StatusInternalServerError, "error interno")
}

func responderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func responderError(w http.ResponseWriter, status int, msg string) {
	responderJSON(w, status, map[string]string{"error": msg})
}
