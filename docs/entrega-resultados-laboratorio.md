# Informe de entrega de resultados del laboratorio

## Integración biométrica local con Go y OpenBiometrics

| Campo | Detalle |
|---|---|
| Tipo de documento | Entrega de resultados técnicos |
| Proyecto | OpenBiometrics Lab |
| Fecha de corte | 3 de noviembre de 2026 |
| Estado | Implementación integrada; validaciones funcionales parciales |
| Entorno | Local, contenedores Podman |

## Resumen ejecutivo

Se integró el motor biométrico OpenBiometrics con un backend Go y una interfaz
Svelte. El backend recibe imágenes JPEG/PNG, las valida y las reenvía al motor
como partes binarias multipart. Se habilitaron detección y comparación facial,
escaneo documental con extracción local heurística de campos de cédula
colombiana, comparación de documento contra selfie y creación de sesiones de
prueba de vida activa.

La infraestructura desplegada incluye los modelos YuNet y SFace en un volumen
persistente, los pesos OCR precargados en la imagen del motor y MediaPipe con
FaceLandmarker precargado para liveness activo. Se verificaron compilación y
pruebas de Go/frontend, salud de servicios, inicialización de OCR y
FaceLandmarker, y creación de una sesión de prueba de vida.


## 1. Objetivo y alcance de la entrega

Documentar los componentes instalados, su integración y los resultados
verificados para facilitar operación, mantenimiento y futuras pruebas del
laboratorio.

El trabajo cubre:

- Integración HTTP del cliente Go con la API OpenBiometrics.
- Validación y transporte de imágenes JPEG/PNG.
- Detección facial, comparación 1:1 y documento contra selfie.
- OCR y extracción local de campos para cédulas colombianas físicas y
  digitales.
- Flujo de prueba de vida activa por capturas sucesivas.
- Construcción de imágenes, modelos, configuración Podman y ajustes locales al
  upstream.

No cubre consultas a Registraduría, autenticidad documental, certificación
biométrica, evaluación formal de sesgo/precisión ni despliegue productivo.

## 2. Resultados verificados

| Componente | Resultado de entrega | Evidencia y límite |
|---|---|---|
| Backend Go | Integrado con OpenBiometrics mediante HTTP | `go test ./...` y `go vet ./...` pasaron en la validación del backend. |
| Frontend | Compila y sirve los flujos biométricos | `npm run check` y `npm run build` pasaron; frontend actualizado en Podman. |
| Comunicación | Frontend → Go → motor por red interna | Health de Go y health proxyado reportaron estado correcto (HTTP 200). |
| Detección facial | YuNet cargado desde el volumen de modelos | Se validó la ruta con imagen sintética en etapas previas; no mide precisión real. |
| Comparación facial | SFace cargado desde el volumen | Comparación local autorizada devolvió puntaje; no se calibró umbral ni precisión. |
| OCR | DocTR y pesos OCR inicializan desde caché de imagen | Inicialización exitosa como usuario sin privilegios; falta evaluar extracción con documentos reales autorizados. |
| Cédula colombiana | Extracción local de número, nombres, apellidos y fechas | Parser y validaciones locales cubiertos por pruebas; heurístico, no oficial. |
| Liveness activo | MediaPipe/FaceLandmarker inicializan; sesión `head_turn` creada y recibió un frame sintético | El frame fue aceptado y procesado; al no contener un rostro, no superó el reto. No se validó detección de movimiento con una captura real. |
| Liveness pasivo | No disponible de extremo a extremo | Falta el modelo opcional `antispoofing.onnx`. |

### Evidencias operativas

- La imagen del motor se construyó y desplegó con OCR y liveness activos.
- OCR cargó sus modelos precargados sin depender de descargas en ejecución.
- FaceLandmarker se inicializó con el usuario no privilegiado del contenedor.
- La red de servicios mantiene Go y el motor privados; solo se publica la UI en
  `127.0.0.1:8081`.
