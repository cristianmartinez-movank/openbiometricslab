# OpenBiometrics engine container

This Containerfile packages the upstream engine and API without changing the
upstream checkout. Run these commands from the laboratory root.

Related documentation:

- [Integration guide](../docs/integracion-go-openbiometrics.md)
- [Laboratory results handoff](../docs/entrega-resultados-laboratorio.md)

## Current status

- Upstream checkout: `openbiometrics/`, detached at
  `ab09d469327684d630b7383119dd4544253a03c9` (version `0.3.0`).
- Image `localhost/openbiometrics-lab-engine:0.3.0` built successfully with
  Podman on 2026-10-30. Image ID:
  `f2672ac75160786d00f5735a7f62a6cc6b0fbfb5202197f73084519cd879727c`.
- The first build failed because setuptools does not allow the engine package's
  dynamic version file (`../VERSION`) to escape its project root.
- The local Containerfile copies `VERSION` into each package directory and
  rewrites the version-file path in the image copies of both `pyproject.toml`
  files. This is only an image-build workaround; it does not change the pinned
  upstream checkout.
- The first container start exposed an upstream configuration mismatch:
  `PipelineConfig` lacked model-selection attributes that `FacePipeline.load()`
  reads. The local patch in `containers/patches/0001-pipeline-config.patch`
  adds those fields and passes them from `FaceConfig`. The initial container
  exited with code 3; after applying this image-only patch, rebuilding, and
  recreating the container, startup succeeded. The upstream checkout remains
  unchanged.
- The document-selfie route requires face extraction from the warped document.
  The upstream API configuration leaves that option disabled by default, so
  the image-only patch `containers/patches/0002-enable-document-face-extraction.patch`
  enables it. Rebuild and recreate the engine container after changing this
  patch; the model volume is preserved.
- Document verification previously failed when YuNet missed the small face
  inside the extracted photo region. The image-only patch
  `containers/patches/0003-improve-document-face-detection.patch` upscales
  small regions, retries face detection against the rectified document, and
  makes document scan's `has_face` field depend on actual face detection
  rather than merely the existence of a crop.
- YuNet was downloaded to the Podman volume `openbiometrics-models` on
  2026-10-30. Verified file: `yunet.onnx`, 228 KB,
  SHA-256
  `8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4`.
  No face image or biometric sample was used.
- The 1:1 comparison initially returned HTTP 500 because the engine had no
  face-recognition model loaded; YuNet is only a detector. With approval,
  SFace was downloaded to the same volume on 2026-10-30. Verified file:
  `sface.onnx`, 38,696,353 bytes, SHA-256
  `0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79`.
  The upstream registry lists SFace as Apache-2.0. No face image or biometric
  sample was used for the download or model-load verification.
- After restarting the engine, `GET /api/v1/admin/models` reported
  `face_recognizer: loaded=true`. A local authorized test returned a comparison
  score; matching thresholds still require separate calibration.
- Document-selfie verification was returning HTTP 400 because the API's
  `DocumentConfig` left `enable_face_extraction` disabled. The local image
  patch `containers/patches/0002-enable-document-face-extraction.patch` enables
  it without modifying the upstream checkout. OCR is still optional and is not
  required for selfie comparison. A further failure showed YuNet could not
  detect the face in the extracted region; patch 0003 adds upscaling and
  retries against the rectified document. With the user's selected local
  document and selfie, the endpoint then returned a comparison score instead
  of HTTP 400. No image or score is stored in this repository.
- The engine is running as `openbiometrics-engine` on internal network
  `openbiometrics-lab`; no host port is published.
- The engine image includes DocTR and its pretrained OCR weights. OCR
  initialization was verified in the running, unprivileged container using
  the image cache. An authorized Colombian ID image has not yet been used to
  measure field-extraction quality.
- Active liveness now includes MediaPipe and its preloaded FaceLandmarker
  model. Detector initialization succeeded as the unprivileged container user,
  and creating a `head_turn` session through the frontend proxy returned HTTP
  200. No camera frames were submitted; challenge evaluation still needs an
  authorized browser-camera test.
- `GET /api/v1/health` returned HTTP 200 and reported `models_loaded: true`.
  `GET /openapi.json` returned HTTP 200 and includes `/api/v1/detect`.
- A synthetic, non-biometric blank image was submitted through the Go detection
  route to the running engine and returned `{"faces":[],"count":0}`. A real,
  authorized face image has not yet been used to validate face detection.
- The installed distribution reports version `0.3.0`, but the API runtime
  reports `0.0.0-dev`: upstream `openbiometrics.__init__` looks for `VERSION`
  relative to the source checkout, which is not present at that location after
  installation into site-packages. This does not prevent the tested health or
  OpenAPI routes from working.
