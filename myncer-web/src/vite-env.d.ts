/// <reference types="vite/client" />

interface MyncerConfig {
  VITE_SPOTIFY_CLIENT_ID: string;
  VITE_SPOTIFY_REDIRECT_URI: string;
  VITE_YOUTUBE_CLIENT_ID: string;
  VITE_YOUTUBE_REDIRECT_URI: string;
}

interface Window {
  MYNCER_CONFIG: MyncerConfig;
}