- El volumen `openbiometrics-models` se conservó al recrear el motor.
- Los ejemplos de solicitudes y las rutas implementadas se detallan en los
  anexos técnicos.

## 3. Dependencias y recursos entregados

| Recurso | Ubicación | Función | Estado |
|---|---|---|---|
| OpenBiometrics engine/API `0.3.0` | Imagen Podman | API y pipeline biométrico | Instalado |
| DocTR + PyTorch y pesos | `/opt/openbiometrics/ocr-cache` en imagen | OCR | Instalado y precargado |
| MediaPipe + `face_landmarker.task` | Caché bajo `/opt/openbiometrics` | Landmarks y retos activos | Instalado y precargado |
| YuNet `yunet.onnx` | Volumen `openbiometrics-models` | Detección de rostro | Descargado |
| SFace `sface.onnx` | Volumen `openbiometrics-models` | Comparación de rostro | Descargado |
| Go API | Contenedor `openbiometrics-api` | Validación y proxy | Desplegado |
| Svelte/Nginx | Contenedor `openbiometrics-frontend` | Interfaz local | Desplegado |

Siguen pendientes estos recursos opcionales para su validación y pruebas de ser necesarios:

- `antispoofing.onnx`: modelo de análisis pasivo que estima si un rostro parece
  corresponder a una persona real o a una presentación falsa, como una foto o
  una pantalla.
- `vit_genderage.onnx`: modelo que estima edad y género aparente a partir del
  rostro; no confirma esos datos.
- `yolov8n.pt`: pesos del modelo YOLOv8 nano para detección de objetos, incluida
  la detección de personas.
- Ultralytics: biblioteca necesaria para cargar y ejecutar `yolov8n.pt`.

## 4. Prueba de consumo y concurrencia

### Entorno y configuración medida

La prueba se ejecutó el 3 de noviembre de 2026 sobre el host disponible:

| Recurso | Valor observado |
|---|---|
| CPU del host | AMD Ryzen 5 150 con Radeon Graphics; 12 CPU lógicos |
| RAM del host | 7,4 GiB totales; 5,1 GiB disponibles antes de iniciar la carga y 3,9 GiB al finalizarla |
| GPU | Ningún dispositivo GPU asignado a los contenedores; las inferencias medidas se ejecutaron en CPU |
| Límites Podman | Sin límite explícito de CPU o memoria (`memory=0`, `cpuQuota=0`, `nanoCPUs=0`); límite de procesos 2048 por contenedor |
| Imágenes | Motor 7,27 GB; API Go 14,5 MB; frontend 64,5 MB |
| Modelos persistentes | Volumen YuNet/SFace: 38,93 MB |

El host es un entorno local virtualizado; sus cifras no representan por sí
solas un servidor productivo. La suma aproximada de imágenes es 7,35 GB,
además del volumen de modelos. El uso del motor varía según qué modelos se
inicialicen en el proceso.

### Método

- Se usaron únicamente imágenes PNG generadas para la prueba: una imagen de
  640×480 sin rostro para detección y una tarjeta documental sintética de
  1200×750 para OCR. No se utilizaron documentos ni rostros reales.
- Los servicios se llamaron a través del frontend/Nginx y el backend Go, como
  en el flujo del laboratorio.
- Se midieron secuencialmente concurrencias de 1, 2 y 4. Cada worker envió
  solicitudes en serie; no se simuló una tasa de llegada sostenida externa.
- Las latencias incluyen la ruta local frontend → Go → motor. La muestra OCR
  se hizo después de una primera solicitud que inicializó/calentó los modelos.
- CPU y memoria se muestrearon con `podman stats` cada 250 ms durante OCR. Los
  picos son observaciones durante esta ejecución, no límites ni reservas.

### Detección facial

| Concurrencia | Solicitudes | Respuestas | Tiempo total | Rendimiento | Latencia promedio | P95 |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 30 | 30 HTTP 200 | 0,85 s | 35,30 req/s | 28,2 ms | 40,8 ms |
| 2 | 60 | 60 HTTP 200 | 1,10 s | 54,71 req/s | 36,2 ms | 44,0 ms |
| 4 | 120 | 120 HTTP 200 | 1,91 s | 62,81 req/s | 63,1 ms | 73,6 ms |