- The Go backend now implements `GET /healthz` and
  `GET /api/v1/biometrics/health`. Its tests pass, and the Podman instance
  successfully reached the engine on the private network.
- The Go backend also implements `POST /api/v1/biometrics/detect`: JPEG/PNG,
  configurable file limit (default 5 MiB), content-based type and decoding
  checks, and validation of the upstream `faces`/`count` JSON. Unit tests use
  generated synthetic images and fake HTTP services. A separate live-engine
  integration used a synthetic blank image; no real face image was used.
- Colombian ID scan support distinguishes the physical and digital presentation
  of the same citizenship card. It extracts OCR text for number, names,
  surnames, birth date, and (when supplied) issue date, then applies local
  format/date checks. Physical cards can be submitted as front plus optional
  reverse; OCR from both sides is combined. This is not an authenticity,
  validity, or Registraduría lookup.
- The Go backend now also proxies 1:1 face comparison, document scanning,
  document-to-selfie verification, and the active liveness session lifecycle.
  Their multipart fields and response contracts are covered by local HTTP
  tests. A synthetic blank image was also sent through the running document
  scan route, which returned no document/OCR/MRZ result. An authorized local
  document/selfie pair was used to verify that the comparison route returns a
  result; biometric files and their score are not persisted.
- Go maps known document/selfie face-detection failures to safe error codes
  that the frontend explains in Spanish. Unknown engine details remain
  sanitized rather than being returned to the browser.
- Podman images `localhost/openbiometrics-lab-api:0.1.0` and
  `localhost/openbiometrics-lab-frontend:0.1.0` are built and running as
  `openbiometrics-api` and `openbiometrics-frontend`. Only the frontend is
  published, on `127.0.0.1:8081`; Go and the engine remain private. The
  frontend-to-Go-to-engine health check returned HTTP 200.
- A separate Svelte + TypeScript frontend now lives in `frontend/`. It provides
  UI flows for face detection, 1:1 comparison, document OCR/MRZ and
  document-selfie verification, and active liveness capture. Liveness sends
  camera frames sequentially until each challenge passes, with pause/resume
  controls. It calls Go only; the upstream React dashboard remains unchanged.
- Optional-component audit: OCR's `python-doctr[torch]` dependency and pretrained
  weights, plus MediaPipe and its `face_landmarker.task` model, are included and
  preloaded in the engine image. `ultralytics` is not installed.
  `antispoofing.onnx`, `vit_genderage.onnx`, and `yolov8n.pt` are not in the
  model volume. Passive liveness, demographics, and person detection still
  need their optional models or dependencies.
- The core document-selfie flow does not need OCR or an additional ONNX model:
  document detection uses OpenCV, face extraction is enabled by the local patch,
  and face matching uses the loaded YuNet and SFace models.

The local environment has Podman 4.9.3, Go 1.22.2, Node.js 24.22.1, and Python
3.12.3. `podman compose` has no installed Compose provider, so the current
engine instructions use `podman build` and `podman run` directly.

## Build

```sh
podman build \
  --file containers/OpenBiometrics.Containerfile \
  --tag openbiometrics-lab-engine:0.3.0 \
  .
```

The image installs the upstream `engine` with its document/OCR extra and the
`api` project from the pinned checkout. It applies local patches for face-model
selection and document face extraction, and preloads the OCR text-detection and
recognition weights during image build. Runtime OCR therefore does not depend
on network access. YuNet and SFace remain separate files in the read-only model
volume.

## Prepare face detection and recognition models

The upstream API config points to `engine/models`. Its model download script
defaults to `./models`, and the `face` group downloads additional models that
are not needed for detection. To seed only the MIT-licensed YuNet detector
into a named volume, first create the volume:

```sh
podman volume create openbiometrics-models
```

The approved download was run with:

```sh
podman run --rm \
  --user 0:0 \
  --volume openbiometrics-models:/opt/openbiometrics/engine/models \
  --entrypoint python \
  openbiometrics-lab-engine:0.3.0 \
  -c "import sys; sys.path.insert(0, '/opt/openbiometrics/engine'); from openbiometrics.runtime.registry import ModelRegistry; ModelRegistry('/opt/openbiometrics/engine/models').ensure_model('yunet')"
```

This downloads the model from the URL recorded in the upstream model registry.
The temporary root user is needed to write to a newly created named volume;
the long-running service uses the image's unprivileged user and mounts the
volume read-only. No face image or biometric sample is required for this step.
To verify the volume contents:

```sh
podman run --rm \
  --volume openbiometrics-models:/models:ro \
  --entrypoint sh \
  openbiometrics-lab-engine:0.3.0 \
  -c 'test -s /models/yunet.onnx && ls -lh /models/yunet.onnx && sha256sum /models/yunet.onnx'
```

