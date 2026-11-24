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
- la **versión mínima de Go** (`go 1.26.0`). Lo importante es tener Go 1.22 o más nuevo, porque desde esa versión existen las rutas que incluyen el método y parámetros, como `GET /productos/{id}`,
- las **dependencias externas**. La única que usamos directamente es `modernc.org/sqlite`, el driver de SQLite. Se agrega sola al ejecutar `go mod tidy`.

Las líneas marcadas `// indirect` son dependencias **de nuestras dependencias**: el driver de SQLite las necesita, nosotros no las usamos. Go las anota para que todos compilen con exactamente las mismas versiones.

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

**Conceptos de Go que aparecen acá:**
- **`package main` y `func main()`**: un programa ejecutable en Go siempre tiene un paquete `main` con una función `main`. Todos los `.go` de la carpeta son del mismo paquete, por eso `main.go` puede usar `NewStore` (de `store.go`) y `API` (de `handlers.go`) sin importar nada.
- **Errores como valores**: Go no tiene excepciones. Las funciones que pueden fallar devuelven un `error` como último valor y se revisa con `if err != nil`. Por ejemplo, `store, err := NewStore(rutaDB)`.
- **`log.Fatalf`**: escribe el mensaje y termina el programa. Se usa cuando no tiene sentido seguir, por ejemplo si no se puede abrir la base.
- **`defer store.Close()`**: deja anotado que `Close` se ejecute cuando termine `main`. Así la base se cierra siempre, aunque el programa salga por un error.
- **Middleware**: `logger` recibe un `http.Handler` (las rutas) y devuelve otro `http.Handler` que primero escribe el log y después llama al original con `next.ServeHTTP(w, r)`. Con el mismo patrón se pueden agregar autenticación, CORS, medición de tiempos, etc.
- **`http.ListenAndServe`**: deja el programa escuchando en el puerto. No termina mientras el servidor esté funcionando; si devuelve algo, es porque hubo un error (por ejemplo, el puerto ya está en uso).

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

**Recorrido de una petición, paso a paso.** Qué pasa cuando llega `POST /productos` con `{"nombre":"Mate","precio":1500,"stock":10}`:

1. `logger` (en `main.go`) escribe `POST /productos` en la consola.
2. El `ServeMux` busca qué ruta coincide con el método y la URL, y llama a `a.crear`.
3. `crear` llama a `leerProducto`, que convierte el JSON en un `Producto`, le quita los espacios al nombre (`strings.TrimSpace`) y lo valida. Si algo está mal, ya respondió 400 y devuelve `ok = false`, así que `crear` termina con `return`.
4. Si los datos son válidos, `crear` llama a `a.store.Crear(...)`, que hace el `INSERT` y devuelve el producto con su `id` y fechas.
5. `responderJSON` pone el header `Content-Type: application/json`, el código **201** y escribe el producto en JSON.

**Conceptos de Go que aparecen acá:**
- **Métodos**: `func (a *API) crear(...)` es una función que pertenece al tipo `API`. Por eso adentro se puede usar `a.store`.
- **Firma de un handler**: todas las funciones de ruta reciben `w http.ResponseWriter` (donde se escribe la respuesta) y `r *http.Request` (la petición: método, URL, headers, cuerpo).
- **Patrón `(valor, ok)`**: `leerID` y `leerProducto` devuelven el dato y un `bool`. Si `ok` es `false`, el error ya se le respondió al cliente y el handler solo tiene que salir. Así no se repite el código de error en cada handler.
- **`r.PathValue("id")`**: obtiene la parte `{id}` de la URL. Siempre es texto; `strconv.Atoi` lo convierte a número.
- **`r.Context()`**: se le pasa al store. Si el cliente cancela la petición (cierra la conexión), la consulta a la base también se cancela.
- **`errors.Is(err, ErrNoEncontrado)`**: compara el error con uno conocido. Es la forma de Go de decir "si es este error en particular, hacé tal cosa".

**PUT reemplaza todo:** como `actualizar` usa la misma validación que `crear`, hay que mandar `nombre`, `precio` y `stock`. Si falta el nombre, responde 400; si faltan `precio` o `stock`, se guardan en `0` (en Go, un campo que no viene en el JSON queda con su "valor cero").

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