La imagen intencionalmente no contenía un rostro; esta prueba mide el camino
de recepción, validación y detección negativa. No mide la calidad de detección
de rostros reales.

### OCR de documento

| Concurrencia | Solicitudes | Respuestas OCR | Tiempo total | Rendimiento | Latencia promedio | P95 |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 6 | 6 de 6 | 13,67 s | 0,44 req/s | 2,28 s | 2,38 s |
| 2 | 12 | 12 de 12 | 27,44 s | 0,44 req/s | 4,38 s | 4,70 s |
| 4 | 24 | 24 de 24 | 53,61 s | 0,45 req/s | 8,65 s | 9,03 s |

Al aumentar la concurrencia de 1 a 4, el rendimiento OCR se mantuvo cerca de
0,44–0,45 solicitudes/s mientras la latencia aumentó aproximadamente en
proporción a la concurrencia. En esta prueba, enviar más solicitudes en
paralelo no aumentó materialmente el caudal OCR y sí elevó el tiempo de
respuesta.

### CPU y memoria observadas

Durante la serie OCR, `podman stats` muestreó los contenedores durante 127
intervalos:

| Contenedor | CPU máxima muestreada | Memoria máxima muestreada |
|---|---:|---:|
| `openbiometrics-engine` | 286,4% (aprox. 2,86 CPU lógicas según la métrica de Podman) | 1,23 GiB |
| `openbiometrics-api` | 0,55% | 26,7 MiB |
| `openbiometrics-frontend` | 0,15% | 9,6 MiB |

El pico de CPU es instantáneo, no el promedio de toda la prueba. El cambio de
RAM disponible del host —de 5,1 a 3,9 GiB— es consistente con la memoria
adicional del motor al inicializar OCR, pero no debe tratarse como una medición
aislada exacta del RSS de todos los procesos.

### Prueba de vida activa

Se crearon sesiones `head_turn` con concurrencias de 1, 2 y 4. Cada sesión
recibió un frame de la tarjeta sintética sin rostro y luego se canceló. Las
solicitudes se procesaron y respondieron; `passed` fue `false` y la sesión
permaneció `in_progress`, resultado esperado para una imagen sin rostro.
Debido a la brevedad de esta prueba, no se usa como estimación de memoria pico
ni de capacidad sostenida para liveness con personas reales.

### Estimación preliminar para dimensionamiento

Para un laboratorio pequeño con las funciones actuales, las mediciones dan
como referencia inicial un servidor de **8 GB de RAM, 4 vCPU y al menos 10 GB
libres para imágenes/cachés**, sin GPU obligatoria. Es una recomendación
preliminar, no un mínimo garantizado: la máquina probada tenía 12 CPU lógicas y
7,4 GiB de RAM, y no se probó en hardware de 4 vCPU ni con carga de producción.

Para OCR, conviene iniciar con una concurrencia baja (por ejemplo, 1–2
solicitudes por instancia) o una cola, porque en esta prueba la concurrencia 4
no aumentó el caudal y sí elevó la latencia. Antes de fijar capacidad por
servidor o estimar solicitudes diarias se requiere una prueba más larga con
resoluciones y documentos autorizados representativos, patrón de carga
definido, varias repeticiones y margen de memoria.

## 5. Conclusiones y próximos pasos

La integración entre Go y OpenBiometrics está operativa para las rutas
probadas, y la imagen del motor tiene las dependencias necesarias para iniciar
OCR y prueba de vida activa sin descargar sus modelos al procesar solicitudes.
La extracción de campos colombianos está implementada como una capa local
heurística sobre el texto OCR.

Antes de considerar el laboratorio listo para una evaluación funcional
representativa:

1. Probar OCR con cédulas físicas (frente y reverso) y digitales autorizadas,
   midiendo campos correctamente extraídos y errores por campo.
