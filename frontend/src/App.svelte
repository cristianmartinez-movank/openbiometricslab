<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import FilePicker from './lib/FilePicker.svelte';
  import {
    cancelLivenessSession,
    createLivenessSession,
    detectFace,
    getHealth,
    getLivenessPresets,
    scanDocument,
    submitLivenessFrame,
    verifyDocument,
    verifyFaces,
    type DetectResponse,
    type DocumentScanResponse,
    type LivenessFrameResult,
    type LivenessPreset,
    type LivenessSession,
    type VerifyResponse,
  } from './api';

  type ViewID = 'overview' | 'detect' | 'verify' | 'documents' | 'liveness';

  const navigation: Array<{ id: ViewID; label: string; detail: string }> = [
    { id: 'overview', label: 'Resumen', detail: 'Inicio' },
    { id: 'detect', label: 'Detección facial', detail: 'Rostros' },
    { id: 'verify', label: 'Comparar rostros', detail: '1:1' },
    { id: 'documents', label: 'Documentos', detail: 'OCR · MRZ' },
    { id: 'liveness', label: 'Prueba de vida', detail: 'Capturas' },
  ];

  let activeView: ViewID = 'overview';
  let engineStatus = 'Consultando motor';
  let engineAvailable = false;
  let busy = false;
  let errorMessage = '';
  let detectionFile: File | null = null;
  let firstFaceFile: File | null = null;
  let secondFaceFile: File | null = null;
  let documentFile: File | null = null;
  let documentBackFile: File | null = null;
  let documentPresentation: 'physical' | 'digital' = 'physical';
  let selfieFile: File | null = null;
  let faceThreshold = 0.4;
  let documentThreshold = 0.4;
  let detectionResult: DetectResponse | null = null;
  let faceVerificationResult: VerifyResponse | null = null;
  let documentScanResult: DocumentScanResponse | null = null;
  let documentVerificationResult: VerifyResponse | null = null;
  let livenessPresets: LivenessPreset[] = [];
  let selectedPreset = 'head_turn';
  let livenessPresetError = '';
  let livenessSession: LivenessSession | null = null;
  let challengeIndex = 0;
  let livenessResult: LivenessFrameResult | null = null;
  let livenessComplete = false;
  let cameraStream: MediaStream | null = null;
  let cameraActive = false;
  let livenessCapturing = false;
  let livenessCaptureLoop: Promise<void> | null = null;
  let videoElement: HTMLVideoElement;

  $: currentChallenge = livenessSession?.challenges[challengeIndex] ?? null;

  onMount(() => {
    void refreshHealth();
    void refreshPresets();
  });

  onDestroy(() => stopCamera());

  function titleFor(view: ViewID): string {
    return navigation.find((item) => item.id === view)?.label ?? 'OpenBiometrics Lab';
  }

  function navigate(view: ViewID) {
    if (activeView === 'liveness' && view !== 'liveness') {
      stopCamera();
    }
    activeView = view;
    errorMessage = '';
  }

  async function refreshHealth() {
    try {
      const health = await getHealth();
      engineAvailable = health.status === 'ok' && health.engine === 'available';
      engineStatus = engineAvailable ? 'Motor disponible' : 'Motor no disponible';
    } catch {
      engineAvailable = false;
      engineStatus = 'Sin conexión con el motor';
    }
  }

  async function refreshPresets() {
    try {
      livenessPresets = await getLivenessPresets();
      if (!livenessPresets.some((preset) => preset.name === selectedPreset)) {
        selectedPreset = livenessPresets.find((preset) => preset.num_challenges > 0)?.name ?? '';
      }
    } catch (error) {
      livenessPresetError = errorMessageFor(error);
    }
  }

  async function runDetection() {
    if (!detectionFile) return;
    detectionResult = null;
    await runAction(async () => {
      detectionResult = await detectFace(detectionFile!);
    });
  }

  async function runFaceVerification() {
    if (!firstFaceFile || !secondFaceFile) return;
    faceVerificationResult = null;
    await runAction(async () => {
      faceVerificationResult = await verifyFaces(firstFaceFile!, secondFaceFile!, faceThreshold);
    });
  }

  async function runDocumentScan() {
    if (!documentFile) return;
    documentScanResult = null;
    await runAction(async () => {
      documentScanResult = await scanDocument(
        documentFile!,
        documentPresentation,
        documentPresentation === 'physical' ? documentBackFile : null,
      );
    });
  }

  async function runDocumentVerification() {
    if (!documentFile || !selfieFile) return;
    documentVerificationResult = null;
    await runAction(async () => {
      documentVerificationResult = await verifyDocument(
        documentFile!,
        selfieFile!,
        documentThreshold,
      );
    });
  }

  async function runAction(action: () => Promise<void>) {
    busy = true;
    errorMessage = '';
    try {
      await action();
    } catch (error) {
      errorMessage = errorMessageFor(error);
    } finally {
      busy = false;
    }
  }

  function errorMessageFor(error: unknown): string {
    return error instanceof Error ? error.message : 'Ocurrió un error inesperado.';
  }

  async function openCamera() {
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error('Este navegador no permite acceso a cámara o la página no usa HTTPS/localhost.');
    }
    cameraStream = await navigator.mediaDevices.getUserMedia({
      video: {
        facingMode: 'user',
        width: { ideal: 640 },
        height: { ideal: 480 },
      },
      audio: false,
    });
    cameraActive = true;
    await tick();
    if (videoElement) {
      videoElement.srcObject = cameraStream;
      await videoElement.play();
      await waitForCameraFrame(videoElement);
    }
  }

  function waitForCameraFrame(video: HTMLVideoElement): Promise<void> {
    if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) {
      return Promise.resolve();
    }

    return new Promise((resolve, reject) => {
      const cleanup = () => {
        window.clearTimeout(timeout);
        video.removeEventListener('loadeddata', onLoaded);
      };
      const onLoaded = () => {
        cleanup();
        resolve();
      };
      const timeout = window.setTimeout(() => {
        cleanup();
        reject(new Error('La cámara no entregó una imagen. Verifica el permiso e inténtalo otra vez.'));
      }, 5000);

      video.addEventListener('loadeddata', onLoaded, { once: true });
      if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) {
        onLoaded();
      }
    });
  }

  function stopCamera() {
    livenessCapturing = false;
    cameraStream?.getTracks().forEach((track) => track.stop());
    cameraStream = null;
    cameraActive = false;
    if (videoElement) {
      videoElement.srcObject = null;
    }
  }

  async function beginLiveness() {
    busy = true;
    errorMessage = '';
    try {
      await openCamera();
      livenessSession = await createLivenessSession(selectedPreset);
      challengeIndex = 0;
      livenessResult = null;
      livenessComplete = false;
    } catch (error) {
      stopCamera();
      errorMessage = errorMessageFor(error);
    } finally {
      busy = false;
    }
    if (livenessSession && cameraActive) {
      startLivenessCapture();
    }
  }

  async function resumeLivenessCamera() {
    busy = true;
    errorMessage = '';
    try {
      await openCamera();
    } catch (error) {
      stopCamera();
      errorMessage = errorMessageFor(error);
    } finally {
      busy = false;
    }
    if (livenessSession && cameraActive) {
      startLivenessCapture();
    }
  }

  function captureFrame(): Promise<Blob> {
    return new Promise((resolve, reject) => {
      if (!videoElement || videoElement.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) {
        reject(new Error('La cámara todavía no está lista. Espera un momento e inténtalo otra vez.'));
        return;
      }
      const canvas = document.createElement('canvas');
      canvas.width = videoElement.videoWidth;
      canvas.height = videoElement.videoHeight;
      const context = canvas.getContext('2d');
      if (!context) {
        reject(new Error('No se pudo preparar la captura de cámara.'));
        return;
      }
      context.drawImage(videoElement, 0, 0, canvas.width, canvas.height);
      canvas.toBlob((blob) => {
        if (blob) resolve(blob);
        else reject(new Error('No se pudo crear la captura.'));
      }, 'image/jpeg', 0.9);
    });
  }

  function startLivenessCapture() {
    if (!livenessSession || !cameraActive || livenessComplete) return;
    livenessCapturing = true;
    if (livenessCaptureLoop) return;

    const loop = captureLivenessFrames(livenessSession.session_id);
    livenessCaptureLoop = loop;
    void loop.finally(() => {
      if (livenessCaptureLoop === loop) {
        livenessCaptureLoop = null;
        if (livenessCapturing && cameraActive && !livenessComplete) {
          startLivenessCapture();
        }
      }
    });
  }

  async function captureLivenessFrames(sessionID: string) {
    while (
      livenessCapturing &&
      cameraActive &&
      livenessSession?.session_id === sessionID &&
      !livenessComplete
    ) {
      try {
        const frame = await captureFrame();
        if (!cameraActive || livenessSession?.session_id !== sessionID) return;

        const frameFile = new File([frame], 'liveness-frame.jpg', { type: 'image/jpeg' });
        const result = await submitLivenessFrame(sessionID, frameFile);
        if (livenessSession?.session_id !== sessionID) return;

        livenessResult = result;
        if (result.passed) {
          if (result.session_complete) {
            livenessComplete = true;
            stopCamera();
            return;
          }
          challengeIndex += 1;
        } else if (result.state === 'failed' || result.state === 'expired') {
          livenessComplete = true;
          stopCamera();
          return;
        }

        if (livenessCapturing) {
          await new Promise((resolve) => setTimeout(resolve, 300));
        }
      } catch (error) {
        if (!livenessCapturing || !cameraActive || livenessSession?.session_id !== sessionID) {
          return;
        }
        livenessCapturing = false;
        errorMessage = errorMessageFor(error);
      }
    }
  }

  function toggleLivenessCapture() {
    if (livenessCapturing) {
      livenessCapturing = false;
      return;
    }
    startLivenessCapture();
  }

  async function cancelLiveness() {
    if (!livenessSession) return;
    stopCamera();
    await runAction(async () => {
      await livenessCaptureLoop;
      await cancelLivenessSession(livenessSession!.session_id);
      livenessSession = null;
      livenessResult = null;
      livenessComplete = false;
      challengeIndex = 0;
    });
  }

  function formatJSON(value: unknown): string {
    return JSON.stringify(value, null, 2);
  }
