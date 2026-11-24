# CRUD de productos en Go

API REST para **C**rear, **L**eer (**R**ead), **A**ctualizar (**U**pdate) y **E**liminar (**D**elete) productos.
Los datos se guardan en una base **SQLite**, que es un único archivo (`productos.db`) y no necesita un servidor de base de datos aparte.

---

## Estructura del proyecto

```
crud-go/
├── go.mod             → definición del módulo y sus dependencias
├── go.sum             → (se genera solo) huellas de seguridad de las dependencias
├── main.go            → punto de entrada: arranca el servidor
├── handlers.go        → capa HTTP: rutas, validación y respuestas JSON
├── store.go           → capa de datos: modelo Producto y consultas SQL
├── handlers_test.go   → tests automáticos de toda la API
├── crud-go.postman_collection.json → colección de Postman con todos los endpoints
├── productos.db       → (se genera solo) la base de datos SQLite
├── crud.exe           → (opcional, se genera con go build) el servidor compilado
├── .gitignore         → archivos que no se suben a git (crud.exe, productos.db)
└── README.md          → este documento
```

El código está separado en **capas**. Cada archivo tiene una sola responsabilidad:

```
  Cliente (curl, navegador, Postman)
            │  petición HTTP
            ▼
  ┌─────────────────────┐
  │  main.go            │  arranca todo y registra cada petición en la consola
  └─────────┬───────────┘
            ▼
  ┌─────────────────────┐
  │  handlers.go        │  ¿qué ruta es? ¿los datos son válidos? → responde JSON
  └─────────┬───────────┘
            ▼
  ┌─────────────────────┐
  │  store.go           │  ejecuta el SQL contra la base
  └─────────┬───────────┘
            ▼
        productos.db
```

La ventaja de separar así es que si mañana querés cambiar SQLite por PostgreSQL, solo tocás `store.go`. Y si querés cambiar las rutas o el formato de las respuestas, solo tocás `handlers.go`.

---

## Qué hace cada archivo

### `go.mod`
Es la "ficha" del proyecto, como el `package.json` en Node.js. Indica:
- el **nombre del módulo** (`crud-go`),
- la **versión mínima de Go** (1.22, necesaria para las rutas del tipo `GET /productos/{id}`),
- las **dependencias externas**. La única es `modernc.org/sqlite`, el driver de SQLite. Se agrega sola al ejecutar `go mod tidy`.

### `go.sum`
Lo crea Go automáticamente al descargar las dependencias. Guarda una huella (hash) de cada una para garantizar que nadie las modificó. **No se edita a mano**, pero sí conviene subirlo al repositorio.

### `main.go`: el punto de entrada
Es lo primero que se ejecuta (función `main`). Hace cuatro cosas:
1. Lee la configuración de variables de entorno:
   - `PORT`: el puerto donde escucha el servidor (por defecto `8080`).
   - `DB_PATH`: la ruta del archivo de base de datos (por defecto `productos.db`).
2. Abre la base de datos con `NewStore(...)`.
3. Crea la `API` y le entrega el store.
4. Levanta el servidor HTTP.

También define el middleware `logger`, una función que envuelve a todas las rutas y escribe en consola cada petición que llega (por ejemplo `GET /productos`).

### `handlers.go`: la capa HTTP
Recibe las peticiones y decide qué responder. No sabe nada de SQL, solo le pide cosas al store.

**Rutas** (función `Rutas`):

| Método | Ruta              | Función      | Qué hace             | Respuesta OK |
|--------|-------------------|--------------|----------------------|--------------|
| GET    | `/productos`      | `listar`     | Devuelve todos       | 200          |
| POST   | `/productos`      | `crear`      | Crea uno nuevo       | 201          |
| GET    | `/productos/{id}` | `obtener`    | Devuelve uno         | 200          |
| PUT    | `/productos/{id}` | `actualizar` | Reemplaza sus datos  | 200          |
| DELETE | `/productos/{id}` | `eliminar`   | Lo borra             | 204          |