2. Probar liveness desde un navegador con permiso de cámara, confirmar que se
   reciben varios frames y que el reto avanza ante los movimientos indicados.
3. Decidir si se instalará y validará `antispoofing.onnx` para análisis pasivo.
4. Calibrar umbrales de comparación con muestras autorizadas y criterios
   definidos; no usar el valor predeterminado como umbral productivo.
5. Para un entorno productivo, definir cifrado, autorización, retención y
   eliminación de imágenes, auditoría, base legal y monitorización.

## Anexo A. Arquitectura de integración

En el despliegue local, los componentes se comunican así:

```text
Navegador
  │ HTTP (127.0.0.1:8081)
  ▼
Frontend Svelte + Nginx
  │ /api/* → Go (red interna openbiometrics-lab)
  ▼
Backend Go :8080
  │ HTTP multipart/JSON → http://openbiometrics:8000
  ▼
API OpenBiometrics + motor Python
```

Solo el frontend se publica en el host, enlazado a `127.0.0.1:8081`. Go y
OpenBiometrics permanecen en la red privada de Podman; no se publica el puerto
del motor. Go es el adaptador de integración: valida las solicitudes,
construye las llamadas HTTP al motor, valida la forma de algunas respuestas y
presenta una API propia bajo `/api/v1/biometrics/...`.

El checkout upstream está en `openbiometrics/`, fijado en el commit
`ab09d469327684d630b7383119dd4544253a03c9` (versión `0.3.0`). Los ajustes
locales del motor se aplican al construir la imagen y no modifican el checkout
upstream.

## Anexo B. Dependencias y modelos

### Imagen del motor

La imagen definida por `containers/OpenBiometrics.Containerfile` incluye:

- Proyecto upstream `engine` con los extras `document` y `liveness`.
- Proyecto upstream `api`.
- `python-doctr[torch]` para OCR y PyTorch.
- MediaPipe para landmarks faciales de prueba de vida activa.
- Bibliotecas del sistema requeridas por OpenCV y MediaPipe, incluidas EGL y
  GLES.
- Pesos OCR de detección y reconocimiento precargados durante el build en
  `/opt/openbiometrics/ocr-cache`.
- Modelo `face_landmarker.task` precargado por MediaPipe durante el build y
  accesible en la caché bajo `/opt/openbiometrics`.

Estas cachés se preparan antes de cambiar al usuario no privilegiado
`10001:10001`; el motor puede inicializar los modelos sin descargar archivos al
procesar una solicitud.

### Volumen de modelos faciales

El volumen Podman `openbiometrics-models` se monta en
`/opt/openbiometrics/engine/models` como solo lectura en el contenedor
principal:

- `yunet.onnx`: detector de rostros YuNet. En el registro upstream figura con
  licencia MIT.
- `sface.onnx`: reconocedor facial SFace utilizado por comparación 1:1 y
  documento contra selfie. En el registro upstream figura con licencia
  Apache-2.0.

El README de contenedores contiene los comandos usados para crear el volumen,
descargar cada archivo desde el registro upstream, verificar su checksum y
reiniciar el motor. Los modelos faciales y las cachés OCR/MediaPipe tienen
ubicaciones distintas: YuNet y SFace están en el volumen; OCR y FaceLandmarker
están en la imagen.

### Componentes que aún requieren recursos

En el estado documentado del laboratorio:

- `antispoofing.onnx` no está en el volumen. Por tanto, no se debe interpretar
  la disponibilidad de los retos activos como disponibilidad de un análisis
  pasivo anti-spoofing.
- `ultralytics` y `yolov8n.pt` no están instalados; detección de personas no
  está habilitada de extremo a extremo.
- `vit_genderage.onnx` no está en el volumen; demografía requiere ese modelo.

Antes de distribuir los modelos en otro entorno, revisar sus fuentes, términos
de licencia y políticas de uso aplicables.

## Anexo C. Creación del cliente Go