</script>

<svelte:head>
  <title>{titleFor(activeView)} · OpenBiometrics Lab</title>
</svelte:head>

<div class="app-shell">
  <aside class="sidebar">
    <a class="brand" href="/" onclick={(event) => { event.preventDefault(); navigate('overview'); }}>
      <span class="brand-mark">OB</span>
      <span class="brand-name">OpenBiometrics<span>LABORATORIO LOCAL</span></span>
    </a>

    <div class="nav-caption">PLATAFORMA</div>
    <nav aria-label="Secciones del laboratorio">
      {#each navigation as item}
        <button
          class="nav-item"
          class:active={activeView === item.id}
          aria-current={activeView === item.id ? 'page' : undefined}
          onclick={() => navigate(item.id)}
        >
          <span class="nav-indicator" aria-hidden="true"></span>
          <span>{item.label}</span>
          {#if item.id !== 'overview'}<small>{item.detail}</small>{/if}
        </button>
      {/each}
    </nav>

    <div class="sidebar-bottom">
      <span class:online={engineAvailable} class="status-dot"></span>
      <div>
        <strong>{engineStatus}</strong>
        <small>API Go · localhost</small>
      </div>
      <button class="refresh-button" aria-label="Revisar conexión" onclick={refreshHealth}>↻</button>
    </div>
  </aside>

  <main class="main-content">
    <header class="topbar">
      <div class="breadcrumb"><span>Laboratorio</span><span class="breadcrumb-separator">/</span><strong>{titleFor(activeView)}</strong></div>
      <div class="topbar-right">
        <span class:online={engineAvailable} class="connection-pill">
          <span class="status-dot"></span>{engineStatus}
        </span>
        <span class="local-badge">ENTORNO LOCAL</span>
      </div>
    </header>

    <div class="page-wrap">
      {#if activeView === 'overview'}
        <section class="page-heading overview-heading">
          <div>
            <p class="eyebrow">OPENBIOMETRICS · ZONA DE PRUEBAS</p>
            <h1>Tu laboratorio biométrico.</h1>
            <p class="lead">Prueba los flujos del motor desde una interfaz local. Las imágenes se envían al backend Go y no se guardan en esta aplicación.</p>
          </div>
          <div class="heading-art" aria-hidden="true"><div class="art-ring"></div><div class="art-core">OB</div><span class="art-dot dot-one"></span><span class="art-dot dot-two"></span></div>
        </section>

        <section class="metrics-grid" aria-label="Estado del laboratorio">
          <article class="metric-card"><span class="metric-label">ESTADO DEL MOTOR</span><strong class:available={engineAvailable}>{engineAvailable ? 'Disponible' : 'Sin conexión'}</strong><span>OpenBiometrics API</span></article>
          <article class="metric-card"><span class="metric-label">SERVICIO API</span><strong>Go</strong><span>Proxy biométrico local</span></article>
          <article class="metric-card"><span class="metric-label">DATOS</span><strong>No persistentes</strong><span>Sin almacenamiento en la UI</span></article>
        </section>

        <div class="section-title-row"><div><p class="eyebrow">HERRAMIENTAS</p><h2>¿Qué quieres probar?</h2></div><span class="section-count">04 FLUJOS</span></div>
        <section class="tool-grid">
          <button class="tool-card" onclick={() => navigate('detect')}><span class="tool-number">01</span><span class="tool-icon face-icon">◎</span><strong>Detección facial</strong><span>Encuentra rostros y consulta calidad de imagen.</span><span class="tool-link">Abrir herramienta <b>↗</b></span></button>
          <button class="tool-card" onclick={() => navigate('verify')}><span class="tool-number">02</span><span class="tool-icon compare-icon">◉</span><strong>Comparación 1:1</strong><span>Compara dos imágenes sin buscar en listas.</span><span class="tool-link">Abrir herramienta <b>↗</b></span></button>
          <button class="tool-card" onclick={() => navigate('documents')}><span class="tool-number">03</span><span class="tool-icon document-icon">▤</span><strong>Documentos</strong><span>Explora OCR, MRZ y comparación con selfie.</span><span class="tool-link">Abrir herramienta <b>↗</b></span></button>
          <button class="tool-card" onclick={() => navigate('liveness')}><span class="tool-number">04</span><span class="tool-icon live-icon">◌</span><strong>Prueba de vida</strong><span>Completa retos activos con capturas de cámara.</span><span class="tool-link">Abrir herramienta <b>↗</b></span></button>
        </section>
        <aside class="notice"><span class="notice-icon">i</span><p><strong>Alcance de laboratorio</strong> Las capacidades de OCR, reconocimiento y prueba de vida dependen de modelos y extras opcionales. Una prueba de vida aprobada no garantiza que todo video o intento de suplantación sea detectado.</p></aside>

      {:else if activeView === 'detect'}
        <section class="page-heading compact-heading"><div><p class="eyebrow">ANÁLISIS DE IMAGEN</p><h1>Detección facial</h1><p class="lead">Carga una imagen para detectar rostros y consultar sus métricas.</p></div><span class="step-badge">01 / 04</span></section>
        <section class="workspace-grid">
          <article class="panel input-panel">
            <div class="panel-heading"><div><span class="panel-kicker">ENTRADA</span><h2>Imagen de prueba</h2></div><span class="panel-index">01</span></div>
            <FilePicker label="Selecciona una imagen" description="Arrastra o elige un archivo JPEG o PNG" bind:file={detectionFile} />
            <p class="field-note">La imagen se procesa en memoria y se envía al motor para esta solicitud.</p>
            <button class="primary-button" disabled={!detectionFile || busy} onclick={runDetection}>{busy ? 'Procesando…' : 'Detectar rostros'} <span>→</span></button>
          </article>
          <article class="panel output-panel">
            <div class="panel-heading"><div><span class="panel-kicker">RESULTADO</span><h2>Respuesta del motor</h2></div><span class="result-live"><i></i> JSON</span></div>
            {#if errorMessage}<div class="error-box" role="alert">{errorMessage}</div>
            {:else if detectionResult}
              <div class="result-summary"><div class="result-count">{detectionResult.count}</div><div><strong>{detectionResult.count === 1 ? 'rostro detectado' : 'rostros detectados'}</strong><span>Resultado del análisis facial</span></div></div>
              {#each detectionResult.faces as face, index}
                <div class="face-row"><span class="face-index">0{index + 1}</span><div><strong>Rostro {index + 1}</strong><small>Confianza de detección</small></div><b>{Math.round(face.detection.confidence * 100)}%</b></div>
              {/each}
              <details class="json-details"><summary>Ver respuesta completa</summary><pre>{formatJSON(detectionResult)}</pre></details>
            {:else}<div class="empty-state"><span class="empty-symbol">◎</span><strong>Aún no hay resultados</strong><p>El análisis aparecerá aquí cuando envíes una imagen.</p></div>{/if}
          </article>
        </section>

      {:else if activeView === 'verify'}
        <section class="page-heading compact-heading"><div><p class="eyebrow">COMPARACIÓN BIOMÉTRICA</p><h1>Comparación facial 1:1</h1><p class="lead">Compara dos muestras. Este flujo no realiza identificación 1:N.</p></div><span class="step-badge">02 / 04</span></section>
        <section class="panel">
          <div class="panel-heading"><div><span class="panel-kicker">MUESTRAS</span><h2>Selecciona dos imágenes</h2></div><span class="result-live">2 imágenes</span></div>
          <div class="upload-pair"><FilePicker label="Imagen de referencia" description="Primera muestra · JPEG o PNG" bind:file={firstFaceFile} /><span class="versus">VS</span><FilePicker label="Segunda imagen" description="Segunda muestra · JPEG o PNG" bind:file={secondFaceFile} /></div>
          <div class="form-footer"><label class="threshold-control">Umbral de comparación <span>{faceThreshold.toFixed(2)}</span><input type="range" min="0" max="1" step="0.01" bind:value={faceThreshold} /></label><button class="primary-button" disabled={!firstFaceFile || !secondFaceFile || busy} onclick={runFaceVerification}>{busy ? 'Comparando…' : 'Comparar rostros'} <span>→</span></button></div>
        </section>
        {#if errorMessage}<div class="error-box result-error" role="alert">{errorMessage}</div>{/if}
        {#if faceVerificationResult}<section class="panel verification-result"><div class="panel-heading"><div><span class="panel-kicker">RESULTADO 1:1</span><h2>{faceVerificationResult.is_match ? 'Coincidencia sobre el umbral' : 'No supera el umbral'}</h2></div><span class:match={faceVerificationResult.is_match} class="match-pill">{faceVerificationResult.is_match ? 'COINCIDE' : 'NO COINCIDE'}</span></div><div class="similarity-row"><div><strong>{(faceVerificationResult.similarity * 100).toFixed(1)}<small>%</small></strong><span>Similitud reportada por el motor</span></div><div class="similarity-track"><i style={`width:${Math.max(0, Math.min(100, faceVerificationResult.similarity * 100))}%`}></i></div></div><details class="json-details"><summary>Ver respuesta completa</summary><pre>{formatJSON(faceVerificationResult)}</pre></details></section>{/if}

      {:else if activeView === 'documents'}
        <section class="page-heading compact-heading"><div><p class="eyebrow">LECTURA Y VERIFICACIÓN</p><h1>Análisis de documentos</h1><p class="lead">Procesa un documento y, si quieres, compara su rostro con una selfie.</p></div><span class="step-badge">03 / 04</span></section>
        <section class="panel">
          <div class="panel-heading"><div><span class="panel-kicker">DOCUMENTO</span><h2>Selecciona la imagen del documento</h2></div><span class="result-live">JPEG · PNG</span></div>
          <FilePicker label="Frente de la cédula" description="CC física o captura digital legible · JPEG o PNG" bind:file={documentFile} />
          <div class="document-scan-controls">
            <label class="document-presentation">Presentación de la cédula
              <select bind:value={documentPresentation}>
                <option value="physical">Física · amarilla con hologramas</option>
                <option value="digital">Digital · captura de pantalla</option>
              </select>
            </label>
            <button class="secondary-button" disabled={!documentFile || busy} onclick={runDocumentScan}>{busy ? 'Analizando…' : 'Analizar documento'} <span>→</span></button>
          </div>
          {#if documentPresentation === 'physical'}
            <div class="document-back-upload"><FilePicker label="Reverso (opcional)" description="Inclúyelo para leer la fecha de expedición · JPEG o PNG" bind:file={documentBackFile} /></div>
          {/if}
          <p class="field-note document-validation-note">Para la CC física, carga también el reverso para intentar leer la fecha de expedición. La lectura y validación son locales; no se consulta la Registraduría ni se confirma autenticidad o vigencia.</p>
        </section>
        {#if errorMessage}<div class="error-box result-error" role="alert">{errorMessage}</div>{/if}
        {#if documentScanResult}<section class="panel document-result"><div class="panel-heading"><div><span class="panel-kicker">LECTURA</span><h2>Resultado del documento</h2></div><span class:match={documentScanResult.detected} class="match-pill">{documentScanResult.detected ? 'DOCUMENTO DETECTADO' : 'SIN DETECCIÓN'}</span></div><div class="document-facts"><div><span>Rostro en documento</span><strong>{documentScanResult.has_face ? 'Detectado' : 'No detectado'}</strong></div><div><span>Texto OCR</span><strong>{documentScanResult.ocr ? 'Disponible' : 'Sin resultado'}</strong></div><div><span>Zona MRZ</span><strong>{documentScanResult.mrz ? 'Leída' : 'No encontrada'}</strong></div></div>
          <div class="colombian-id-result">
            <div class="panel-heading"><div><span class="panel-kicker">CÉDULA DE CIUDADANÍA · {documentScanResult.colombian_id.presentation === 'digital' ? 'DIGITAL' : documentScanResult.colombian_id.presentation === 'physical' ? 'FÍSICA' : 'PRESENTACIÓN NO INDICADA'}</span><h3>Campos extraídos y validaciones locales</h3></div></div>
            <div class="document-facts">
              <div><span>Número</span><strong>{documentScanResult.colombian_id.document_number ?? 'No leído'} · {documentScanResult.colombian_id.number_format_valid === null ? 'Sin validar' : documentScanResult.colombian_id.number_format_valid ? 'Formato válido' : 'Formato inválido'}</strong></div>
              <div><span>Apellidos</span><strong>{documentScanResult.colombian_id.surnames ?? 'No leídos'}</strong></div>
              <div><span>Nombres</span><strong>{documentScanResult.colombian_id.given_names ?? 'No leídos'}</strong></div>
              <div><span>Fecha de nacimiento</span><strong>{documentScanResult.colombian_id.birth_date ?? 'No leída'} · {documentScanResult.colombian_id.birth_date_valid === null ? 'Sin validar' : documentScanResult.colombian_id.birth_date_valid ? 'Fecha válida' : 'Fecha inválida'}</strong></div>
              <div><span>Fecha de expedición</span><strong>{documentScanResult.colombian_id.issue_date ?? 'No leída'} · {documentScanResult.colombian_id.issue_date_valid === null ? 'Sin validar' : documentScanResult.colombian_id.issue_date_valid ? 'Fecha válida' : 'Fecha inválida'}</strong></div>
              <div><span>Verificación oficial</span><strong>No realizada</strong></div>
            </div>
            {#if documentScanResult.colombian_id.warnings.length}<ul class="validation-warnings">{#each documentScanResult.colombian_id.warnings as warning}<li>{warning}</li>{/each}</ul>{/if}
          </div>
          {#if documentScanResult.ocr?.full_text}<div class="ocr-text"><span class="panel-kicker">TEXTO OCR</span><pre>{documentScanResult.ocr.full_text}</pre></div>{/if}{#if documentScanResult.mrz}<div class="mrz-result"><span class="panel-kicker">MRZ · {documentScanResult.mrz.mrz_type}</span><div><span>Documento<strong>{documentScanResult.mrz.document_number}</strong></span><span>Apellidos<strong>{documentScanResult.mrz.surname}</strong></span><span>Nombres<strong>{documentScanResult.mrz.given_names}</strong></span><span>Fecha de nacimiento<strong>{documentScanResult.mrz.date_of_birth}</strong></span><span>Vencimiento<strong>{documentScanResult.mrz.expiry_date}</strong></span><span>Checks<strong>{documentScanResult.mrz.check_digits_valid ? 'Válidos' : 'No válidos'}</strong></span></div></div>{/if}<details class="json-details"><summary>Ver respuesta completa</summary><pre>{formatJSON(documentScanResult)}</pre></details></section>{/if}
        <section class="panel compare-document-panel"><div class="panel-heading"><div><span class="panel-kicker">OPCIONAL · VERIFICACIÓN 1:1</span><h2>Documento contra selfie</h2></div><span class="panel-index">BETA</span></div><div class="upload-pair"><FilePicker label="Documento" description="Imagen del documento · JPEG o PNG" bind:file={documentFile} /><span class="versus">VS</span><FilePicker label="Selfie" description="Rostro en vivo · JPEG o PNG" bind:file={selfieFile} /></div><div class="form-footer"><label class="threshold-control">Umbral de comparación <span>{documentThreshold.toFixed(2)}</span><input type="range" min="0" max="1" step="0.01" bind:value={documentThreshold} /></label><button class="primary-button" disabled={!documentFile || !selfieFile || busy} onclick={runDocumentVerification}>{busy ? 'Comparando…' : 'Comparar documento y selfie'} <span>→</span></button></div>{#if documentVerificationResult}<div class="inline-result"><span class:match={documentVerificationResult.is_match} class="match-pill">{documentVerificationResult.is_match ? 'COINCIDE' : 'NO COINCIDE'}</span><strong>Similitud {(documentVerificationResult.similarity * 100).toFixed(1)}%</strong><details class="json-details"><summary>Ver respuesta completa</summary><pre>{formatJSON(documentVerificationResult)}</pre></details></div>{/if}</section>

      {:else}
        <section class="page-heading compact-heading"><div><p class="eyebrow">RETO INTERACTIVO</p><h1>Prueba de vida activa</h1><p class="lead">Sigue las instrucciones; la cámara se evalúa automáticamente durante cada reto.</p></div><span class="step-badge">04 / 04</span></section>
        <aside class="notice liveness-notice"><span class="notice-icon">!</span><p><strong>Importante:</strong> el motor evalúa acciones en capturas; el resultado es experimental y no garantiza que todo video o suplantación sea rechazado. La cámara se detiene al salir de esta sección.</p></aside>
        {#if livenessPresetError}<div class="error-box" role="alert">{livenessPresetError}</div>{/if}
        <section class="liveness-layout">
          <article class="panel camera-panel">
            <div class="panel-heading"><div><span class="panel-kicker">CÁMARA</span><h2>Captura en tiempo real</h2></div><span class:camera-on={cameraActive} class="camera-status">{cameraActive ? 'ACTIVA' : 'APAGADA'}</span></div>
            <div class="camera-stage">
              {#if cameraActive}
                <video bind:this={videoElement} autoplay muted playsinline aria-label="Vista previa de la cámara"></video>
                <div class="camera-reticle" aria-hidden="true"></div>
              {:else}<div class="camera-placeholder"><span>◎</span><strong>Tu cámara aparecerá aquí</strong><small>Se solicita permiso solo al iniciar una sesión.</small></div>{/if}
            </div>
            {#if !livenessSession}<label class="preset-select">Tipo de reto<select bind:value={selectedPreset} disabled={livenessPresets.length === 0 || busy}>{#each livenessPresets.filter((preset) => preset.num_challenges > 0) as preset}<option value={preset.name}>{preset.name.replaceAll('_', ' ')} · {preset.num_challenges} retos</option>{/each}</select></label>{/if}
            {#if !livenessSession}<button class="primary-button full-button" disabled={busy || livenessPresets.length === 0} onclick={beginLiveness}>{busy ? 'Iniciando…' : 'Iniciar prueba de vida'} <span>→</span></button>
            {:else if livenessComplete}<div class="complete-banner"><strong>{livenessResult?.is_live ? 'Sesión completada' : 'Reto no superado'}</strong><span>{livenessResult?.is_live ? 'El motor indica persona viva.' : 'El tiempo del reto terminó; cierra la sesión para intentar de nuevo.'}</span><button class="secondary-button" onclick={cancelLiveness}>Cerrar sesión</button></div>
            {:else if !cameraActive}<button class="primary-button full-button" disabled={busy} onclick={resumeLivenessCamera}>Activar cámara y continuar <span>→</span></button>
            {:else}<button class="primary-button full-button" disabled={busy || !currentChallenge} onclick={toggleLivenessCapture}>{livenessCapturing ? 'Pausar evaluación' : 'Reanudar evaluación'} <span>{livenessCapturing ? 'Ⅱ' : '→'}</span></button><button class="text-button full-button" disabled={busy} onclick={stopCamera}>Pausar cámara</button>{/if}
            {#if livenessSession && !livenessComplete}<button class="cancel-button" disabled={busy} onclick={cancelLiveness}>Cancelar sesión</button>{/if}
          </article>

          <article class="panel challenge-panel">
            <div class="panel-heading"><div><span class="panel-kicker">SECUENCIA</span><h2>Instrucciones del reto</h2></div>{#if livenessSession}<span class="step-badge">{Math.min(challengeIndex + 1, livenessSession.challenges.length)} / {livenessSession.challenges.length}</span>{/if}</div>
            {#if livenessSession && currentChallenge && !livenessComplete}
              <div class="challenge-current"><span class="challenge-symbol">↗</span><span class="challenge-type">{currentChallenge.type.replaceAll('_', ' ')}</span><h3>{currentChallenge.instruction}</h3><p>{livenessCapturing ? 'Realiza la acción frente a la cámara; se evalúa automáticamente.' : 'Pulsa «Reanudar evaluación» para continuar con las capturas automáticas.'}</p><small>TIEMPO POR RETO · {currentChallenge.timeout_seconds} S</small></div>
              <div class="challenge-progress">{#each livenessSession.challenges as challenge, index}<span class:done={index < challengeIndex} class:current={index === challengeIndex} title={challenge.type}></span>{/each}</div>
            {:else if livenessComplete}<div class="empty-state success-state"><span class="empty-symbol">{livenessResult?.is_live ? '✓' : '!'}</span><strong>{livenessResult?.is_live ? 'Reto completado' : 'Reto no superado'}</strong><p>{livenessResult?.is_live === true ? 'El motor reportó un resultado positivo de prueba de vida.' : 'El tiempo de evaluación terminó antes de superar el reto.'}</p></div>
            {:else if livenessSession}<div class="empty-state"><span class="empty-symbol">◌</span><strong>Preparando secuencia</strong><p>Espera a que el backend prepare los retos.</p></div>
            {:else}<div class="empty-state"><span class="empty-symbol">↗</span><strong>Una secuencia aleatoria</strong><p>Inicia la sesión para recibir instrucciones de movimiento.</p></div>{/if}
            {#if livenessResult && !livenessComplete}<div class:passed={livenessResult.passed} class="frame-feedback" role="status"><strong>{livenessResult.passed ? 'Reto superado' : 'Aún no se detecta el reto'}</strong><span>Confianza reportada: {(livenessResult.confidence * 100).toFixed(0)}%</span></div>{/if}
            <p class="privacy-note">Las capturas se envían al motor para evaluación y no se conservan en el frontend.</p>
          </article>
        </section>
        {#if errorMessage}<div class="error-box result-error" role="alert">{errorMessage}</div>{/if}
      {/if}
      <footer class="page-footer"><span>OPENBIOMETRICS LAB</span><span>Desarrollo local · Las respuestas dependen de los modelos disponibles</span></footer>
    </div>
  </main>
</div>