Detalle: `db.SetMaxOpenConns(1)` limita a una sola conexión, porque SQLite permite un único escritor a la vez. Así se evitan errores de "database is locked". Además, con `":memory:"` cada conexión nueva crearía otra base vacía; con una sola conexión, la base en memoria se mantiene entre consultas.

**Cómo funciona cada operación:**

| Función | SQL | Cómo se entera de que el producto no existe |
|---------|-----|---------------------------------------------|
| `Crear` | `INSERT` | No aplica. Usa `LastInsertId()` para saber qué `id` le asignó la base |
| `Listar` | `SELECT ... ORDER BY id` | No aplica. Si no hay productos devuelve `[]` (lista vacía), no `null` |
| `Obtener` | `SELECT ... WHERE id = ?` | La base devuelve `sql.ErrNoRows` y se traduce a `ErrNoEncontrado` |
| `Actualizar` | `UPDATE ... WHERE id = ?` | `RowsAffected()` da 0 filas modificadas. Después llama a `Obtener` para devolver el producto como quedó guardado |
| `Eliminar` | `DELETE ... WHERE id = ?` | `RowsAffected()` da 0 filas borradas |

**Conceptos de Go que aparecen acá:**
- **El import con `_`**: `_ "modernc.org/sqlite"` importa el paquete solo para que se registre como driver. Nunca lo usamos por nombre: hablamos con la base a través de `database/sql`, que es la librería estándar y funciona igual con cualquier base.
- **Structs y etiquetas**: `Producto` agrupa los campos. Las etiquetas `` `json:"nombre"` `` le dicen a `encoding/json` cómo se llama cada campo en el JSON (en Go los campos van con mayúscula para ser visibles desde otros paquetes, pero en el JSON queremos minúsculas).
- **Interfaces**: `escaneable` es una interfaz con un solo método, `Scan`. Tanto `*sql.Row` (una fila, en `Obtener`) como `*sql.Rows` (varias filas, en `Listar`) lo tienen, así que `escanear` sirve para los dos. En Go no hace falta declarar que un tipo "implementa" una interfaz: alcanza con que tenga los métodos.
- **Fechas**: se guardan como texto en formato RFC 3339 (`2026-11-23T14:00:00Z`), en UTC, para que se lean bien si abrís la base. `escanear` las vuelve a convertir a `time.Time`. `Actualizar` cambia `actualizado_en` pero nunca toca `creado_en`.

### `handlers_test.go`: los tests
Prueban la API de punta a punta, como lo haría un cliente real, pero sin levantar el servidor a mano. Cada test usa una base en memoria nueva, así no se pisan entre sí ni dejan archivos.

- **`TestCRUDCompleto`**: crea, lista, obtiene, actualiza y elimina un producto, y verifica cada respuesta.
- **`TestNoEncontrado`**: comprueba que pedir, actualizar o borrar un ID inexistente devuelve 404.
- **`TestValidaciones`**: manda datos inválidos (nombre vacío, precio negativo, JSON roto, campo extra, id no numérico) y espera 400.

