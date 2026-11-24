package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite" // driver SQLite en Go puro (no necesita CGO)
)

// Producto es la entidad que gestiona el CRUD.
type Producto struct {
	ID       int       `json:"id"`
	Nombre   string    `json:"nombre"`
	Precio   float64   `json:"precio"`
	Stock    int       `json:"stock"`
	CreadoEn time.Time `json:"creado_en"`
	ActualEn time.Time `json:"actualizado_en"`
}

var ErrNoEncontrado = errors.New("producto no encontrado")

// Store accede a la tabla de productos en SQLite.
type Store struct {
	db *sql.DB
}

// NewStore abre (o crea) la base de datos en la ruta indicada y crea la tabla
// si no existe. Usar ":memory:" para una base temporal en memoria (tests).
func NewStore(ruta string) (*Store, error) {
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		return nil, err
	}
	// SQLite admite un solo escritor; con una conexión evitamos bloqueos
	// y la base ":memory:" se mantiene viva entre consultas.
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS productos (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			nombre         TEXT    NOT NULL,
			precio         REAL    NOT NULL,
			stock          INTEGER NOT NULL,
			creado_en      TEXT    NOT NULL,
			actualizado_en TEXT    NOT NULL
		)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Crear(ctx context.Context, p Producto) (Producto, error) {
	ahora := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO productos (nombre, precio, stock, creado_en, actualizado_en)
		 VALUES (?, ?, ?, ?, ?)`,
		p.Nombre, p.Precio, p.Stock, formatear(ahora), formatear(ahora))
	if err != nil {
		return Producto{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Producto{}, err
	}
	p.ID = int(id)
	p.CreadoEn = ahora
	p.ActualEn = ahora
	return p, nil
}

func (s *Store) Listar(ctx context.Context) ([]Producto, error) {
	filas, err := s.db.QueryContext(ctx,
		`SELECT id, nombre, precio, stock, creado_en, actualizado_en
		 FROM productos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	lista := []Producto{}
	for filas.Next() {
		p, err := escanear(filas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, p)
	}
	return lista, filas.Err()
}

func (s *Store) Obtener(ctx context.Context, id int) (Producto, error) {
	fila := s.db.QueryRowContext(ctx,
		`SELECT id, nombre, precio, stock, creado_en, actualizado_en
		 FROM productos WHERE id = ?`, id)
	p, err := escanear(fila)
	if errors.Is(err, sql.ErrNoRows) {
		return Producto{}, ErrNoEncontrado
	}
	return p, err
}

func (s *Store) Actualizar(ctx context.Context, id int, datos Producto) (Producto, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE productos SET nombre = ?, precio = ?, stock = ?, actualizado_en = ?
		 WHERE id = ?`,
		datos.Nombre, datos.Precio, datos.Stock, formatear(time.Now().UTC()), id)
	if err != nil {
		return Producto{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Producto{}, ErrNoEncontrado
	}
	return s.Obtener(ctx, id)
}

func (s *Store) Eliminar(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM productos WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoEncontrado
	}
	return nil
}

// --- helpers ---

// Las fechas se guardan como texto RFC 3339 para que sean legibles en la base.
func formatear(t time.Time) string { return t.Format(time.RFC3339Nano) }

// escaneable lo cumplen tanto *sql.Row como *sql.Rows.
type escaneable interface {
	Scan(dest ...any) error
}

func escanear(e escaneable) (Producto, error) {
	var p Producto
	var creado, actualizado string
	if err := e.Scan(&p.ID, &p.Nombre, &p.Precio, &p.Stock, &creado, &actualizado); err != nil {
		return Producto{}, err
	}
	var err error
	if p.CreadoEn, err = time.Parse(time.RFC3339Nano, creado); err != nil {
		return Producto{}, err
	}
	if p.ActualEn, err = time.Parse(time.RFC3339Nano, actualizado); err != nil {
		return Producto{}, err
	}
	return p, nil
}
