import { Datasource } from "@/generated_grpc/myncer/datasource_pb"
import type { Timestamp } from "@bufbuild/protobuf/wkt"
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import config from "@/config"

const spotifyScopes = [
  "user-read-email",
  "playlist-read-private",
  "playlist-modify-private",
  "playlist-modify-public",
  "user-library-read",
  "user-library-modify"
].join(" ")

const youtubeScopes = [
  "https://www.googleapis.com/auth/youtube"
].join(" ")

const tidalScopes = [
  "r_usr",
  "w_usr",
  "w_sub"
].join(" ")

// --- PKCE Helper Functions ---
async function sha256(plain: string): Promise<ArrayBuffer> {
  const encoder = new TextEncoder()
  const data = encoder.encode(plain)
  return window.crypto.subtle.digest("SHA-256", data)
}

function base64urlencode(a: ArrayBuffer): string {
  return btoa(String.fromCharCode.apply(null, Array.from(new Uint8Array(a))))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "")
}

async function pkceChallenge(verifier: string): Promise<string> {
  const hashed = await sha256(verifier)
  return base64urlencode(hashed)
}

function generateCodeVerifier(): string {
    const randomBytes = new Uint8Array(32);
    window.crypto.getRandomValues(randomBytes);
    return base64urlencode(randomBytes.buffer);
}
// --- Fin de PKCE Helper Functions ---

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export const getSpotifyAuthUrl = () => {
  const clientId = config.spotifyClientId
  const redirectUri = encodeURIComponent(config.spotifyRedirectUri)
  const scope = encodeURIComponent(spotifyScopes)
  const state = crypto.randomUUID() // CSRF protection.

  return `https://accounts.spotify.com/authorize?client_id=${clientId}&response_type=code&redirect_uri=${redirectUri}&scope=${scope}&state=${state}&prompt=consent`
}

export const getYoutubeAuthUrl = () => {
  const clientId = config.youtubeClientId
  const redirectUri = encodeURIComponent(config.youtubeRedirectUri)
  const scope = encodeURIComponent(youtubeScopes)
  const state = crypto.randomUUID() // CSRF protection

  return `https://accounts.google.com/o/oauth2/v2/auth?client_id=${clientId}&redirect_uri=${redirectUri}&response_type=code&scope=${scope}&state=${state}&access_type=offline&prompt=consent`
}

export const getTidalAuthUrl = async (): Promise<string> => {
  const clientId = config.tidalClientId
  // NO codificar aquí - URLSearchParams lo hará
  const redirectUri = config.tidalRedirectUri
  const scope = tidalScopes
  const state = crypto.randomUUID()

  // PKCE logic (required by Tidal)
  const codeVerifier = generateCodeVerifier()
  const codeChallenge = await pkceChallenge(codeVerifier)
  
  // Store the verifier and state to use them in the callback page.
  // This is crucial for the token exchange step.
  sessionStorage.setItem("tidal_code_verifier", codeVerifier)
  sessionStorage.setItem("tidal_csrf_state", state)

  const params = new URLSearchParams({
    response_type: "code",
    client_id: clientId,
    redirect_uri: redirectUri,
    scope: scope,
    code_challenge_method: "S256",
    code_challenge: codeChallenge,
    state: state,
  });

  return `https://login.tidal.com/authorize?${params.toString()}`
}

export const getDatasourceLabel = (datasource: Datasource) => {
  switch (datasource) {
    case Datasource.SPOTIFY:
      return "Spotify"
    case Datasource.YOUTUBE:
      return "YouTube"
    case Datasource.TIDAL:
      return "Tidal"
    default:
      return "Unknown Datasource"
  }
}

export const protoTimestampToDate = (ts: Timestamp): Date => {
  const millis = Number(ts.seconds) * 1000 + Math.floor((ts.nanos || 0) / 1_000_000)
  return new Date(millis)
}

