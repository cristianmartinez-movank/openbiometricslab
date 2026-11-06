# OpenBiometrics Lab frontend

Aplicación local Svelte + TypeScript para probar los endpoints del backend Go.
El dashboard React dentro del checkout upstream se mantiene sin cambios.

## Iniciar en desarrollo

```sh
cd frontend
npm install
npm run dev
```

Vite sirve la interfaz y reenvía `/api` a `http://localhost:8080`. Para usar
otra dirección del backend durante desarrollo, define
`VITE_API_PROXY_TARGET` antes de iniciar Vite.

```sh
VITE_API_PROXY_TARGET=http://127.0.0.1:8080 npm run dev
```

For the Podman stack, open `http://127.0.0.1:8081`; its Nginx server proxies
API calls internally without publishing the Go API port.

`VITE_API_BASE_URL` permite definir un prefijo de API para despliegues donde
el navegador pueda acceder al backend en el mismo origen o exista una
configuración CORS apropiada. La imagen de producción usa el proxy Nginx
configurado en `containers/nginx/frontend.conf`.

## Flujos disponibles

- Detección facial con imagen JPEG/PNG.
- Comparación facial 1:1 con umbral configurable.
- Escaneo documental con OCR/MRZ y comparación documento-selfie.
- Prueba de vida activa mediante cámara y envío de capturas sucesivas.

La interfaz no almacena imágenes ni usa almacenamiento persistente del
navegador. La cámara requiere permiso explícito y se detiene al salir de la
sección. El reconocimiento, OCR y liveness dependen de modelos y extras
opcionales del motor. El resultado de liveness es experimental y no garantiza
que se rechacen todos los videos o ataques de suplantación.

## Validar

```sh
npm run check
npm run build
```