YuNet detects faces but cannot compare them. SFace is the recognition model
required by the 1:1 comparison endpoint. It is listed as Apache-2.0 in the
upstream model registry. Download it separately to the existing model volume:

```sh
podman run --rm \
   --user 0:0 \
   --volume openbiometrics-models:/opt/openbiometrics/engine/models \
   --entrypoint python \
   openbiometrics-lab-engine:0.3.0 \
   -c "import sys; sys.path.insert(0, '/opt/openbiometrics/engine'); from openbiometrics.runtime.registry import ModelRegistry; ModelRegistry('/opt/openbiometrics/engine/models').ensure_model('sface')"
```

Verify the downloaded model before restarting the engine:

```sh
podman run --rm \
   --volume openbiometrics-models:/models:ro \
   --entrypoint sh \
   openbiometrics-lab-engine:0.3.0 \
   -c 'test -s /models/sface.onnx && stat -c "%n %s bytes" /models/sface.onnx && sha256sum /models/sface.onnx'
```

Restart the engine to load SFace, then verify the recognizer status:

```sh
podman restart openbiometrics-engine
podman exec openbiometrics-engine python -c \
   "from urllib.request import urlopen; print(urlopen('http://127.0.0.1:8000/api/v1/admin/models', timeout=10).read().decode())"
```

The model must appear with `"name": "face_recognizer"` and `"loaded": true`.
This status confirms availability, not matching accuracy; use only authorized
images to test a real 1:1 comparison.

## Next steps

1. The engine and both application containers are running. Engine health,
   Go health, frontend serving, API proxying, and a synthetic blank-image
   inference passed. The face detector and SFace recognizer are loaded, and
   document face extraction is enabled. A local authorized document/selfie pair
   returned a similarity result; threshold calibration and identity accuracy
   are not established by that test. Colombian CC OCR needs an image-quality
   test for both physical and digital presentations. Active liveness now has
   its MediaPipe dependency and landmark model; passive liveness, demographics,
   and person detection still need their optional models or dependencies.
2. The Go API routes are implemented and covered by local tests. Run
   `cd backend && go test ./...` from the laboratory root.
3. Test active liveness with an authorized participant and browser camera
   permission. The health route alone does not verify challenge evaluation.
4. Test engine integration only with approved, authorized image fixtures.
   Current Go tests use synthetic images and local fake HTTP servers.
5. To recreate the Go and Svelte containers, use the commands below. The
   frontend is the only service published to the host and proxies `/api` to
   Go. Keep the engine private.

If any step fails, record the exact error and agree on a local adjustment
before changing the Containerfile or upstream checkout.

## Run on an internal network

```sh
podman network create --internal openbiometrics-lab
podman run --detach \
  --name openbiometrics-engine \
  --network openbiometrics-lab \
  --network-alias openbiometrics \
  --volume openbiometrics-models:/opt/openbiometrics/engine/models:ro \
  openbiometrics-lab-engine:0.3.0
```

The engine port is not published to the host. A Go service attached to the same
network can reach it at `http://openbiometrics:8000`. Do not attach this
network to an untrusted network or publish the engine port publicly.

## Build and run Go and the Svelte frontend with Podman

Build both images from the laboratory root. The Go image is a static,
unprivileged service. The frontend image builds the Svelte app and serves it
with Nginx, which proxies API requests over the Podman network.

```sh
podman build \
  --file containers/Go.Containerfile \
  --tag localhost/openbiometrics-lab-api:0.1.0 \
  .

podman build \
  --file containers/Frontend.Containerfile \
  --tag localhost/openbiometrics-lab-frontend:0.1.0 \
  .
```

Attach Go to the existing `openbiometrics-lab` network. It reaches the engine
by the `openbiometrics` network alias; it has no published host port.

```sh
podman run --detach \
  --name openbiometrics-api \
  --network openbiometrics-lab \
  --network-alias openbiometrics-api \
  --env OPENBIOMETRICS_BASE_URL=http://openbiometrics:8000 \
  localhost/openbiometrics-lab-api:0.1.0
```

Only publish the frontend, bound to loopback. The browser accesses Go through
the frontend's Nginx `/api/` proxy; neither the Go API nor the engine is
published to the host.

```sh
podman run --detach \
  --name openbiometrics-frontend \
  --network openbiometrics-lab \
  --publish 127.0.0.1:8081:8080 \
  localhost/openbiometrics-lab-frontend:0.1.0
```

Open `http://127.0.0.1:8081`. Verify the frontend and backend-to-engine health
through the proxy:

```sh
curl --fail http://127.0.0.1:8081/
curl --fail http://127.0.0.1:8081/api/v1/biometrics/health
podman exec openbiometrics-api wget -qO- http://127.0.0.1:8080/healthz
```