Cómo funcionan por dentro está explicado en [Cómo funcionan los tests](#cómo-funcionan-los-tests).

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
Tienen que aparecer los tres tests con `PASS`. Más opciones en [Cómo funcionan los tests](#cómo-funcionan-los-tests).

### 4. Levantar el servidor
```powershell
go run .
```

### 5. Probar la API (desde otra terminal)
Estos comandos son para **PowerShell 7.3 o más nuevo** (`pwsh`, el que suele abrir VS Code). Para ver qué versión tenés: `$PSVersionTable.PSVersion`.

```powershell
# Crear
curl.exe -X POST localhost:8080/productos -H "Content-Type: application/json" -d '{"nombre":"Mate","precio":1500,"stock":10}'

# Listar todos
curl.exe localhost:8080/productos

# Obtener uno
curl.exe localhost:8080/productos/1

# Actualizar
curl.exe -X PUT localhost:8080/productos/1 -H "Content-Type: application/json" -d '{"nombre":"Mate imperial","precio":2500,"stock":5}'

# Eliminar
curl.exe -X DELETE localhost:8080/productos/1
```

> **¿Por qué `curl.exe` y no `curl`?** En Windows PowerShell 5.1, `curl` es un alias de `Invoke-WebRequest`, que usa otros parámetros. Escribiendo `curl.exe` te asegurás de usar el curl real.

**Si usás Windows PowerShell 5.1** (el `powershell.exe` que viene con Windows), esa versión les quita las comillas dobles a los argumentos que les pasa a programas externos como `curl.exe`, así que hay que escaparlas con `\"`:
```powershell
curl.exe -X POST localhost:8080/productos -H "Content-Type: application/json" -d '{\"nombre\":\"Mate\",\"precio\":1500,\"stock\":10}'
```
⚠️ En PowerShell 7.3+ **no** uses `\"`: las barras le llegan al servidor y responde
`{"error":"JSON inválido: invalid character '\\' looking for beginning of object key string"}`.

**Alternativas que funcionan en cualquier versión:**
```powershell
# Con el cmdlet nativo de PowerShell (no tiene problemas de comillas)
Invoke-RestMethod -Method Post -Uri localhost:8080/productos -ContentType "application/json" -Body '{"nombre":"Mate","precio":1500,"stock":10}'

# Con el JSON guardado en un archivo producto.json
curl.exe -X POST localhost:8080/productos -H "Content-Type: application/json" --data-binary "@producto.json"
```
En Git Bash, Linux o Mac no hay ningún problema: se usa el comando sin barras y con `curl` a secas.

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

## Cómo funcionan los tests

Go trae su propio sistema de tests, no hay que instalar nada. Lo que hace falta saber:

### Las reglas de Go para los tests
- Los tests van en archivos que terminan en **`_test.go`**. `go build` los ignora: no forman parte de `crud.exe`.
- Cada test es una función que empieza con **`Test`** y recibe `t *testing.T`, por ejemplo `func TestValidaciones(t *testing.T)`. `go test` las encuentra solas por el nombre.
- `handlers_test.go` está en el mismo paquete (`package main`), así que puede usar todo lo del proyecto: `NewStore`, `API`, `Producto`, etc.
- Un test **pasa** si termina sin reportar errores. Para marcarlo como fallido se usa:
  - **`t.Errorf(...)`**: anota el error y **sigue** ejecutando. Sirve cuando se revisan varios casos y conviene ver todos los que fallan.
  - **`t.Fatalf(...)`**: anota el error y **corta** ese test. Sirve cuando seguir no tiene sentido (si no se pudo crear el producto, no se puede probar actualizarlo).

### Las dos funciones de apoyo
**`nuevoServidor(t)`** prepara un servidor limpio para cada test:
1. Crea un store con `NewStore(":memory:")`: una base SQLite que vive en memoria, arranca vacía y desaparece al terminar. No toca `productos.db`.
2. Levanta la API con `httptest.NewServer(...)`. Es un servidor HTTP real, pero en un puerto libre elegido al azar (por ejemplo `http://127.0.0.1:53124`), así que no choca con el servidor que puedas tener corriendo en el 8080. La dirección queda en `srv.URL`.
3. Con `t.Cleanup(...)` deja anotado que al terminar el test se cierren el servidor y la base.

**`pedir(t, metodo, url, cuerpo, destino)`** hace una petición HTTP, igual que curl o Postman:
1. Arma la petición con el método, la URL y el cuerpo JSON.
2. La envía con `http.DefaultClient.Do(req)`.
3. Si se le pasa `destino`, convierte el JSON de la respuesta en ese valor (por ejemplo, un `Producto`).
4. Devuelve el código de estado (201, 404, etc.) para que el test lo compare con el esperado.

Las dos llaman a `t.Helper()`. Eso hace que, si fallan, Go muestre la línea del test que las llamó y no una línea de adentro de la función auxiliar, que es mucho más útil para encontrar el problema.

### Qué prueba cada test
**`TestCRUDCompleto`**: sigue la vida completa de un producto, en orden. Como la base arranca vacía, el primer producto siempre tiene `id` 1.

| Paso | Petición | Qué verifica |
|------|----------|--------------|
| 1 | `POST /productos` | Responde 201 y devuelve `id` 1 con los mismos datos enviados |
| 2 | `GET /productos` | Responde 200 y la lista tiene 1 producto |
| 3 | `GET /productos/1` | Responde 200 y el nombre es "Mate" |
| 4 | `PUT /productos/1` | Responde 200, los datos cambiaron y `creado_en` sigue igual |
| 5 | `DELETE /productos/1` | Responde 204 |
| 6 | `GET /productos/1` | Responde 404: el producto ya no existe |

**`TestNoEncontrado`**: prueba `GET`, `PUT` y `DELETE` sobre `/productos/99`, que no existe, y espera 404 en los tres.

**`TestValidaciones`**: manda cuerpos inválidos a `POST /productos` y espera 400 en todos. Al final prueba `GET /productos/abc` (id que no es número), que también tiene que dar 400.

| Caso | Cuerpo enviado | Qué regla de `leerProducto` lo rechaza |
|------|----------------|----------------------------------------|
| nombre vacío | `{"nombre":"  ",...}` | Después de `TrimSpace` el nombre queda vacío |
| precio negativo | `{"precio":-1,...}` | `p.Precio < 0` |
| stock negativo | `{"stock":-1,...}` | `p.Stock < 0` |
| JSON roto | `{"nombre":` | `Decode` no puede leerlo |
| campo extra | `{...,"color":"rojo"}` | `DisallowUnknownFields` |

### Tests de tabla
`TestNoEncontrado` y `TestValidaciones` usan un patrón muy común en Go: los **tests de tabla** (*table-driven tests*). En vez de escribir un bloque de código por caso, se arma una lista (o un `map`) con los datos de cada caso y un `for` los recorre:

```go
casos := map[string]string{
    "precio negativo": `{"nombre":"X","precio":-1,"stock":1}`,
    "JSON roto":       `{"nombre":`,
}
for nombre, cuerpo := range casos {
    if st := pedir(t, "POST", base, cuerpo, nil); st != http.StatusBadRequest {
        t.Errorf("%s: status %d, se esperaba 400", nombre, st)
    }
}
```

Para probar un caso nuevo alcanza con agregar una línea a la tabla. Y como usa `t.Errorf` (no `Fatalf`), si fallan varios casos se ven todos juntos.

### Comandos
| Comando | Qué hace |
|---------|----------|
| `go test` | Corre todos los tests y solo muestra el resultado final |
| `go test -v` | Muestra cada test con su resultado (*verbose*) |
| `go test -run Validaciones` | Corre solo los tests cuyo nombre coincide con el texto (es una expresión regular) |
| `go test -count=1` | Obliga a correrlos de nuevo. Sin esto, si el código no cambió, Go puede mostrar el resultado guardado con `(cached)` |
| `go test -cover` | Muestra qué porcentaje del código se ejecutó durante los tests |

Salida de `go test -v` cuando todo está bien:
```
=== RUN   TestCRUDCompleto
--- PASS: TestCRUDCompleto (0.01s)
=== RUN   TestNoEncontrado
--- PASS: TestNoEncontrado (0.00s)
=== RUN   TestValidaciones
--- PASS: TestValidaciones (0.00s)
PASS
ok  	crud-go	0.330s
```

Con `go test -cover` hoy da `coverage: 75.2% of statements`. Lo que queda sin cubrir son sobre todo los caminos de error interno (500), como que falle la base de datos, que son difíciles de provocar en un test.

**Cuando un test falla**, Go muestra `--- FAIL`, el archivo, la línea y el mensaje que se escribió en `t.Errorf` o `t.Fatalf`. Por ejemplo:
```
--- FAIL: TestValidaciones (0.00s)
    handlers_test.go:123: precio negativo: status 201, se esperaba 400
FAIL
```
Eso dice que la API aceptó un precio negativo (201) cuando tendría que haberlo rechazado (400).

### Ejercicio: agregar tu propio test
Una buena forma de practicar es agregar un test al final de `handlers_test.go`. Por ejemplo, para comprobar que la lista arranca vacía:

```go
func TestListaVacia(t *testing.T) {
    srv := nuevoServidor(t)

    var lista []Producto
    st := pedir(t, "GET", srv.URL+"/productos", "", &lista)
    if st != http.StatusOK {
        t.Fatalf("status %d, se esperaba 200", st)
    }
    if len(lista) != 0 {
        t.Fatalf("se esperaba lista vacía, llegaron %d productos", len(lista))
    }
}
```

Corrélo con `go test -v -run ListaVacia`. Otras ideas: probar que un `id` negativo da 400, o que un nombre con espacios alrededor (`"  Mate  "`) se guarda como `"Mate"`.

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