El cliente del motor se crea en `backend/cmd/api/main.go`. `NewClient` recibe
una URL base HTTP(S) y un `http.Client`; rechaza URLs con credenciales, query o
fragmento. El timeout por defecto es de 30 segundos, configurable hasta 600
segundos mediante `BIOMETRICS_TIMEOUT_SECONDS`.

```go
engineClient, err := biometrics.NewClient(
    cfg.OpenBiometricsBaseURL,
    &http.Client{Timeout: cfg.BiometricsTimeout},
)
if err != nil {
    log.Fatalf("configure OpenBiometrics client: %v", err)
}
```

Valores de configuración principales:

| Variable | Valor por defecto | Uso |
|---|---:|---|
| `GO_ADDR` | `:8080` | Dirección HTTP de Go |
| `OPENBIOMETRICS_BASE_URL` | `http://openbiometrics:8000` | URL interna del motor |
| `MAX_IMAGE_BYTES` | `5242880` | Límite por archivo; máximo configurable de 25 MiB |
| `BIOMETRICS_TIMEOUT_SECONDS` | `30` | Timeout HTTP hacia el motor; máximo 600 s |

La comprobación `GET /api/v1/biometrics/health` llama
`GET /api/v1/health` del motor y requiere respuesta exitosa, `status: "ok"` y
`models_loaded: true`. `GET /healthz` solo comprueba que el servicio Go está
vivo.

## Anexo D. Codificación y validación de imágenes

En este laboratorio, las imágenes **no se convierten a Base64** para las rutas
biométricas. El navegador o consumidor envía JPEG/PNG como partes binarias de
`multipart/form-data`. Go las lee a `[]byte`, identifica el tipo a partir del
contenido, comprueba que el archivo se pueda decodificar y vuelve a formar un
multipart binario para OpenBiometrics. Esto evita el aumento de tamaño propio
de Base64 y coincide con los contratos del motor.

Go acepta únicamente `image/jpeg` y `image/png`, verifica que cada imagen no
supere `MAX_IMAGE_BYTES` y rechaza imágenes vacías, malformadas o con
dimensiones fuera del límite interno. El `Content-Type` del archivo se
determina por su contenido; no se confía solo en la extensión o el MIME que
declare el navegador.

Ejemplo de uso directo del cliente Go para comparar dos archivos locales:

```go
package example

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"

    "openbiometrics-lab/backend/internal/biometrics"
)

func compareFaces(ctx context.Context, engineURL, firstPath, secondPath string) (json.RawMessage, error) {
    client, err := biometrics.NewClient(engineURL, &http.Client{Timeout: 30 * time.Second})
    if err != nil {
        return nil, err
    }

    first, err := os.ReadFile(firstPath)
    if err != nil {
        return nil, fmt.Errorf("read first image: %w", err)
    }
    second, err := os.ReadFile(secondPath)
    if err != nil {
        return nil, fmt.Errorf("read second image: %w", err)
    }

    firstType := http.DetectContentType(first)
    secondType := http.DetectContentType(second)
    if (firstType != "image/jpeg" && firstType != "image/png") ||
        (secondType != "image/jpeg" && secondType != "image/png") {
        return nil, fmt.Errorf("images must be JPEG or PNG")
    }
    result, err := client.Verify(ctx, firstType, first, secondType, second, 0.4)
    if err != nil {
        return nil, err
    }
    return json.RawMessage(result), nil
}
```

El ejemplo ilustra la API interna; en un paquete de aplicación se debe respetar
la política de validación de archivos y no exponer credenciales ni el puerto
interno del motor. El límite de 5 MiB se aplica en el backend HTTP del
laboratorio; una llamada directa al cliente debe establecer límites antes de
leer archivos.

El paquete Go vive bajo `backend/internal/biometrics`; por la regla `internal`
de Go, solo puede importarse desde paquetes dentro del módulo `backend`. Para
otro repositorio, usar la API HTTP del laboratorio o extraer el cliente a un
módulo público con su propio contrato y pruebas.

Si un sistema externo solo puede transportar imágenes dentro de JSON, ese
sistema puede representarlas en Base64 y decodificarlas antes de invocar el
cliente. Esa conversión **no forma parte del contrato implementado aquí**:

