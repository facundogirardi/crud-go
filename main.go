package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}
	rutaDB := os.Getenv("DB_PATH")
	if rutaDB == "" {
		rutaDB = "productos.db"
	}

	store, err := NewStore(rutaDB)
	if err != nil {
		log.Fatalf("no se pudo abrir la base de datos: %v", err)
	}
	defer store.Close()

	api := &API{store: store}

	log.Printf("Base de datos: %s", rutaDB)
	log.Printf("Servidor escuchando en http://localhost:%s", puerto)
	if err := http.ListenAndServe(":"+puerto, logger(api.Rutas())); err != nil {
		log.Fatal(err)
	}
}

// logger registra cada petición que llega.
func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