For OCR and face inference, submit authorized JPEG or PNG files through the UI.
The engine image includes preloaded OCR weights; face recognition uses models
in the named model volume. Active liveness still requires optional dependencies
and models. Do not mount or store biometric test images in the repository.

To stop the app containers without removing the engine or its model volume:

```sh
podman stop openbiometrics-frontend openbiometrics-api
podman rm openbiometrics-frontend openbiometrics-api
```

## Go backend API routes

The Go module is in `backend/`. Its settings are:

| Variable | Default | Validation |
|----------|---------|------------|
| `GO_ADDR` | `:8080` | Host:port with a numeric port from 1 to 65535 |
| `OPENBIOMETRICS_BASE_URL` | `http://openbiometrics:8000` | Must be an HTTP(S) URL accepted by the client |
| `MAX_IMAGE_BYTES` | `5242880` (5 MiB) | Positive integer, maximum 25 MiB |
| `BIOMETRICS_TIMEOUT_SECONDS` | `30` | Positive integer, maximum 600 seconds |

- `GET /healthz` checks that the Go HTTP service is alive.
- `GET /api/v1/biometrics/health` calls the engine's
  `GET /api/v1/health`, requiring HTTP success, JSON `status: "ok"`, and
  `models_loaded: true`. Engine failures return a sanitized HTTP 503 response.
- `POST /api/v1/biometrics/detect` accepts one multipart field named `image`,
  permits JPEG and PNG up to `MAX_IMAGE_BYTES`, checks the file content and image decoding,
  then forwards to the engine's `POST /api/v1/detect`. The response is checked
  for `faces`, `count`, and a `detection` object per face. Invalid input returns
  a 4xx response; engine or engine-contract failures return a sanitized 502.
- `POST /api/v1/biometrics/verify` accepts `image1`, `image2`, and an optional
  `threshold` from 0 to 1 (default `0.4`), then calls the engine's 1:1
  `/api/v1/verify` route.
- `POST /api/v1/biometrics/documents/scan` accepts `image`, optional
  `back_image`, and optional `presentation=physical|digital`. It detects the
  document, proxies OCR/MRZ/face presence, and adds a `colombian_id` object
  with extracted CC fields and local format/date checks. OCR from both sides
  is combined when a physical-card reverse is submitted. The extraction does
  not check authenticity or consult an official registry.
- `POST /api/v1/biometrics/documents/verify` accepts `document`, `selfie`, and
  an optional `threshold` from 0 to 1 (default `0.4`), then proxies the engine's
  document-face-to-selfie comparison.
- `GET /api/v1/biometrics/liveness/presets` lists upstream challenge presets.
  `POST /api/v1/biometrics/liveness/sessions` creates an active challenge
  session, optionally with a preset (`eye`, `smile`, `multi_range`,
  `head_turn`, or `full`). Submit JPEG/PNG frames with
  `POST /api/v1/biometrics/liveness/sessions/{sessionID}/frame`, check the
  session with `GET /api/v1/biometrics/liveness/sessions/{sessionID}`, and
  cancel it with `DELETE` on that same URL.
- Every image field is validated independently against `MAX_IMAGE_BYTES`, and
  only decoded JPEG/PNG data is forwarded. Engine failures return a sanitized
  502; malformed client input returns a 4xx response.
- Active challenges evaluate successive frames for actions such as blinking or
  turning the head. This is not a guarantee that a video replay or other
  spoofing attack will be rejected. The engine image includes the MediaPipe
  dependency and landmark model required by these routes; actual challenge
  evaluation still depends on camera permission and suitable frames.
- The HTTP client timeout for engine requests is controlled by
  `BIOMETRICS_TIMEOUT_SECONDS`.

Run the unit tests from the laboratory root with:

```sh
cd backend
go test ./...
```

These tests use local HTTP test servers. They validate the Go contract and
failure handling, but are not an end-to-end test against the running
OpenBiometrics container.

## Check the API from inside the container

```sh
podman exec openbiometrics-engine python -c \
  "from urllib.request import urlopen; print(urlopen('http://127.0.0.1:8000/api/v1/health', timeout=5).read().decode())"
```

The route contract observed in the pinned source is
`POST /api/v1/detect`, multipart field `image`, with JSON fields `faces` and
`count` in the response. Engine health and OpenAPI availability were checked
successfully. A synthetic blank-image inference passed through Go and the
running engine. The face-recognition model is loaded, but real face comparison
and document-to-selfie verification remain unverified with authorized samples.
A synthetic document outline passed document detection and reached face
processing; as expected, the face-less image returned HTTP 400. OCR, passive
liveness, active-liveness frame analysis, demographics, and person detection
remain unavailable until their optional dependencies/models are installed.

## Stop and remove the engine container

```sh
podman stop openbiometrics-engine
podman rm openbiometrics-engine
```

The model volume and internal network are retained by these commands.
