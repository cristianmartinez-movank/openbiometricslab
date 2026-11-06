const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '';

export interface FaceDetection {
  bbox: number[];
  confidence: number;
  face_size: number;
  landmarks: number[][];
}

export interface FaceResult {
  detection: FaceDetection;
  quality?: {
    overall_score: number;
    is_acceptable: boolean;
    reasons: string[];
  } | null;
  liveness?: {
    is_live: boolean | null;
    score: number | null;
  } | null;
  has_embedding: boolean;
}

export interface DetectResponse {
  faces: FaceResult[];
  count: number;
}

export interface VerifyResponse {
  is_match: boolean;
  similarity: number;
  face1: FaceResult | null;
  face2: FaceResult | null;
}

export interface DocumentScanResponse {
  detected: boolean;
  colombian_id: {
    document_type: 'CC';
    presentation: 'physical' | 'digital' | 'unspecified';
    document_number: string | null;
    number_format_valid: boolean | null;
    surnames: string | null;
    given_names: string | null;
    birth_date: string | null;
    birth_date_valid: boolean | null;
    issue_date: string | null;
    issue_date_valid: boolean | null;
    authenticity_verified: boolean;
    registry_verified: boolean;
    warnings: string[];
  };
  ocr: {
    full_text: string;
    confidence: number;
    lines: Array<{ text: string; confidence: number }>;
  } | null;
  mrz: {
    mrz_type: string;
    document_number: string;
    surname: string;
    given_names: string;
    nationality: string;
    date_of_birth: string;
    sex: string;
    expiry_date: string;
    issuing_country: string;
    check_digits_valid: boolean;
  } | null;
  has_face: boolean;
}

export interface LivenessPreset {
  name: string;
  description: string;
  num_challenges: number;
  challenge_types: string[];
  timeout_seconds: number;
  require_passive: boolean;
}

export interface LivenessChallenge {
  type: string;
  instruction: string;
  timeout_seconds: number;
}

export interface LivenessSession {
  session_id: string;
  state: string;
  challenges: LivenessChallenge[];
}

export interface LivenessFrameResult {
  session_id: string;
  challenge_type: string | null;
  passed: boolean;
  confidence: number;
  challenges_remaining: number;
  session_complete: boolean;
  is_live: boolean | null;
  state: string;
}

interface APIErrorBody {
  error?: string;
}

const errorMessages: Record<string, string> = {
  active_liveness_preset_required: 'Elige un reto de prueba de vida activo.',
  biometrics_engine_error: 'El motor no pudo completar la operación. Revisa sus modelos y dependencias.',
  document_face_not_detected: 'Se encontró una zona de foto en el documento, pero no se pudo reconocer el rostro. Asegúrate de que la foto esté completa, enfocada y con buena resolución.',
  document_face_not_found: 'No se encontró el rostro del titular en el documento. Usa una imagen nítida, frontal y que muestre el documento completo.',
  empty_image: 'El archivo seleccionado está vacío.',
  exactly_one_image_required: 'Selecciona exactamente una imagen para cada campo.',
  image_too_large: 'La imagen supera el límite permitido por el backend.',
  invalid_image: 'El archivo no es una imagen JPEG o PNG válida.',
  invalid_liveness_preset: 'El preset de prueba de vida no es válido.',
  invalid_liveness_session_id: 'La sesión de prueba de vida no es válida.',
  invalid_multipart_request: 'No se pudo leer el formulario enviado.',
  invalid_threshold: 'El umbral debe estar entre 0 y 1.',
  invalid_engine_response: 'El motor devolvió una respuesta inesperada.',
  selfie_face_not_detected: 'No se detectó un rostro claro en la selfie. Usa una foto frontal, enfocada y con el rostro completo visible.',
  unsupported_image_type: 'Formato no compatible. Usa una imagen JPEG o PNG.',
};

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, init);
  } catch {
    throw new Error('No se pudo conectar con el backend Go. Verifica que esté iniciado.');
  }

  const bodyText = await response.text();
  let body: unknown;
  if (bodyText) {
    try {
      body = JSON.parse(bodyText) as unknown;
    } catch {
      throw new Error('El backend devolvió una respuesta que no es JSON válido.');
    }
  }

  if (!response.ok) {
    const errorCode = isAPIErrorBody(body) ? body.error : undefined;
    throw new Error(
      errorCode
        ? errorMessages[errorCode] ?? `El backend respondió con el error: ${errorCode}.`
        : `La solicitud falló con HTTP ${response.status}.`,
    );
  }
  if (body === undefined) {
    throw new Error('El backend devolvió una respuesta vacía.');
  }
  return body as T;
}

function isAPIErrorBody(value: unknown): value is APIErrorBody {
  return typeof value === 'object' && value !== null && 'error' in value
    && typeof value.error === 'string';
}

function imageFormData(images: Record<string, File>, fields?: Record<string, string>): FormData {
  const formData = new FormData();
  for (const [name, file] of Object.entries(images)) {
    formData.append(name, file);
  }
  for (const [name, value] of Object.entries(fields ?? {})) {
    formData.append(name, value);
  }
  return formData;
}

function postImages<T>(
  path: string,
  images: Record<string, File>,
  fields?: Record<string, string>,
): Promise<T> {
  return requestJSON<T>(path, {
    method: 'POST',
    body: imageFormData(images, fields),
  });
}

export function getHealth(): Promise<{ status: string; engine?: string }> {
  return requestJSON('/api/v1/biometrics/health');
}

export function detectFace(image: File): Promise<DetectResponse> {
  return postImages('/api/v1/biometrics/detect', { image });
}

export function verifyFaces(
  image1: File,
  image2: File,
  threshold: number,
): Promise<VerifyResponse> {
  return postImages(
    '/api/v1/biometrics/verify',
    { image1, image2 },
    { threshold: String(threshold) },
  );
}

export function scanDocument(
  image: File,
  presentation: 'physical' | 'digital',
  backImage?: File | null,
): Promise<DocumentScanResponse> {
  return postImages(
    '/api/v1/biometrics/documents/scan',
    backImage ? { image, back_image: backImage } : { image },
    { presentation },
  );
}

export function verifyDocument(
  document: File,
  selfie: File,
  threshold: number,
): Promise<VerifyResponse> {
  return postImages(
    '/api/v1/biometrics/documents/verify',
    { document, selfie },
    { threshold: String(threshold) },
  );
}

export function getLivenessPresets(): Promise<LivenessPreset[]> {
  return requestJSON('/api/v1/biometrics/liveness/presets');
}

export function createLivenessSession(preset: string): Promise<LivenessSession> {
  const query = preset ? `?preset=${encodeURIComponent(preset)}` : '';
  return requestJSON(`/api/v1/biometrics/liveness/sessions${query}`, { method: 'POST' });
}

export function submitLivenessFrame(
  sessionID: string,
  image: File,
): Promise<LivenessFrameResult> {
  return postImages(
    `/api/v1/biometrics/liveness/sessions/${encodeURIComponent(sessionID)}/frame`,
    { image },
  );
}

export function cancelLivenessSession(sessionID: string): Promise<{ cancelled: string }> {
  return requestJSON(
    `/api/v1/biometrics/liveness/sessions/${encodeURIComponent(sessionID)}`,
    { method: 'DELETE' },
  );
}