```go
imageBytes, err := base64.StdEncoding.DecodeString(encodedImage)
if err != nil {
    return fmt.Errorf("decode image base64: %w", err)
}
contentType := http.DetectContentType(imageBytes)
// Validar JPEG/PNG y tamaño antes de usar imageBytes en el cliente.
```

## Anexo E. Rutas de la API Go

Las rutas del navegador y de integraciones locales hablan con Go, no con el
motor directamente:

| Operación | Ruta Go | Partes/valores |
|---|---|---|
| Salud de Go | `GET /healthz` | — |
| Salud del motor | `GET /api/v1/biometrics/health` | — |
| Detección facial | `POST /api/v1/biometrics/detect` | `image` |
| Comparación 1:1 | `POST /api/v1/biometrics/verify` | `image1`, `image2`; `threshold` opcional |
| Lectura de documento | `POST /api/v1/biometrics/documents/scan` | `image`, `back_image` opcional; `presentation` |
| Documento contra selfie | `POST /api/v1/biometrics/documents/verify` | `document`, `selfie`; `threshold` opcional |
| Presets de liveness | `GET /api/v1/biometrics/liveness/presets` | — |
| Crear sesión activa | `POST /api/v1/biometrics/liveness/sessions?preset=head_turn` | `preset` opcional |
| Enviar frame | `POST /api/v1/biometrics/liveness/sessions/{id}/frame` | `image` |
| Consultar sesión | `GET /api/v1/biometrics/liveness/sessions/{id}` | — |
| Cancelar sesión | `DELETE /api/v1/biometrics/liveness/sessions/{id}` | — |

El umbral de comparación está entre `0` y `1`, con valor predeterminado `0.4`.
El umbral no es una medida universal de identidad: debe calibrarse con datos
autorizados representativos antes de cualquier uso de producción.

### Ejemplos multipart con curl

Detección facial:

```sh
curl --fail-with-body \
  -F 'image=@rostro.jpg;type=image/jpeg' \
  http://127.0.0.1:8081/api/v1/biometrics/detect
```

Comparación 1:1:

```sh
curl --fail-with-body \
  -F 'image1=@persona-a.jpg;type=image/jpeg' \
  -F 'image2=@persona-b.png;type=image/png' \
  -F 'threshold=0.4' \
  http://127.0.0.1:8081/api/v1/biometrics/verify
```

El proxy de Nginx del laboratorio enruta `/api/` hacia Go. En otro despliegue,
usar la URL del backend correspondiente y no publicar el servicio de motor
directamente.

## Anexo F. Lectura local de cédulas colombianas

### Qué prepara la lectura

El OCR upstream se instala con `python-doctr[torch]`; sus pesos de detección y
reconocimiento se precargan durante el build de la imagen. En ejecución, el
motor usa la caché local, por lo que la lectura no depende de que el contenedor
tenga acceso a Internet. La lectura de texto es realizada por OpenBiometrics;
la extracción de campos colombianos y sus validaciones básicas se realizan
localmente en Go.

La categoría del documento es `CC`, tanto para presentación física como
digital. La interfaz envía `presentation=physical` o `presentation=digital`;
para el documento físico se puede adjuntar el reverso, donde suele estar la
fecha de expedición. Para la presentación digital, si no se detecta un
contorno rectangular, el motor aplica OCR a la captura completa y la marca
como detectada solo cuando encuentra indicadores textuales de cédula
colombiana. Esto mejora la lectura de capturas de pantalla, pero no verifica
autenticidad. Go:

1. Valida y envía el frente al endpoint upstream
   `/api/v1/documents/scan`.
2. Si se adjunta `back_image`, escanea también el reverso.
3. Combina texto OCR de ambos lados; conserva el estado facial del frente y
   toma el MRZ disponible.
4. Extrae y valida localmente los campos del texto combinado.

Ejemplo físico con anverso y reverso:

```sh
curl --fail-with-body \
  -F 'image=@cedula-frente.jpg;type=image/jpeg' \
  -F 'back_image=@cedula-reverso.jpg;type=image/jpeg' \
  -F 'presentation=physical' \
  http://127.0.0.1:8081/api/v1/biometrics/documents/scan
```

Ejemplo de presentación digital:

```sh
curl --fail-with-body \
  -F 'image=@cedula-digital.png;type=image/png' \
  -F 'presentation=digital' \
  http://127.0.0.1:8081/api/v1/biometrics/documents/scan
```

La respuesta añade `colombian_id`, con campos como:

```json
{
  "colombian_id": {
    "document_type": "CC",
    "presentation": "physical",
    "document_number": "1234567890",
    "number_format_valid": true,
    "surnames": "EJEMPLO APELLIDO",
    "given_names": "EJEMPLO NOMBRE",
    "birth_date": "1990-01-02",
    "birth_date_valid": true,
    "issue_date": "2010-03-04",
    "issue_date_valid": true,
    "authenticity_verified": false,
    "registry_verified": false,
    "warnings": []
  }
}
```

El ejemplo es ilustrativo; esos datos no corresponden a una consulta real.
Las validaciones actuales son heurísticas locales:

- Número: entre 6 y 10 dígitos después de normalizar separadores.
- Nombres y apellidos: se extraen alrededor de etiquetas OCR como `NOMBRES`
  y `APELLIDOS`.
- Fechas: se buscan cerca de etiquetas de nacimiento y expedición; se
  normalizan a `YYYY-MM-DD`, se comprueba que sean fechas posibles y no
  futuras. Si ambas son válidas, expedición no puede ser anterior al
  nacimiento.

Los formatos borrosos, reflejos, rotación, recortes y errores OCR pueden
producir campos faltantes o incorrectos. Estas reglas **no comprueban
autenticidad, vigencia legal, titularidad ni coincidencia con Registraduría u
otra fuente oficial**. El campo `registry_verified` permanece en `false`.

### Comparación documento contra selfie

La comparación usa el campo `document` y el campo `selfie`; no necesita que el
OCR lea los datos del documento. La imagen del documento se rectifica y se
extrae su zona de rostro. Los parches locales habilitan la extracción, amplían
recortes pequeños y reintentan la detección en el documento rectificado.
Después se compara el rostro extraído con la selfie mediante el pipeline
facial y SFace.

```sh
curl --fail-with-body \
  -F 'document=@cedula-frente.jpg;type=image/jpeg' \
  -F 'selfie=@selfie.jpg;type=image/jpeg' \
  -F 'threshold=0.4' \
  http://127.0.0.1:8081/api/v1/biometrics/documents/verify
```

Un puntaje de similitud y su umbral deben validarse con un conjunto de pruebas
autorizado y adecuado al caso de uso; una sola muestra no demuestra precisión.

## Anexo G. Prueba de vida activa

El motor requiere MediaPipe y el modelo FaceLandmarker. Ambos se instalan o
precargan en la imagen; `face_landmarker.task` no se descarga al recibir una
solicitud. La prueba activa recibe imágenes, no un video codificado:

1. El cliente solicita presets y elige uno activo, por ejemplo `head_turn`.
2. Go crea una sesión upstream y devuelve un `session_id` y una secuencia
   aleatoria de retos.
3. Mientras la persona realiza el reto, el frontend captura JPEG de la cámara
   y envía frames sucesivos, uno por solicitud.
4. El motor calcula landmarks y evalúa cada frame. Si `passed` es `true`,
   el frontend avanza al siguiente reto; si `session_complete` es `true`,
   detiene la cámara.
5. Si el usuario cancela, Go elimina la sesión en el motor.

Ejemplo de creación:

```sh
curl --fail-with-body -X POST \
  'http://127.0.0.1:8081/api/v1/biometrics/liveness/sessions?preset=head_turn'
```

Enviar un frame (repetir en orden mientras el reto esté activo):