**Funciones auxiliares:**
- `leerID`: toma el `{id}` de la URL y verifica que sea un número positivo. Si no lo es, responde **400**.
- `leerProducto`: lee el JSON del cuerpo y lo valida: nombre obligatorio, precio y stock no negativos, sin campos desconocidos, máximo 1 MB. Si algo falla, responde **400** con el motivo.
- `manejarError`: traduce errores a códigos HTTP. "No encontrado" se convierte en **404** y cualquier otro error en **500**. El detalle de un 500 se registra en consola, no se le muestra al cliente.
- `responderJSON` y `responderError`: escriben la respuesta en formato JSON.

### `store.go`: la capa de datos
Todo lo relacionado con la base de datos.

- **`Producto`**: la estructura (struct) que representa un producto. Las etiquetas `json:"..."` indican cómo se llama cada campo en el JSON:
  ```json
  {
    "id": 1,
    "nombre": "Mate",
    "precio": 1500,
    "stock": 10,
    "creado_en": "2026-11-23T14:00:00Z",
    "actualizado_en": "2026-11-23T14:00:00Z"
  }
  ```
- **`ErrNoEncontrado`**: el error que se devuelve cuando el ID no existe. `handlers.go` lo usa para responder 404.
- **`NewStore(ruta)`**: abre la base y crea la tabla `productos` si todavía no existe. Con la ruta especial `":memory:"` la base vive solo en memoria, lo que sirve para los tests.
- **`Crear`, `Listar`, `Obtener`, `Actualizar`, `Eliminar`**: una función por operación del CRUD. Cada una ejecuta su sentencia SQL (`INSERT`, `SELECT`, `UPDATE`, `DELETE`). Todas usan `?` para los valores, lo que **protege contra inyección SQL**.
- **`formatear` y `escanear`**: auxiliares para convertir las fechas a texto y para pasar una fila de la base a un `Producto`.

Detalle: `db.SetMaxOpenConns(1)` limita a una sola conexión, porque SQLite permite un único escritor a la vez. Así se evitan errores de "database is locked".

### `handlers_test.go`: los tests
Prueban la API de punta a punta, como lo haría un cliente real, pero sin levantar el servidor a mano. Cada test usa una base en memoria nueva, así no se pisan entre sí ni dejan archivos.

- **`TestCRUDCompleto`**: crea, lista, obtiene, actualiza y elimina un producto, y verifica cada respuesta.
- **`TestNoEncontrado`**: comprueba que pedir, actualizar o borrar un ID inexistente devuelve 404.
- **`TestValidaciones`**: manda datos inválidos (nombre vacío, precio negativo, JSON roto, campo extra, id no numérico) y espera 400.

### `crud-go.postman_collection.json`: colección de Postman
Archivo que **Postman** puede importar. Trae ya armadas todas las peticiones para probar la API, así no hay que cargarlas a mano.

**Sin el archivo**, para cada endpoint habría que crear la petición en Postman y completar el método (POST, GET, PUT o DELETE), la URL, el header `Content-Type: application/json` y el cuerpo en JSON. Y eso una vez por cada endpoint.

**Con el archivo**, se importa una sola vez y aparecen 7 peticiones listas, agrupadas en la carpeta **CRUD Productos (Go)**:
- las 5 operaciones del CRUD: crear, listar, obtener, actualizar y eliminar;
- 2 ejemplos de error: datos inválidos (400) y producto no encontrado (404).

Además trae dos cosas que ahorran trabajo:
1. **Variables:** `{{baseUrl}}` guarda la dirección del servidor y `{{id}}` el producto con el que se está trabajando. Si cambia el puerto, se edita en un solo lugar.
2. **Un script en "Crear producto":** después de crear un producto, toma el `id` que devuelve la API y lo guarda en `{{id}}`. Así **Obtener**, **Actualizar** y **Eliminar** usan ese producto sin escribir el número.

**Otras ventajas:**
- **Compartir:** cualquiera que trabaje en el proyecto importa el archivo y ya puede probar la API.
- **Queda junto al código:** como está en la carpeta del proyecto, si se agrega un endpoint se puede sumar también a la colección.

**Lo que no hace:** no forma parte del backend. Go no lo usa y el servidor funciona igual aunque se borre. Es solo una herramienta para probar la API. Cómo importarlo está en [Probar con Postman](#6-probar-con-postman-alternativa-a-curl).

### `productos.db`
El archivo de la base de datos. Se crea solo la primera vez que arrancás el servidor. Si lo borrás, empezás con la base vacía. Se puede abrir con herramientas como [DB Browser for SQLite](https://sqlitebrowser.org/).

---

## Cómo usarlo

### 1. Instalar Go
Descargalo de https://go.dev/dl/ o, en Windows:
```powershell
winget install GoLang.Go
```
Cerrá y volvé a abrir la terminal después de instalarlo.

### 2. Descargar dependencias (solo la primera vez)
```powershell
cd crud-go
go mod tidy
```

### 3. Ejecutar los tests
```powershell
go test -v
```

### 4. Levantar el servidor
```powershell
go run .
```

### 5. Probar la API (desde otra terminal)
```powershell
# Crear
curl.exe -X POST localhost:8080/productos -H "Content-Type: application/json" -d '{\"nombre\":\"Mate\",\"precio\":1500,\"stock\":10}'

# Listar todos
curl.exe localhost:8080/productos

# Obtener uno
curl.exe localhost:8080/productos/1

# Actualizar
curl.exe -X PUT localhost:8080/productos/1 -d '{\"nombre\":\"Mate imperial\",\"precio\":2500,\"stock\":5}'

# Eliminar
curl.exe -X DELETE localhost:8080/productos/1
```

### 6. Probar con Postman (alternativa a curl)
El proyecto incluye la colección `crud-go.postman_collection.json` con todos los endpoints ya configurados.

1. Con el servidor levantado (`go run .`), abrí Postman.
2. Hacé clic en **Import** y arrastrá el archivo `crud-go.postman_collection.json`.
3. Aparece la colección **CRUD Productos (Go)**. Ejecutá las peticiones en este orden:
   1. **Crear producto**: guarda automáticamente el `id` devuelto en la variable `{{id}}`.
   2. **Listar productos**
   3. **Obtener producto**: usa `{{id}}`.
   4. **Actualizar producto**
   5. **Eliminar producto**
   6. **Error: datos inválidos (400)** y **Error: no encontrado (404)**: muestran cómo responde la API ante errores.

Variables de la colección (pestaña *Variables*):
- `baseUrl`: `http://localhost:8080`. Cambiala si usás otro `PORT`.
- `id`: el producto sobre el que actúan Obtener, Actualizar y Eliminar.

También podés correr todas juntas con **Run collection** (botón *Run* en la colección).

**Si preferís cargarlas a mano en Postman:**

| Método | URL                                   | Body (raw → JSON)                                     |
|--------|---------------------------------------|-------------------------------------------------------|
| POST   | `http://localhost:8080/productos`     | `{"nombre": "Mate", "precio": 1500, "stock": 10}`     |
| GET    | `http://localhost:8080/productos`     | (ninguno)                                             |
| GET    | `http://localhost:8080/productos/1`   | (ninguno)                                             |
| PUT    | `http://localhost:8080/productos/1`   | `{"nombre": "Mate imperial", "precio": 2500, "stock": 5}` |
| DELETE | `http://localhost:8080/productos/1`   | (ninguno)                                             |

En los POST y PUT, en la pestaña **Body** elegí **raw** y el tipo **JSON**. El PUT reemplaza el producto completo, así que hay que enviar los tres campos.

### Compilar un ejecutable

**Cómo compilarlo, paso a paso:**

1. Abrí una terminal en la carpeta del proyecto:
   ```powershell
   cd crud-go
   ```
2. Si es la primera vez, descargá las dependencias:
   ```powershell
   go mod tidy
   ```
3. (Recomendado) Comprobá que los tests pasan, así no compilás algo roto:
   ```powershell
   go test
   ```
4. Compilá:
   ```powershell
   go build -o crud.exe .
   ```
   - `go build` compila el programa.
   - `-o crud.exe` indica el nombre del archivo que se genera. Sin `-o`, se llamaría como el módulo (`crud-go.exe`).
   - `.` significa "compilar el paquete de esta carpeta" (todos los `.go` menos los `_test.go`).

   Si todo sale bien, **no muestra nada** y aparece `crud.exe` en la carpeta. Si hay un error en el código, lo muestra con el archivo y la línea, y no genera el `.exe`.
5. Ejecutalo:
   ```powershell
   .\crud.exe
   ```
   Deberías ver `Servidor escuchando en http://localhost:8080`. Se detiene con `Ctrl+C`.

**Ejecutable más chico (opcional):** quitando la información de depuración el archivo pesa bastante menos. Sirve para distribuirlo; para desarrollar no hace falta.

```powershell
go build -ldflags "-s -w" -o crud.exe .
```

**Errores frecuentes al compilar:**

| Error | Solución |
|-------|----------|
| `go : El término 'go' no se reconoce...` | Go no está instalado o la terminal no ve el `PATH`. Reabrí la terminal |
| `missing go.sum entry...` o `no required module provides package` | Ejecutá `go mod tidy` |
| `go.mod requires go >= 1.26.0` | Tu Go es más viejo que el que pide `go.mod`. Actualizalo desde https://go.dev/dl/ |
| `Acceso denegado` / `The process cannot access the file` | `crud.exe` está corriendo. Detenelo con `Ctrl+C` y volvé a compilar |

**Qué es `crud.exe`:** el servidor ya compilado, en un solo archivo. Incluye todo el programa y el driver de SQLite, así que no necesita nada más para funcionar.

**Para qué sirve:**
- **Arrancar más rápido:** `.\crud.exe` hace lo mismo que `go run .`, pero no compila cada vez.
- **Usarlo en otra PC con Windows sin instalar Go:** alcanza con copiar el archivo y ejecutarlo.
- **Configurarlo igual que con `go run`:** acepta las mismas variables `PORT` y `DB_PATH`. Si no indicás `DB_PATH`, crea `productos.db` en la carpeta desde donde lo ejecutás.

  ```powershell
  $env:PORT = "9090"; $env:DB_PATH = "C:\datos\productos.db"; .\crud.exe
  ```

**Tené en cuenta:**
- **No se actualiza solo.** Si cambiás el código, el `.exe` sigue teniendo la versión anterior hasta que lo vuelvas a compilar con `go build -o crud.exe .`.
- **Solo funciona en Windows.** Para Linux o Mac hay que compilarlo para ese sistema con las variables `GOOS` y `GOARCH`. Funciona sin configurar nada más porque el driver de SQLite está escrito en Go puro:

  ```powershell
  # Linux (por ejemplo, un servidor)
  $env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -o crud .

  # Mac con chip Apple (M1, M2...)
  $env:GOOS = "darwin"; $env:GOARCH = "arm64"; go build -o crud .

  # Volver a compilar para Windows
  Remove-Item Env:GOOS, Env:GOARCH
  ```

- **No hace falta para desarrollar:** `go run .` compila y ejecuta en un solo paso.
- **No se sube a git:** es un archivo generado que se puede recrear en cualquier momento, por eso está en el `.gitignore`. Se puede borrar sin problema.

---

## Códigos de respuesta

| Código | Significado                                    |
|--------|------------------------------------------------|
| 200    | OK                                             |
| 201    | Creado                                         |
| 204    | Eliminado (sin contenido en la respuesta)      |
| 400    | Datos inválidos (el JSON explica el motivo)    |
| 404    | El producto no existe                          |
| 500    | Error interno (el detalle aparece en consola)  |

Los errores siempre tienen la forma:
```json
{ "error": "el nombre es obligatorio" }
```