```sh
curl --fail-with-body \
  -F 'image=@frame-001.jpg;type=image/jpeg' \
  'http://127.0.0.1:8081/api/v1/biometrics/liveness/sessions/SESSION_ID/frame'
```

Las respuestas de frame incluyen `passed`, `confidence`,
`challenges_remaining`, `session_complete`, `is_live` y `state`. Los nombres de
retos disponibles se consultan en `GET /api/v1/biometrics/liveness/presets`;
entre ellos están
`eye`, `smile`, `multi_range`, `head_turn` y `full`. El preset
`passive_only` no inicia una sesión activa.

En el frontend del laboratorio, las capturas se envían automáticamente con una
pausa breve entre solicitudes, sin esperar un clic para cada frame. Es una
secuencia de imágenes JPEG sucesivas, no WebRTC ni un stream de video continuo.
La cámara requiere permiso explícito del navegador y debe estar disponible en
un contexto seguro (HTTPS o localhost).

Los retos son experimentales. El envío sucesivo permite evaluar movimientos,
pero no equivale por sí mismo a una solución anti-spoofing certificada. El
modelo opcional `antispoofing.onnx` no está incluido. No afirmar rechazo de
replays, máscaras u otros ataques sin pruebas y controles adicionales.

## Anexo H. Parches locales del motor

Los parches en `containers/patches/` se aplican a copias dentro de la imagen:

1. `0001-pipeline-config.patch`: alinea atributos de selección de detector,
   reconocedor y modelo demográfico entre `FaceConfig` y `FacePipeline`.
2. `0002-enable-document-face-extraction.patch`: activa la extracción facial
   en la configuración upstream del pipeline documental.
3. `0003-improve-document-face-detection.patch`: amplía el rostro extraído
   cuando es pequeño, reintenta sobre el documento rectificado y hace que
   `has_face` dependa de detección facial real.

Además, el Containerfile copia `VERSION` a los proyectos upstream y ajusta la
ruta de versión del paquete para que setuptools construya la imagen sin leer
archivos fuera de la raíz del proyecto. Ninguno de estos cambios altera el
checkout upstream.

## Anexo I. Límites operativos y privacidad

- El volumen de YuNet/SFace se conserva al recrear el motor y se monta como
  solo lectura en producción de laboratorio.
- La red de ejecución es interna; las dependencias y pesos que se requieran
  deben instalarse o precargarse durante el build.
- Go devuelve errores de entrada como 4xx y sanitiza fallos del motor como
  502. Los detalles internos quedan en logs locales del backend.
- La interfaz no persiste imágenes en su almacenamiento. Las solicitudes sí
  transportan datos biométricos al motor; una instalación productiva debe
  definir cifrado en tránsito, retención, controles de acceso, eliminación,
  auditoría y base legal.
- Usar únicamente documentos y rostros de personas autorizadas. No guardar
  imágenes de prueba dentro del repositorio.
- Las pruebas Go (`cd backend && go test ./...`) usan imágenes sintéticas y
  servidores HTTP locales. El estado de salud verifica conectividad/modelos,
  no exactitud del OCR, precisión de identidad ni efectividad del liveness.

## Anexo J. Referencias de implementación

- Cliente HTTP y creación: `backend/internal/biometrics/client.go`,
  `backend/cmd/api/main.go`.
- Multipart, verificación, documentos y liveness:  
  `backend/internal/biometrics/operations.go`.
- Validación y lectura de archivos: `backend/internal/httpapi/uploads.go`.
- Rutas del backend: `backend/internal/httpapi/health.go`,
  `backend/internal/httpapi/documents.go`,
  `backend/internal/httpapi/liveness.go`.
- Extracción y fusión de OCR colombiano:
  `backend/internal/documents/colombian_id.go`,
  `backend/internal/documents/merge_scan.go`,
  `backend/internal/documents/scan_response.go`.
- Imagen y parches upstream: `containers/OpenBiometrics.Containerfile`,
  `containers/patches/`.
- Instrucciones del despliegue y comandos de descarga de modelos:
  `containers/README.md`.
